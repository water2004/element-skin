# 阶段 3：拆分 SiteBackend，抽出 ProfileImportBackend

## 目标

`backends/site_backend.py`（555 行）单个类承担 5+ 个领域。本阶段：

1. 抽出 **`ProfileImportBackend`**（远程 Yggdrasil 导入），这是耦合度最低、最该独立的部分。
2. 收敛重复代码：唯一角色名生成、角色名校验正则。

> 设置管理（`get_*_settings` 等）在 `admin_backend`，不在本文件，留到阶段 5。

## 当前问题（证据）

`SiteBackend` 混入的领域：

- 认证：`login` / `register` / `refresh_token` / `reset_password` / `change_password`（213-446 行）
- 邮箱验证码：`send_verification_code` / `verify_code`（165-211 行）
- 用户管理：`get_user_info` / `update_user_info` / `delete_user`
- 角色管理：`create_profile` / `update_profile` / `delete_profile` / `clear_profile_texture` / `apply_texture_to_profile`（450-538 行）
- **远程 Yggdrasil 导入**：`get_ygg_profiles` / `import_ygg_profile` / `import_ygg_profiles` / `_import_single_ygg_profile`（42-163 行，约 120 行）
- 杂项：`list_carousel_images` / `get_fallback_services`

重复代码：

- **唯一角色名生成循环**（`while + suffix++ + > 100` 上限）：
  - `_import_single_ygg_profile`（66-74 行）
  - `register`（293-301 行）
- **角色名校验正则** `^[a-zA-Z0-9_]{1,16}$`：
  - `create_profile`（454 行）、`update_profile`（480 行）
  - `admin_backend.update_profile`（255 行，写法略不同）

## 设计

### 1. 抽出 `ProfileImportBackend`

```
backends/
  profile_import_backend.py   # ProfileImportBackend
```

迁移方法（从 `site_backend.py` 移出）：
- `get_ygg_profiles`
- `import_ygg_profile`
- `import_ygg_profiles`
- `_import_single_ygg_profile`

依赖：`db`（建角色、查重、记材质元数据）、阶段 1 引入的 `TextureStorage`（下载的皮肤要落盘）、`YggdrasilClient` / `download_texture`。构造：

```python
class ProfileImportBackend:
    def __init__(self, db: Database, texture_storage: TextureStorage):
        self.db = db
        self.texture_storage = texture_storage
```

`_import_single_ygg_profile` 内部当前调 `self.db.texture.upload(...)`（site_backend.py:84,95）。阶段 1 已删除 `db.texture.upload`，这里改为 `texture_storage.process_and_save(...)` + `db.texture.add_to_library(...)`。

> 顺序依赖：阶段 3 假定阶段 1 已完成。若先做 3，需在本阶段临时保留旧 upload 调用，徒增返工，故遵循 README 的 1→2→3 顺序。

唯一角色名生成此处复用下面抽出的公共函数。

### 2. 抽出公共工具

新增 `utils/profile_naming.py`（纯函数，无状态，可单测）：

```python
import re

PROFILE_NAME_RE = re.compile(r"^[a-zA-Z0-9_]{1,16}$")

def is_valid_profile_name(name: str) -> bool:
    return bool(name) and bool(PROFILE_NAME_RE.match(name))

async def generate_unique_profile_name(base: str, exists: Callable[[str], Awaitable[bool]],
                                       max_attempts: int = 100) -> str:
    """base 被占用时尝试 base_1, base_2 ...；超出 max_attempts 抛 ValueError。
    exists: async 谓词，传入候选名返回是否已存在。"""
    candidate = base
    suffix = 1
    while await exists(candidate):
        candidate = f"{base}_{suffix}"
        suffix += 1
        if suffix > max_attempts:
            raise ValueError("无法生成唯一的角色名称")
    return candidate
```

调用方传入 `exists=lambda n: db.user.get_profile_by_name(n) is not None` 形态的谓词（注意包装成返回 bool 的 async）。

替换点：
- `site_backend.register`（290-301 行）
- `ProfileImportBackend._import_single_ygg_profile`（原 66-74 行）
- `site_backend.create_profile` / `update_profile` 的正则校验 → `is_valid_profile_name`
- `admin_backend.update_profile`（255 行）→ `is_valid_profile_name`

> 校验失败的报错文案保持原样（各处中文提示不变），只把"判断逻辑"收敛，不改对外行为。

## 组装根改造

在 app 启动处实例化 `ProfileImportBackend(db, texture_storage)`，注入到需要它的 router（`/remote-ygg/*` 三个端点，目前在 `site_routes.py:124-151`）。`site_routes` 的 `setup_routes` 增加一个 `profile_import_backend` 参数。

## 影响文件

- 新增：`backends/profile_import_backend.py`
- 新增：`utils/profile_naming.py`
- 修改：`backends/site_backend.py`（移除 4 个导入方法 + 复用命名工具，预计 -120 行）
- 修改：`backends/admin_backend.py`（`update_profile` 用 `is_valid_profile_name`）
- 修改：`routers/site_routes.py`（`/remote-ygg/*` 改调 `profile_import_backend`，`setup_routes` 加参）
- 修改：组装根（实例化与注入）

## 测试

- `tests/backends/test_site_backend_import.py`：目标类从 `SiteBackend` 改为 `ProfileImportBackend`，构造参数同步（注入 `texture_storage`）。
- 新增 `tests/utils/test_profile_naming.py`：覆盖正则边界（空、超 16、含非法字符）、唯一名生成（首选可用、需加后缀、超上限抛错）。
- `tests/backends/test_site_backend.py`：`register` / `create_profile` 用例确认行为不变。
- `tests/api/test_site_api.py`：`/remote-ygg/*` 端到端契约不变。

## 完成标准

- `site_backend.py` 不再包含 `get_ygg_profiles` / `import_ygg_profile(s)` / `_import_single_ygg_profile`。
- `^[a-zA-Z0-9_]{1,16}$` 在 backend 代码中只出现在 `utils/profile_naming.py` 一处。
- `while`+`suffix`+`> 100` 唯一名循环只出现在 `utils/profile_naming.py` 一处。
- `pytest -q` 全绿。

## 风险与回滚

中等风险，主要在依赖注入接线。建议两个 commit：先抽 `utils/profile_naming.py` 并替换（小、纯函数、易验证），再抽 `ProfileImportBackend`。
