# 阶段 2：统一上传大小限制 + 图像 DoS 加固

## 目标

让任意上传/导入路径都无法用「超大文件」或「超大像素图」拖垮服务：

1. **所有** Web 上传路径统一执行 `max_texture_size` 校验（当前只有游戏端 PUT 做了）。
2. 给图像加**像素尺寸上限**，堵住「64 的倍数即合法」导致的超大图。
3. 设置 Pillow `MAX_IMAGE_PIXELS`，防解压炸弹。
4. 把逐像素哈希循环**向量化**，并将 CPU 密集的图像处理**下放到线程池**，不阻塞事件循环。

## 问题证据

- 大小校验只在 `backends/yggdrasil_backend.py:346-348`（游戏端 `PUT /api/user/profile/...`）：
  ```python
  max_size_kb_str = await self.db.setting.get("max_texture_size", "1024")
  if len(file_bytes) > int(max_size_kb_str) * 1024:
      raise IllegalArgumentException("Texture file too large.")
  ```
  而 Web 端三处 `content = await file.read()` 后直接处理，**无任何大小校验**：
  - `routers/site_routes.py` `/me/textures`（`upload_texture_to_library`）
  - `routers/site_routes.py` `/textures/upload`（`upload_and_apply_texture`）
  - `routers/admin_routes.py` `/admin/carousel`（`upload_carousel`）
  - 此外 `microsoft_backend._import_texture` 与 `profile_import_backend._import_texture` 走 `download_texture` 拉远程图，也无大小限制（与阶段 3 的 SSRF 相关，本阶段补大小）。
- `utils/image_utils.py:validate_texture_dimensions`：`(w % 64 == 0 and h == w)` 等条件**只约束整除关系，不设上限** → 6400×6400 可过。
- `utils/image_utils.py:compute_texture_hash_from_image`：纯 Python 双层 `for x: for y:` 逐像素 `pixels[x,y]` 解包 + 写 `bytearray`，O(W×H) 且常数大。配合上一条，单请求可阻塞事件循环数秒。
- `normalize_png` 用 `Image.open` 打开任意 PNG，**未设 `Image.MAX_IMAGE_PIXELS`**，存在解压炸弹风险。
- 全代码库无 `run_in_executor`/`to_thread`（已 grep 确认），图像处理全程占用 event loop。

## 设计决策

- **大小限制**放在「读入字节后、处理前」的统一入口。最干净的位置是 `TextureStorage.process_and_save` 的调用前置校验，但 carousel 不走 texture storage，且 `max_size_kb` 是 DB 设置（同步函数读不到）。因此：
  - 材质类：在各 backend 方法入口处统一校验（已 async，能读 `db.setting`）。抽一个 helper 避免重复。
  - carousel：单独的图片，给一个固定上限（如复用 `max_texture_size` 或独立常量）。
- **像素上限**与 **PNG 解码保护**放在 `utils/image_utils.py`（领域最内层，所有路径必经）。
- **线程池下放**放在 `TextureStorage.process_and_save`（CPU 密集的唯一入口），改为 `async` 或提供 async 包装。

## 改造清单

### 2.1 统一材质大小校验

`backends` 内新增共享校验（可放 `services/texture_storage.py` 旁或 `utils`）。鉴于 `max_texture_size` 在 DB，给 `TextureStorage` 之外加一个轻量校验函数，由各 backend 调用：

```python
# 例如在 backends 内或 utils 内
async def assert_texture_size(db, file_bytes: bytes):
    max_kb = int(await db.setting.get("max_texture_size", "1024"))
    if len(file_bytes) > max_kb * 1024:
        raise ValueError("Texture file too large.")   # router 已统一把 ValueError 转 400
```

接入点：
- `site_backend.upload_texture_to_library`、`upload_and_apply_texture` 入口先 `await assert_texture_size(...)`。
- `microsoft_backend._import_texture`、`profile_import_backend._import_texture`：`download_texture` 返回后校验（也作为 SSRF 拉取大文件的兜底）。
- `yggdrasil_backend.upload_texture` 已有校验，改为调用同一 helper，消除重复。

`admin_routes.upload_carousel`：读入后校验大小（给个明确上限，如 5MB 常量），并保持已有扩展名校验。

### 2.2 像素上限 + 解压炸弹保护

`utils/image_utils.py`：

```python
from PIL import Image

# 单张材质的像素上限（皮肤最大约 1024x1024 已极宽松；按需调整）
MAX_TEXTURE_DIMENSION = 1024
# 解压炸弹保护：限制解码后总像素
Image.MAX_IMAGE_PIXELS = 1024 * 1024 * 4   # 触发 DecompressionBombError
```

`validate_texture_dimensions` 增加上限判断：

```python
def validate_texture_dimensions(img, is_cape=False) -> bool:
    w, h = img.size
    if w <= 0 or h <= 0 or w > MAX_TEXTURE_DIMENSION or h > MAX_TEXTURE_DIMENSION:
        return False
    # ...原有整除关系判断不变
```

`normalize_png` 捕获 `Image.DecompressionBombError`，归一化为现有的 `ValueError("Failed to normalize PNG: ...")`（已有 `except Exception` 兜底，确认其会把 DecompressionBombError 也转成 ValueError 即可——它会，因为 DecompressionBombError 是 Exception 子类）。

### 2.3 哈希向量化

`compute_texture_hash_from_image` 用 `img.tobytes()` 取 RGBA 平面后向量化处理 alpha=0 → RGB 清零，再拼宽高头计算 SHA-256。优先用 numpy（项目可加 numpy 依赖；Pillow 常已带 numpy 依赖链，但需在 requirements 显式声明）：

```python
import numpy as np

def compute_texture_hash_from_image(img) -> str:
    width, height = img.size
    arr = np.asarray(img, dtype=np.uint8).reshape(height, width, 4)  # H,W,RGBA
    # 规范要求按 (x, y) 即列优先遍历：转成 W,H,RGBA
    arr = arr.transpose(1, 0, 2)            # W,H,RGBA
    rgba = arr.reshape(-1, 4)               # 顺序 = for x: for y
    # alpha==0 → rgb 清零
    zero_mask = rgba[:, 3] == 0
    rgba[zero_mask, 0:3] = 0
    # 规范写入顺序为 ARGB
    argb = rgba[:, [3, 0, 1, 2]].tobytes()
    header = struct.pack(">II", width, height)
    return hashlib.sha256(header + argb).hexdigest()
```

> **关键约束**：哈希值必须与改造前**逐字节一致**（材质 hash 既是文件名又是 DB 主键，变了会导致存量材质全部错位）。务必用真实皮肤跑「旧实现 vs 新实现」对比测试，确认 hash 相同后再替换。遍历顺序（列优先 x→y）和 ARGB 字节序是对齐的核心，必须保持。

如不引入 numpy，可保留纯 Python 但用 `img.tobytes()` 批量取字节（比 `pixels[x,y]` 逐点快一个量级），逻辑等价改写。numpy 方案更优，二选一。

### 2.4 图像处理下放线程池

`services/texture_storage.py` 的 `process_and_save` 是 CPU 密集同步函数。新增 async 包装，调用方改用它：

```python
import asyncio

class TextureStorage:
    def process_and_save(self, file_bytes, texture_type) -> str:
        ...   # 保持同步实现不变

    async def process_and_save_async(self, file_bytes, texture_type) -> str:
        return await asyncio.to_thread(self.process_and_save, file_bytes, texture_type)
```

把 backend 中所有 `self.texture_storage.process_and_save(...)` 调用改为 `await self.texture_storage.process_and_save_async(...)`：
- `yggdrasil_backend.upload_texture`
- `site_backend.upload_texture_to_library`
- `microsoft_backend._import_texture`
- `profile_import_backend._import_texture`

> free-threaded 3.14 下线程池能真正并行，受益更明显；GIL 模式下也至少避免阻塞 event loop。

## 影响文件

- 修改：`utils/image_utils.py`（像素上限、MAX_IMAGE_PIXELS、哈希向量化）
- 修改：`services/texture_storage.py`（async 包装）
- 修改：`backends/yggdrasil_backend.py`、`backends/site_backend.py`、`backends/microsoft_backend.py`、`backends/profile_import_backend.py`（统一大小校验 + async 调用）
- 修改：`routers/admin_routes.py`（carousel 大小校验）
- 修改：`requirements.txt`（如采用 numpy 方案，显式加 `numpy`）
- 可能新增：大小校验 helper（位置见 2.1）

## 测试与验证

- **哈希一致性（最高优先）**：取若干真实皮肤/披风，断言新旧 `compute_texture_hash_from_image` 输出逐字节相同。建议在 `tests/services/test_texture_storage.py` 或 `tests/utils` 增加固定样本回归。
- 尺寸上限：构造 2048×2048 合法整除图 → `validate_texture_dimensions` 返回 False → 上传得到 400。
- 大小限制：构造超过 `max_texture_size` 的字节 → 四条材质路径 + carousel 均返回 400。
- 解压炸弹：构造声明超大但压缩率极高的 PNG → `normalize_png` 抛 ValueError → 400，且未 OOM。
- 事件循环：可选——并发上传多张大图时，其他轻量请求（如 `/public/settings`）延迟不应明显升高（验证 `to_thread` 生效）。
- `pytest -q` 全绿。

## 风险与回滚

中风险，集中在 **2.3 哈希向量化**：一旦 hash 与旧实现不一致，存量材质会全部失联。务必以「哈希一致性测试通过」为合并前置条件；若无把握，可先只做 2.1/2.2/2.4（已能挡住绝大部分 DoS），把 2.3 拆成独立 PR 单独验证。各子项可独立 revert。
