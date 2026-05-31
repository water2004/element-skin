# 阶段 5：抽出 SettingsBackend + 领域对象清理

## 目标

两块低风险收尾：

1. 抽出 **`SettingsBackend`**，收敛分散的设置默认值，消除 backend 与 router 间重复的默认值。
2. 修复 `utils/typing.py` 的命名不一致、潜在 bug，以及 DB 层按下标重排构造对象的脆弱写法。

## 部分 A：SettingsBackend

### 当前问题（证据）

设置处理散在 `admin_backend.py`：
- `get_site_settings` / `get_security_settings` / `get_auth_settings` / `get_microsoft_settings` / `get_email_settings` / `get_fallback_settings`（19-76 行）
- `save_settings_group`（78-128 行）、`_validate_fallback_services`（150-200 行，50 行纯校验）
- legacy 的 `get_admin_settings` / `save_admin_settings`（132-146 行）

**默认值重复且有不一致风险**：同一批默认值在两处硬编码：
- `admin_backend.get_site_settings`（19-34 行）：`"皮肤站"`、`"简洁、高效..."`、`max_texture_size="1024"` 等。
- `site_routes.py:425-445` `/public/settings`：又写了一遍 `"皮肤站"`、`"简洁、高效..."` 等默认值。

两处若改一处忘改另一处，公开页和管理页会显示不同默认值。

### 设计

新增 `backends/settings_backend.py`，迁移上述所有设置方法。集中定义默认值表：

```python
# settings_backend.py
SETTING_DEFAULTS = {
    "site_name": "皮肤站",
    "site_subtitle": "简洁、高效、现代的 Minecraft 皮肤管理站",
    "max_texture_size": "1024",
    "jwt_expire_days": "7",
    # ... 单一事实来源
}

class SettingsBackend:
    def __init__(self, db: Database):
        self.db = db

    async def get_public_settings(self) -> dict: ...   # 供 /public/settings（阶段4 的占位在此落地）
    async def get_site_settings(self) -> dict: ...
    async def get_security_settings(self) -> dict: ...
    async def get_auth_settings(self) -> dict: ...
    async def get_microsoft_settings(self) -> dict: ...
    async def get_email_settings(self) -> dict: ...
    async def get_fallback_settings(self) -> dict: ...
    async def save_settings_group(self, group, body): ...
    def _validate_fallback_services(self, services): ...
```

`/public/settings` 与各 admin getter 都从 `SETTING_DEFAULTS` 取默认，消除重复。

`admin_backend` 改为持有/委托 `SettingsBackend`（组合而非继承），或直接在组装根把设置端点指向 `SettingsBackend`。legacy 的 `get_admin_settings` / `save_admin_settings` 标注 `# deprecated`，确认前端无引用后可删（见下）。

> **实现备注（与文档差异）**：执行时核查确认前端与 router 均无 `get_admin_settings` / `save_admin_settings` 引用，因此**直接删除**这两个方法，未走"标注 `# deprecated`"中间态——按全局约束（无兼容性代码），无引用的方法即死代码，应删不应留。

### 前端引用核查

删 legacy 方法前，确认 `element-skin/src/api/admin/settings.ts` 调用的是分组端点还是整体端点：

```bash
grep -rn "admin.*settings" element-skin/src/api/
```

若仍用整体端点，保留 legacy 方法；否则移除。

## 部分 B：领域对象清理（utils/typing.py）

### B1. `Texture.to_json` 缺失 return（潜在 bug）

`typing.py:18-41`：方法构建了 `textures_payload` 但**没有 `return`**，隐式返回 `None`。需确认调用方（`PlayerProfile.to_json` → `texture.to_json(...)`，104-107 行）实际依赖返回值。

```bash
grep -rn "\.to_json(" skin-backend
```

- 若 Yggdrasil 材质响应确实走这条路径，这是真 bug，补 `return textures_payload`，并加测试断言 SKIN/CAPE 结构。
- 若该路径未被实际使用（死代码），记录并考虑删除。
> **先查清再动**：本阶段不臆断，先 grep 调用链确认。

### B2. 命名不一致

`User` 类同时有 `preferredLanguage`（camelCase，51/66 行）和 `display_name` / `is_admin`（snake_case）。统一为 snake_case（`preferred_language`），并全局改引用：

```bash
grep -rn "preferredLanguage" skin-backend
```

影响点：`site_backend.get_user_info`（337 行 `user_row.preferredLanguage`）、`admin_backend.get_user_info`（214 行）、`yggdrasil_backend`（153 行）、`typing.py` 自身的 `to_json`。
> 这是纯内部重命名，不改 API 输出字段名（API 里若对外是 `preferredLanguage` 则保持响应键不变，仅改 Python 属性名）。

### B3. 按下标重排构造 User（脆弱）

`user.py:103, 148`：

```python
items = [User(r[0], r[1], "", r[3], r[5], r[2], r[4], r[6]) for r in rows[:limit]]
```

这种"按 SELECT 列下标手动重排到构造参数"极易因列顺序变动而错位，且不可读。改造选项：
- **首选**：用关键字参数构造 `User(id=r[0], email=r[1], password="", is_admin=r[3], display_name=r[2], ...)`，列顺序变动时仍正确。
- 或：让 `User` 提供 `from_row(row, columns)` 类方法。

本阶段采用关键字参数构造，最小改动、即时可读。

## 影响文件

- 新增：`backends/settings_backend.py`
- 修改：`backends/admin_backend.py`（移除设置方法，委托/删除 legacy）
- 修改：`routers/site_routes.py`（`/public/settings` 走 `SettingsBackend.get_public_settings`）
- 修改：`routers/admin_routes.py`（设置端点指向 `SettingsBackend`）
- 修改：`utils/typing.py`（修 `to_json` return、`preferredLanguage`→`preferred_language`）
- 修改：`database_module/modules/user.py`（关键字参数构造 `User`）
- 修改：所有引用 `preferredLanguage` 的 backend
- 组装根：实例化 `SettingsBackend`

## 测试

- 新增 `tests/backends/test_settings_backend.py`：默认值回退、分组保存、`_validate_fallback_services` 边界（非 list、缺 url、负 cache_ttl）。
- `tests/api/test_site_api.py`：`/public/settings` 默认值与重构前一致。
- `tests/api/test_admin_api.py`：设置读写契约不变。
- 新增/更新 `Texture.to_json` 的单测（若 B1 确认为 bug）。
- `tests/database/test_user.py`：分页构造的 `User` 字段映射正确（关键字构造后尤其要验证 `display_name` / `is_admin` 没错位）。

## 完成标准

- `"皮肤站"` 等默认值字符串在代码中只出现在 `SETTING_DEFAULTS` 一处。
- `grep -rn "preferredLanguage" skin-backend` 仅在"对外 API 响应键"处保留（若有），Python 属性统一 snake_case。
- `user.py` 无 `User(r[0], r[1], "", r[3], ...)` 式按下标重排构造。
- `Texture.to_json` 行为明确（修复或删除，二选一，有记录）。
- `pytest -q` 全绿。

## 风险与回滚

风险最低（多为重命名与默认值收敛）。可作为整个重构的"热身"提前到阶段 1 之前做，也可按 README 顺序在阶段 3 之后做。拆成两个 commit：A（SettingsBackend）、B（typing 清理），互不依赖。
