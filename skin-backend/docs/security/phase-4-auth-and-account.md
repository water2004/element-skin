# 阶段 4：站点侧封禁/降权拦截 + 弱口令策略 + 改邮箱校验 + 降低枚举

## 目标

修复无状态 JWT 与账号状态之间的脱节，并补齐若干账号安全细节：

1. **封禁用户能被站点 API 拦截**（当前只有游戏端 `has_joined` 检查封禁）。
2. **降权/封禁尽量及时生效**，缩小 JWT 过期前的「权限滞后」窗口。
3. 弱口令策略真正有效（当前 6 位、复杂度判断易绕过）。
4. 改邮箱做格式与唯一性预检，避免未捕获 500。
5. 降低注册/登录的用户枚举差异。

## 问题证据

- `routers/deps.py`：`get_current_user` / `admin_required` 仅解析 JWT，**不查用户当前 `banned_until` / `is_admin`**。被封用户、被降权的前管理员在 token 过期（默认 7 天）前畅通无阻。封禁检查只在 `backends/yggdrasil_backend.py:307`（`has_joined`）。
- `utils/password_utils.py:52` `validate_strong_password`：最低 6 位；复杂度判断 `(has_upper+has_lower+has_digit)==1 and not has_special` —— 如 `aaa111`（两类）直接判定为「不弱」。且默认 `enable_strong_password_check=false`。bcrypt 对 >72 字节静默截断，未预处理。
- `backends/site_backend.py:372-373` `update_user_info`：`update_email` 不校验格式、不预检唯一性。撞 `users.email UNIQUE` → asyncpg 抛异常 → **未捕获 500**（不像 `register` 那样 try/except）。
- 枚举差异：
  - `register`：邮箱已存在返回明确 "Email already registered"（`site_backend.py:201`）。
  - `login`：邮箱不存在时直接 401 返回（`site_backend.py:231`），不执行 bcrypt；存在时执行 bcrypt 才返回 → 存在**时序侧信道**。
  - `reset` 路径已正确（不存在也返 `{"ok": True}`）。

## 设计决策

- **封禁/降权及时性**：在 `get_current_user` 中增加一次用户查询，校验 `banned_until` 并以 DB 的 `is_admin` 为准（覆盖 JWT 里的旧值）。代价是每个鉴权请求多一次 `get_by_id`。考虑到 `db.setting` 已用内存缓存模式，可接受；若担心热路径开销，可加一个短 TTL 的用户状态缓存（本阶段先用直查，简单可靠，缓存留作后续优化）。
- **密码策略**：提高下限、修正复杂度逻辑、限制最大长度（防 bcrypt 截断歧义）。是否默认开启交由业主决定；本阶段先把策略本身改对。
- **枚举**：登录路径无论邮箱是否存在都执行一次 bcrypt（对齐时序），统一返回相同错误文案。注册的存在性提示较难完全消除（需要明确告知用户邮箱已注册以保证体验），保持现状但记录权衡；可选改为「发邮件提示」式的弱化，超出本阶段范围。

## 改造清单

### 4.1 站点鉴权拦截封禁 + 以 DB 为准的 admin

`routers/deps.py` 改为 async 查询用户状态。注意 `get_current_user` 已是 async，可直接注入 `db`（通过闭包/依赖）。由于 `deps.py` 当前无 `db` 句柄，方案二选一：

- 方案 A（推荐）：把 `db` 作为模块级可注入对象。`routes_reference.py` 在启动时 `deps.bind_db(db)`，`deps` 用该引用。
- 方案 B：将 `get_current_user` 改造成依赖工厂 `make_get_current_user(db)`，在各 `setup_routes` 注入（改动面更大）。

采用方案 A：

```python
# routers/deps.py
from fastapi import Request, HTTPException, Depends
import time
from utils.jwt_utils import decode_jwt_token

_db = None
def bind_db(db):
    global _db
    _db = db

async def get_current_user(request: Request) -> dict:
    token = request.cookies.get("jwt")
    if not token:
        raise HTTPException(status_code=401, detail="not authenticated")
    payload = decode_jwt_token(token)
    if not payload:
        raise HTTPException(status_code=401, detail="invalid or expired token")

    user = await _db.user.get_by_id(payload.get("sub"))
    if not user:
        raise HTTPException(status_code=401, detail="user not found")
    # 封禁拦截（banned_until 为毫秒时间戳）
    if user.banned_until and int(time.time() * 1000) < user.banned_until:
        raise HTTPException(status_code=403, detail="account is banned")
    # 以 DB 的 is_admin 为准，修正可能过期的 JWT 声明
    payload["is_admin"] = bool(user.is_admin)
    return payload
```

`admin_required` 不变（读 `payload["is_admin"]`，现在已是 DB 真值）。

`routes_reference.py` 初始化段调用 `deps.bind_db(db)`（在 include_router 之前）。

> 行为变化：被封用户调用任意需要登录的接口将得 403；被降权管理员立即失去后台权限。这是预期的安全收紧。

### 4.2 密码策略

`utils/password_utils.py:validate_strong_password` 重写：

```python
def validate_strong_password(password: str) -> list[str]:
    errors = []
    if len(password) < 8:
        errors.append("密码长度至少 8 位")
    if len(password.encode("utf-8")) > 72:
        errors.append("密码过长（不超过 72 字节）")  # bcrypt 限制
    classes = sum([
        bool(re.search(r"[A-Z]", password)),
        bool(re.search(r"[a-z]", password)),
        bool(re.search(r"\d", password)),
        bool(re.search(r"[^\w\s]", password)),
    ])
    if classes < 2:
        errors.append("密码需包含至少两类字符（大写/小写/数字/符号）")
    return errors
```

> 是否默认开启 `enable_strong_password_check` 由业主定。建议生产开启。

### 4.3 改邮箱校验

`backends/site_backend.py:update_user_info` 的 email 分支：

```python
if "email" in data and data["email"]:
    new_email = data["email"].strip()
    if not re.match(r"[^@]+@[^@]+\.[^@]+", new_email):
        raise HTTPException(status_code=400, detail="Invalid email format")
    user_row = await self.db.user.get_by_id(user_id)
    if user_row and user_row.email != new_email:
        existing = await self.db.user.get_by_email(new_email)
        if existing:
            raise HTTPException(status_code=400, detail="Email already in use")
        await self.db.user.update_email(user_id, new_email)
```

> 仍存在并发竞态（两请求同时通过预检），但 `UNIQUE` 约束兜底。为彻底避免 500，可在 `user.update_email` 捕获 `asyncpg.UniqueViolationError` 转 400（参考 `update_profile_name` 的写法）。建议两者都做。

### 4.4 登录时序对齐 + 统一文案

`backends/site_backend.py:login`：邮箱不存在时也执行一次 bcrypt（用一个固定的 dummy hash），再统一返回 401：

```python
_DUMMY_HASH = "$2b$12$" + "x" * 53   # 一个合法格式的 bcrypt 占位 hash

async def login(self, email, password):
    user_row = await self.db.user.get_by_email(email)
    if not user_row:
        verify_password(password, _DUMMY_HASH)   # 消耗与真实校验相近的时间
        raise HTTPException(status_code=401, detail="Invalid credentials")
    if not verify_password(password, user_row.password):
        raise HTTPException(status_code=401, detail="Invalid credentials")
    ...
```

> dummy hash 需是 bcrypt 能正常处理（返回 False）的合法格式串，建议用 `hash_password("dummy")` 在模块加载时生成一个常量，避免手写格式出错。

## 影响文件

- 修改：`routers/deps.py`（封禁拦截 + DB admin）、`routes_reference.py`（`deps.bind_db(db)`）
- 修改：`utils/password_utils.py`（策略重写）
- 修改：`backends/site_backend.py`（改邮箱校验、登录时序）
- 修改：`database_module/modules/user.py`（可选：`update_email` 捕获 UniqueViolation）

## 测试与验证

- 封禁：给用户设 `banned_until` 为未来 → 其 `/me`、上传等返回 403；解封后恢复。
- 降权：管理员 A 降权管理员 B 后，B 用旧 cookie 访问 `/admin/*` 立即得 403（无需等 JWT 过期）。
- 密码策略：`abc`（短）、`aaaaaaaa`（单一类）→ 报错；`Abc12345` → 通过；72 字节以上 → 报错。
- 改邮箱：非法格式 → 400；改成他人已用邮箱 → 400（非 500）；并发撞约束 → 400。
- 登录枚举：对「不存在的邮箱」与「存在但密码错」测响应时间分布，差异显著缩小；两者文案一致均为 "Invalid credentials"。
- 回归：`tests/api`、`tests/backends/test_auth_logic.py`、`test_site_backend.py` 需更新——尤其 `get_current_user` 现在依赖 DB，测试需在已建用户的前提下持有有效 token（conftest 的 `auth_headers`/`admin_headers` 已先建用户再发 token，基本兼容；但「持 token 但用户已删」类用例需调整预期为 401）。
- `pytest -q` 全绿。

## 风险与回滚

中风险，主因是 **4.1 改变了鉴权热路径**（每请求多一次 DB 查询）与**测试假设**（token 不再独立于 DB 用户存在）。若性能敏感，后续可加用户状态短 TTL 缓存。各子项相对独立：4.2/4.3/4.4 可与 4.1 分开提交、独立 revert。
