# Access Token + Refresh Token 改造计划

## 1. 目标

把当前的"单一长效 JWT"鉴权，改造为标准的 **短效 access token + 长效可撤销 refresh token** 模式。
按既定要求：除数据库新增表 + 在 `init` 中写升级迁移外，其余代码**彻底改写**，不保留旧版兼容分支，
不写防御性兼容逻辑。

## 2. 现状（改造前）

- 登录后签发**单一无状态 JWT**（`utils/jwt_utils.create_jwt_token`），放入 httponly cookie `jwt`。
- payload = `{sub, is_admin, exp}`，有效期 = 管理面板设置 `jwt_expire_days`（默认 7 天）。
- **JWT 不入库**；`routers/deps.get_current_user` 每次请求查库校验"用户仍存在 + 以库内 is_admin 为准"
  （删号/降权即时生效；封禁**不**拦截站点，仅限制 Yggdrasil 游戏登录）。
- `tokens` 表是 **Yggdrasil 游戏令牌**，与站点 JWT 无关，本次**不动**。
- `/me/refresh-token` 已存在，但错误地要求有效 access token；前端 `refreshToken()` 已定义但从未调用。

## 3. 目标设计（已确认的决策）

| 项 | 决策 |
|---|---|
| 模式 | access token（无状态 JWT）+ refresh token（不透明随机串，入库可撤销） |
| access 有效期 | **30 分钟**（代码常量，可经 `config.get("jwt.access_expire_minutes", 30)` 覆盖，但**不改 config.yaml**） |
| refresh / 会话时长 | 沿用管理面板 `jwt_expire_days`（默认 7 天） |
| 轮换 | **每次刷新即轮换**：校验旧 refresh → 删除旧行 → 写入新行 → 同时下发新 access + 新 refresh |
| 前端 | **一并改造**：axios 响应拦截器在 401 时自动刷新并重试，失败跳 `/login` |
| refresh 存储 | 入库存 **SHA-256 哈希**（库泄露不致直接可用），与密码哈希同理；非兼容/防御逻辑 |
| cookie | 拆为两个 httponly cookie：`access_token`（max_age=30min）、`refresh_token`（max_age=expire_days） |
| 撤销事件 | 登出→撤销当前 refresh；改密/重置密码→撤销该用户全部 refresh；删号→级联删除。封禁**不**撤销（仅影响游戏登录） |

## 4. 数据库变更

### 4.1 新增表（`database_module/initsql.py`，加入 `INIT_SQL`）

```sql
CREATE TABLE IF NOT EXISTS site_refresh_tokens (
    token_hash TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    expires_at BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    FOREIGN KEY(user_id) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_site_refresh_user ON site_refresh_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_site_refresh_expires ON site_refresh_tokens (expires_at);
```

`CREATE TABLE IF NOT EXISTS` 本身即"旧库升级迁移"（新表，无需 ALTER）。放在 users 表定义之后、
保证 FK 目标已存在。索引：`user_id`（按用户批量撤销）、`expires_at`（清理过期）。

### 4.2 新增 DB 模块方法（`database_module/modules/user.py`，紧跟 Tokens 段后新增一段）

```python
# ========== Site Refresh Tokens ==========
async def add_refresh_token(self, token_hash, user_id, expires_at, created_at)
async def get_refresh_token(self, token_hash) -> Record | None   # 返回 user_id/expires_at
async def delete_refresh_token(self, token_hash)                 # 单条撤销（轮换/登出）
async def delete_refresh_tokens_by_user(self, user_id)           # 全部撤销（改密/重置）
async def delete_expired_refresh_tokens(self, cutoff)            # 清理过期
```

并在 `UserModule.delete()` 的事务里加 `DELETE FROM site_refresh_tokens WHERE user_id=$1`
（与现有 profiles/tokens/user_textures 删除并列）。

## 5. 后端代码改造（彻底改写，不留兼容）

### 5.1 `utils/jwt_utils.py` — 重写

- `create_jwt_token` 改名/改义为 **`create_access_token(user_id, is_admin)`**：仅签发 access，
  有效期 = `config.get("jwt.access_expire_minutes", 30)` 分钟（payload 加 `"type": "access"`）。
- 新增 **`generate_refresh_token() -> (raw, token_hash)`**：`secrets.token_urlsafe(48)` + `hashlib.sha256`。
- 新增 **`hash_refresh_token(raw) -> str`**：校验时复算哈希查库。
- `decode_jwt_token` → **`decode_access_token`**：校验签名 + `exp` + `type == "access"`，否则 None。
- cookie helper 拆成两个：
  - `get_access_cookie_settings()` → key=`access_token`，max_age=30min。
  - `get_refresh_cookie_settings()` → key=`refresh_token`，max_age=`jwt_expire_days*86400`，
    `path="/"`（注：refresh 端点为 `/me/refresh-token`，path 用 `/` 即可，无需收窄）。
- 删除旧的 `create_jwt_token` / `decode_jwt_token` / `get_cookie_settings` 旧名（调用方全部改新名，无别名兜底）。

### 5.2 `backends/site_backend.py`

- `login()`：校验密码后，**同时**签发 access（`create_access_token`）+ refresh（`generate_refresh_token`
  → `add_refresh_token`），返回 `{"access_token", "refresh_token", "user_id", "is_admin"}`。
  expire_days 仍读 `jwt_expire_days` 设置，用于 refresh 的 `expires_at` 与 cookie max_age。
- 新增 **`rotate_refresh_token(raw_refresh) -> dict`**（取代旧 `refresh_token(user_id)`）：
  1. `hash_refresh_token` 查库；查不到 / 已过期 → `raise HTTPException(401, "invalid refresh token")`（过期行顺手删）。
  2. 查 `user` 是否仍存在（删号即失效）→ 否则 401。
  3. **轮换**：`delete_refresh_token(old_hash)` → `generate_refresh_token` → `add_refresh_token`。
  4. 以**库内** `is_admin` 重新签发 access。
  5. 返回 `{"access_token", "refresh_token", "is_admin"}`。
- 新增 **`revoke_refresh_token(raw_refresh)`**（登出用）：哈希后 `delete_refresh_token`（找不到也无所谓，不抛）。
- `change_password()` / `reset_password()` 成功后调用 `delete_refresh_tokens_by_user(user_id)`，
  强制其它会话重新登录（改密应使旧会话失效）。

### 5.3 `routers/deps.py` — `get_current_user`

- 从 **`access_token`** cookie 取 token（不再是 `jwt`），用 `decode_access_token`。
- 其余查库逻辑（用户存在 / 以库内 is_admin 为准 / 封禁不拦截）**保持不变**。
- `admin_required` 不变。

### 5.4 `routers/site_routes.py`

- `/site-login`：set 两个 cookie（access + refresh），body 返回 `{user_id, is_admin}`。
- `/site-logout`：读 `refresh_token` cookie → `site_backend.revoke_refresh_token(...)`；
  delete 两个 cookie。
- **`/me/refresh-token` 改为不依赖 `get_current_user`**（access 已过期时也能调用）：
  读 `refresh_token` cookie → `site_backend.rotate_refresh_token(...)` → set 新的两个 cookie，
  body 返回 `{is_admin}`；无 cookie 或失败 → 401。
- 其余受保护路由不变（依旧 `Depends(get_current_user)`）。

### 5.5 其它

- `routes_reference.py`：可选地在 `lifespan` 启动时 `delete_expired_refresh_tokens(now)` 清理一次（轻量，
  非必须，但顺手做）。Microsoft OAuth 路由**不动**（它只在已登录态导入角色，不签发站点令牌）。
- `config.yaml` **不修改**（access 时长用代码默认 30min；如需覆盖才加 `jwt.access_expire_minutes`，本次不加）。

## 6. 前端改造（`element-skin/`）

### 6.1 `src/api/client.ts` — 加响应拦截器

```ts
// 401 → 调 /me/refresh-token（cookie 自动带上）→ 成功则重试原请求；
// 失败 / 刷新本身 401 → 跳 /login。用单个 in-flight Promise 去重并发刷新，
// 刷新端点自身 401 不再触发二次刷新（避免死循环）。
```

要点：`withCredentials` 已开启，refresh 走 cookie 无需手动取 token；
对 `/me/refresh-token`、`/site-login` 自身的 401 不做拦截重试。

### 6.2 `src/api/me.ts`

- `refreshToken()` 返回类型改为 `{ data: { is_admin: boolean } }`（不再返回 token，token 在 cookie）。
  此函数现在由拦截器内部调用。

### 6.3 `src/api/auth.ts` / 登录登出

- `siteLogin` 返回 `{ user_id, is_admin }`（与后端对齐）；逻辑基本不变（cookie 由后端 set）。
- `siteLogout`（`AppLayout.vue`）不变（后端负责撤销 + 清 cookie）。

## 7. 测试改造（`tests/`）

- `tests/conftest.py`：`auth_headers`/`admin_headers` 把 cookie 名从 `jwt` 改为
  `access_token`，token 由新的 `create_access_token` 生成。
- `tests/api/test_site_api.py` & `test_integration.py`：所有 `cookies={"jwt": ...}` →
  `{"access_token": ...}`；登录断言 `set-cookie` 含 `access_token=` 与 `refresh_token=`。
- 新增 `tests/api/test_refresh_token.py`：
  1. 登录拿到 refresh cookie → 调 `/me/refresh-token` → 200 且返回新 refresh（值不同 = 轮换）。
  2. 旧 refresh 轮换后再次使用 → 401（一次性）。
  3. access 过期（构造过期 access）但 refresh 有效 → 刷新成功。
  4. 登出后 refresh 失效 → `/me/refresh-token` 401。
  5. 改密后该用户全部 refresh 失效。
  6. 删号后 refresh 失效。
- 新增 `tests/database/`（或并入 `test_user.py`）：refresh token CRUD + 按用户撤销 + 过期清理。

## 8. 文档

- 更新 `docs/security/phase-4-auth-and-account.md`：补充 access+refresh 模型说明（封禁语义不变）。
- 本计划文件保留为实现依据。

## 9. 实施顺序与验证

1. DB：建表 + 模块方法（先跑 `tests/database` 验证 CRUD）。
2. `jwt_utils` 重写 → `site_backend` → `deps` → `site_routes`（后端自洽）。
3. 改测试 + 新增 refresh 测试，跑后端全量 `pytest -q`（需本地 PostgreSQL）。
4. 前端拦截器 + api 类型对齐，`npm run type-check`。
5. 一次提交（DB+后端+测试+前端+文档），分支 `dev`，不推送。

## 10. 影响文件清单

后端：`database_module/initsql.py`、`database_module/modules/user.py`、`utils/jwt_utils.py`、
`backends/site_backend.py`、`routers/deps.py`、`routers/site_routes.py`、`routes_reference.py`(可选)、
`tests/conftest.py`、`tests/api/test_site_api.py`、`tests/api/test_integration.py`、
`tests/api/test_refresh_token.py`(新)、`tests/database/test_user.py`、
`docs/security/phase-4-auth-and-account.md`。
前端：`src/api/client.ts`、`src/api/me.ts`、`src/api/auth.ts`、`src/api/types.ts`(如需调整 `LoginResponse`)。
