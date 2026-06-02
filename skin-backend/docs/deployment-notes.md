# 部署运营注记

本文件集中记录**配置/部署层**的安全须知。这些事项不在本轮代码改动范围内——
因为 `config.yaml` 不在本轮修改、且大多属于反向代理/部署职责——但上线前必须逐项确认。

> 与代码侧安全修复（见 `docs/security/phase-*.md`）互补：代码堵住可在应用内修复的漏洞，
> 本文件覆盖只能在部署层正确配置的部分。

## 1. CORS

- 当前默认 `cors.allow_origins=["*"]` 且 `cors.allow_credentials=true`（见 `routes_reference.py` 的 CORS 中间件，取自 `config.yaml`）。
- Starlette 在 `allow_credentials=true` 下会**反射**请求的 `Origin` 并回 `Access-Control-Allow-Credentials: true`，等价于对任意站点放行带凭证的跨站请求。
- 目前靠 Cookie 的 `SameSite=Lax` 兜底。**一旦放宽 SameSite 或改用 header 鉴权，此配置会被直接利用。**
- **生产要求**：把 `cors.allow_origins` 设为明确白名单（你的前端域名），**不可** `*` 与 `allow_credentials: true` 并用。

## 2. Cookie Secure

- token cookie 是否带 `Secure` 由 `server.site_url` 是否以 `https://` 开头决定（见 `utils/jwt_utils.py`）。
- 典型坑：反向代理终止 TLS（对外 https），但后端 `server.site_url` 仍写 `http://` → 签发的 cookie **不带 `Secure`**，可能经明文信道泄露。
- **生产要求**：`server.site_url` 必须为 `https://`；或在反代层统一对 Set-Cookie 追加 `Secure`。

## 3. CSRF

- 目前无 CSRF token，状态变更端点仅靠 Cookie `SameSite=Lax` 兜底。
- `SameSite=Lax` 能挡多数跨站写请求，但不是完整 CSRF 防护。
- **若未来放宽 SameSite 或引入跨站场景**：需引入 CSRF token 或严格的 `Origin` 校验。

## 4. 单实例约束

以下状态均为**进程内**，跨进程不共享：

- OAuth state / 一次性导入 token（`routers/microsoft_routes.py` 的 `InMemoryStateStore`）
- settings 缓存 / fallback 缓存
- 过期 refresh token 的周期清理任务（`routes_reference.py` 的 `_refresh_cleanup_loop`）

**因此本服务只能单实例 / 单 worker 运行。** 多实例/多 worker 会导致 OAuth state 丢失、
缓存不一致、清理任务重复。若需横向扩展，须先把上述状态外置（如 Redis）。

## 5. 限流（应由 nginx 在更底层先行排除）

- 应用层限流以 `request.client.host` 为 key。在 `forwarded_allow_ips="*"` 下，
  客户端可伪造 `X-Forwarded-For` 绕过。
- **异常流量应由 nginx 在更底层先行排除**（连接数限制、限速、封禁），应用层不承担重限流职责。
- **生产要求**：`forwarded_allow_ips` 应设为真实反代网段，而非 `*`。

---

> 本轮**不修改 `config.yaml`**，以上仅为运营须知，落地时由部署方在配置/反代层执行。
