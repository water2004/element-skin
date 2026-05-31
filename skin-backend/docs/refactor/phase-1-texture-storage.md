# 阶段 1：抽出 TextureStorage，剥离 DB 层的图像/文件 IO

## 目标

把 `database_module/modules/texture.py` 中**不属于数据访问层**的职责剥离出去：图像规范化、尺寸校验、哈希计算、文件系统读写。数据库模块只保留对 `user_textures` / `skin_library` 表的纯记录操作。

## 当前问题（证据）

`database_module/modules/texture.py`：

- `__init__`（21-22 行）：`self.textures_dir = config.get(...)` + `os.makedirs(...)` —— DB 模块知道了"材质存在磁盘上"。
- `upload()`（24-48 行）：
  ```python
  normalized_bytes, img = normalize_png(file_bytes)     # 图像解码
  if not validate_texture_dimensions(img, is_cape): ... # 业务校验
  texture_hash = compute_texture_hash_from_image(img)   # 哈希
  with open(file_path, "wb") as f: f.write(...)         # 文件写入
  await self.add_to_library(...)                        # ← 唯一真正属于 DB 的一行
  ```
- 顶部 `import os` / `from PIL import Image` / `from io import BytesIO` / `from utils.image_utils import (...)` / `from config_loader import config` —— 这些依赖都不该出现在 DB 模块。

## 目标架构

新增 `services/` 层（介于 utils 和 backend 之间的领域服务），或放在 `backends/` 下。建议新建：

```
skin-backend/
  services/
    __init__.py
    texture_storage.py     # TextureStorage：图像处理 + 文件落盘
```

> 命名说明：叫 `services` 而非 `backends`，是为了区分"无业务编排的纯领域服务"和"有业务编排的 backend"。若团队更倾向扁平结构，也可放 `backends/texture_storage.py`，不影响本阶段核心。

### `TextureStorage` 职责

```python
class TextureStorage:
    def __init__(self, textures_dir: str):
        self.textures_dir = textures_dir
        os.makedirs(self.textures_dir, exist_ok=True)

    def process_and_save(self, file_bytes: bytes, texture_type: str) -> str:
        """规范化、校验尺寸、计算 hash、落盘，返回 texture_hash。
        校验失败抛 ValueError。"""
        normalized_bytes, img = normalize_png(file_bytes)
        is_cape = texture_type.lower() == "cape"
        if not validate_texture_dimensions(img, is_cape):
            raise ValueError("Invalid texture dimensions")
        texture_hash = compute_texture_hash_from_image(img)
        path = os.path.join(self.textures_dir, f"{texture_hash}.png")
        with open(path, "wb") as f:
            f.write(normalized_bytes)
        return texture_hash

    def delete_file(self, texture_hash: str) -> None:
        """物理删除材质文件（幂等）。供后续清理孤儿文件用，本阶段可选实现。"""
        ...
```

### `TextureModule` 瘦身后

```python
# texture.py 顶部不再 import PIL / os / image_utils / config
class TextureModule:
    def __init__(self, db: BaseDB):
        self.db = db
    # 删除 upload()；保留 add_to_library / delete_from_library / *_cursor / count_* 等纯 DB 方法
```

`upload()` 整体从 DB 层移除。它的编排（先 storage 后 DB）上移到调用方（见下）。

## 调用方改造

当前 `db.texture.upload(...)` 有 4 个调用点。本阶段先引入 storage，并把"处理+落盘+记库"的两步编排收敛到一个 backend 方法里：

| 调用点 | 现状 | 改造后 |
|--------|------|--------|
| `routers/site_routes.py:196` `/me/textures` | `await db.texture.upload(...)` | 调 `site_backend.upload_texture_to_library(...)`（阶段 4 会进一步收口，本阶段先让 backend 暴露此方法） |
| `routers/site_routes.py:394` `/textures/upload` | `await db.texture.upload(...)` | 同上，调 backend |
| `backends/site_backend.py:84,95` 导入皮肤/披风 | `await self.db.texture.upload(...)` | 改调 `self.texture_storage.process_and_save(...)` + `self.db.texture.add_to_library(...)` |
| `backends/yggdrasil_backend.py:248` | `await self.db.texture.upload(...)` | 同上 |

为避免在本阶段就动 router（那是阶段 4 的事），建议在 `SiteBackend` 上新增一个权威方法，供 router 和内部复用：

```python
# site_backend.py
async def upload_texture_to_library(self, user_id, file_bytes, texture_type,
                                    note="", is_public=False, model="default") -> tuple[str, str]:
    texture_hash = self.texture_storage.process_and_save(file_bytes, texture_type)
    await self.db.texture.add_to_library(user_id, texture_hash, texture_type, note, is_public, model)
    return texture_hash, texture_type
```

`yggdrasil_backend` 因为不持有 `SiteBackend`，给它也注入 `TextureStorage`（构造函数加参数），或抽一个共享的 `TextureService`。本阶段选**注入 `TextureStorage` 到两个 backend**，最小改动。

## 依赖注入改造

找到组装根（app 启动处，搜索 `TextureModule` / `SiteBackend(` / `YggdrasilBackend(` 的实例化点），创建单个 `TextureStorage(textures_dir)` 实例，注入到 `SiteBackend` 和 `YggdrasilBackend`。`textures_dir` 仍来自 `config.get("textures.directory", "textures")`，只是读取点从 DB 模块移到组装根。

## 影响文件

- 新增：`services/texture_storage.py`、`services/__init__.py`
- 修改：`database_module/modules/texture.py`（删 `upload`、删多余 import、瘦 `__init__`）
- 修改：`backends/site_backend.py`（注入 storage，新增 `upload_texture_to_library`，改导入逻辑）
- 修改：`backends/yggdrasil_backend.py`（注入 storage，改 `upload_texture`）
- 修改：app 组装根（依赖注入）
- 修改：`routers/site_routes.py` 两个上传端点改调 backend（最小改动版）

## 测试

- `tests/database/test_texture.py`：移除/改写针对 `TextureModule.upload` 的用例（图像处理部分迁到新测试）。
- 新增 `tests/services/test_texture_storage.py`：覆盖 normalize/校验失败/hash 稳定性/落盘。
- `tests/backends/test_site_backend.py`、`test_yggdrasil_backend.py`、`test_site_backend_import.py`：更新对上传路径的 mock（现在 mock `texture_storage.process_and_save` + `db.texture.add_to_library`，而非 `db.texture.upload`）。
- `tests/api/test_site_api.py`：上传接口行为应保持不变（端到端契约）。

## 完成标准

- `database_module/modules/texture.py` 不再 import PIL / os（除路径拼接外）/ image_utils / config。
- `grep -rn "db.texture.upload\|\.texture\.upload(" skin-backend` 无残留。
- `pytest -q` 全绿，上传相关 API 行为与重构前一致。

## 回滚

本阶段为纯结构移动。若出问题，单 PR `git revert` 即可恢复 `TextureModule.upload`。
