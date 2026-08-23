# Element Skin v4.0.0

> **重大版本升级 / BREAKING CHANGES**
>
> v4.0.0 是相对 v3.0.2 的身份、OAuth/OIDC、Webhook、站点 API、权限和部署整体升级。
>
> v4.0.0 的自动数据库升级只支持 **v3.0.2 → v4.0.0**。v2.x、v3.0.0 和 v3.0.1 站点必须先升级到 v3.0.2 并确认运行正常，再升级到 v4.0.0。
>
> **部署方式已经改变。** 相比 v3，v4 的完整部署必须同时运行主后端与独立的 Webhook Worker。Docker Compose 用户必须同步新版 `.env.example` 和 `docker-compose.yml`，不能只替换镜像版本。
>
> 升级前请完整备份 PostgreSQL 数据库、`./data` 中的 Yggdrasil/OIDC 密钥、材质与首页媒体文件、生产环境变量和当前 Compose 配置。

## 版本范围

本版本以 v3.0.2 为功能比较基线，自动迁移入口只面向正式发布的 v3.0.2 数据库结构。v3.0.2 之前的版本和开发期间的中间结构不属于兼容面，也不保留对应迁移路径。

产品版本与 API 协议版本相互独立：

- Element Skin 产品版本升级为 **v4.0.0**。
- 站点 JSON API 从 `/v1` 升级为 **`/v2`**。
- OAuth 2.1、OpenID Connect 与 Yggdrasil 继续使用各自的标准端点和协议格式。

本版本的用户可见变化主要包括：

- 新增统一的外部 OIDC 身份管理与第三方登录。
- 将 Microsoft 正版角色导入改为可持续管理的身份绑定与显式同步。
- 在现有 OAuth 应用体系中加入标准 OpenID Connect Provider 能力。
- 新增按应用权限投递、带签名和重试机制的 Webhook。
- 站点 JSON API 升级到 `/v2`，统一响应、错误、状态码与字段语义。
- 新增邮箱后缀策略，完善注册邀请码和公开注册配置。
- 更新细粒度权限、管理页面、Python SDK、开发者文档和部署方式。

## 重大变化

### 1. 统一外部 OIDC 身份

管理员可以配置多个兼容 OIDC Discovery 的外部身份提供方：

- 支持通用 OIDC 提供方和 Microsoft 能力适配器。
- 可配置名称、Issuer、Client ID、Client Secret、scope、图标和显示顺序。
- 可以分别控制提供方是否启用、是否允许第三方登录、是否允许已登录用户连接身份。
- 保存配置时会验证 Discovery 信息；远端端点必须使用安全的 HTTPS 地址。
- 回调地址由 `SERVER_API_URL` 统一生成，并在创建和编辑页展示。

用户新增独立的“身份管理”页面：

- 同一用户可以在同一个提供方下连接多个不同外部账号。
- 每个身份可以设置只在本站显示的标签。
- 身份页会展示头像、显示名称、邮箱、外部账号标识、授权状态及关联的正版角色。
- 用户可以添加、重新连接和删除身份。
- 重新连接时必须确认仍是原来的外部账号；选错账号不会替换既有身份。
- 外部 access token 只在实际调用远端能力时按需刷新。
- refresh token 或长期授权失效后，身份会标记为需要重新连接；原身份和正版绑定关系不会被静默删除。

外部登录不会绕过本站注册要求：

- 已连接身份可以直接登录对应本站账号。
- 没有匹配账户时不会按邮箱自动关联，也不会自动创建本站账号。
- 用户仍须填写本站用户名、邮箱、密码，并完成当前启用的邮箱验证码和邀请码校验。
- OIDC 返回的邮箱、显示名或 `email_verified` 不能代替本站账户字段与验证流程。

### 2. Microsoft 正版角色绑定与同步

旧版一次性“导入 Microsoft 角色”改为长期、可管理的正版角色绑定：

- Microsoft 连接与其他 OIDC 身份走同一套身份接口、授权状态和重新连接流程。
- 点击“绑定正版”时，用户需要从已经连接且状态正常的 Microsoft 身份中选择账号。
- 本站以远端正版 UUID 建立关系：已有自己的角色会复用，没有时会创建，已属于其他用户时会明确拒绝。
- 只有用户点击“同步正版数据”时，才同步远端角色名称、皮肤和披风。
- 同步前会提示将要覆盖的内容。
- 正版角色会在角色卡片、角色详情和身份管理页显示对应关系。

外部身份、正版绑定和本站角色保持独立：

- 角色仍然可以改名、编辑材质、清除材质或删除。
- 解除正版绑定会保留本站角色和已经同步的材质。
- 删除外部身份会解除其正版绑定，但不会删除本站角色。
- 删除角色只清理对应绑定，不会删除外部身份。

### 3. 本站支持标准 OpenID Connect Provider

现有 OAuth 客户端体系现在同时支持标准 OpenID Connect，不另建一套客户端或审核入口：

- 新增 OpenID Discovery、JWKS、RS256 ID Token 和 UserInfo。
- 支持 `openid`、`profile`、`email`、`offline_access` 标准 scope。
- `profile` 与 `email` claim 只有在应用申请且用户同意对应 scope 时才会提供。
- 用户标识使用按客户端隔离的 pairwise `sub`。
- OIDC scope 与 Element Skin `/v2` 权限分开申请和展示。
- 只需要“使用本站登录”的应用可以不申请任何站点 API 权限。
- Authorization Code 只有包含并获批 `offline_access` 时才签发 refresh token。
- 撤销授权后，关联 access token、refresh token 和 UserInfo 访问会失效。
- 授权确认页分别解释 OIDC scope 和站点 API 权限。
- 未登录用户完成登录后会返回原授权请求，不会丢失 OAuth/OIDC 参数。

第三方应用仍需登记精确回调地址并通过审核。Authorization Code 使用 PKCE S256，原有 Device Code、Client Credentials、Refresh Token、Revoke 和 Introspection 能力继续可用。

### 4. 站点 API 升级到 `/v2`

所有站点 JSON API 已从 `/v1` 迁移到 `/v2`。旧 `/v1` 和未版本化站点业务路径不再保留，自定义前端、脚本、机器人、SDK 和第三方集成必须同步更新。

本次变化不能通过简单替换 URL 中的 `v1` 完成：

- JSON 字段统一使用 `snake_case`，时间字段统一使用 Unix 毫秒时间戳。
- 单资源查询直接返回资源对象。
- 非分页集合统一返回 `{ "items": [...] }`。
- Cursor 分页统一返回 `items`、`has_next`、`next_cursor`、`page_size`。
- 创建资源统一使用 `201 Created`。
- 无返回内容的动作、幂等设置和删除统一使用 `204 No Content`。
- 不再返回只用于表达 HTTP 成功的 `ok`、`success`、`message` 或通用 `data` envelope。
- 材质资源字段使用 `type`，写入和筛选参数使用 `texture_type`。
- 明确提供但无效的 Cookie 或 Bearer 凭据返回 `401`，不会降级为访客。

普通 `/v2` 错误统一为：

~~~json
{
  "error": {
    "object": "resource",
    "operation": "read",
    "reason": "not_found",
    "params": {}
  }
}
~~~

`params` 为空时省略。OAuth/OIDC 标准端点仍返回 RFC 错误对象，Yggdrasil 仍返回协议规定的错误格式。

### 5. 权限模型调整

新增及替换后的身份和审核权限为：

~~~text
external_identity.read.owned
external_identity.create.owned
external_identity.update.owned
external_identity.delete.owned

official_profile.read.owned
official_profile.create.owned
official_profile.refresh.owned
official_profile.delete.owned

identity_provider.read.public
identity_provider.read.any
identity_provider.create.any
identity_provider.update.any
identity_provider.delete.any

oauth_app.review.any
~~~

- 旧 `microsoft_import.*` 权限已移除。
- OAuth 应用的通过、驳回、停用和重新启用由独立的 `oauth_app.review.any` 授权，不再隐含在 `oauth_app.update.any` 中。
- 内置角色已经更新；自定义角色和申请过旧权限的应用需要管理员重新检查。
- OIDC、OAuth、身份、正版角色和管理页面按细粒度权限展示入口与操作。
- 只拥有部分管理权限的用户可以进入对应页面，但只能查看和执行实际获授权的操作。

## OAuth 应用与 Webhook

### 1. 第三方应用管理

- 应用创建和编辑改为独立页面。
- 应用可以统一维护基本信息、公开或机密类型、OAuth 回调、OIDC scope、站点 API 权限和 Webhook endpoint。
- Authorization Code 应用必须配置回调；只使用 Device Code 或 Client Credentials 的应用可以不配置回调。
- OAuth 回调地址与 Webhook 接收地址是两种独立配置。
- 只使用 OIDC 登录的应用可以申请零项站点 API 权限。
- Client Secret 与 Webhook signing secret 生成后只显示一次。
- 同一用户对同一应用只保留一条有效授权；再次批准会更新原授权。

### 2. Webhook 事件

每个 OAuth 应用最多可以配置 5 个独立启停的 Webhook endpoint，并为每个 endpoint 选择事件。当前公开事件包括：

- 账户：`account.created`、`account.updated`、`account.deleted`
- OAuth 授权：`oauth_grant.created`、`oauth_grant.updated`、`oauth_grant.revoked`
- 正版白名单：`official_whitelist.added`、`official_whitelist.removed`
- 权限：`permission.updated`
- 角色：`profile.created`、`profile.updated`、`profile.deleted`
- 材质：`texture.created`、`texture.updated`、`texture.deleted`

`GET /v2/oauth/webhook-events` 返回事件说明，以及各事件可使用的用户委托权限和机密应用权限。

endpoint 的事件范围同时受以下条件约束：

- 应用类型和已审核权限。
- 用户授权 grant 与用户当前权限。
- 应用、endpoint 和授权是否仍然有效。

公开和机密应用均可订阅用户委托事件；需要 `*.read.any` 的管理型事件只允许机密应用通过 Client Credentials 订阅。

### 3. Webhook 安全与投递

- endpoint 只接受公网 HTTPS 地址。
- 不接受内网、回环或链路本地地址，不跟随 HTTP 重定向。
- Webhook 使用 `POST application/json`，事件仅携带事件类型、用户 UUID 和资源 ID 等基础标识。
- 接收方验签后应使用自己的 OAuth access token 调用 `/v2` 获取资源当前状态。
- 每个 endpoint 使用独立且只显示一次的 `signing_secret`。
- 签名算法为 `HMAC-SHA256(signing_secret, timestamp + "." + raw_body)`。
- 请求头为 `Webhook-Id`、`Webhook-Delivery`、`Webhook-Timestamp` 和 `Webhook-Signature`。
- 任意 `2xx` 响应视为成功，单次请求超时为 10 秒。
- 网络错误、超时或非 `2xx` 会退避重试，最多 12 次且总时长不超过 72 小时。
- Webhook 为至少一次投递，不保证全局顺序，也可能重复；接收方必须按事件 ID 实现业务幂等。

## 注册、邮箱后缀与邀请码

### 1. 邮箱后缀策略

邮件设置新增公共的账户邮箱后缀策略：

- 关闭：不限制邮箱后缀。
- 白名单：只允许完整列表中的后缀。
- 黑名单：拒绝完整列表中的后缀。
- 白名单和黑名单均返回不分页的完整列表。
- 后缀按忽略大小写的字面值匹配；例如 `@example.com` 不会自动匹配 `@sub.example.com`。
- 策略适用于注册和修改账户邮箱，不影响已有账户找回密码。

注册页和修改邮箱交互会读取公开策略：

- 白名单模式提供允许后缀的选择。
- 黑名单模式在前端直接提示不可用，不继续请求验证码。
- 前端预检只用于交互，后端仍执行最终校验。

### 2. 公开注册要求

公开站点配置明确返回：

- `allow_register`
- `require_invite`
- `email_verify_enabled`
- `email_suffix_policy`

需要邀请码时，注册页显示必填项并执行必填校验；不需要时不显示。公开注册配置加载失败时，前端会阻止提交，避免遗漏站点要求。

### 3. 任意字符串邀请码

- 邀请码支持包含空格、引号、斜杠、反斜杠和其他任意 UTF-8 字符。
- 邀请码按原文校验，保留大小写、空格和符号。
- 管理 API 创建时使用无填充 Base64URL 的 `code_base64` 字段。
- 管理 API 删除路径使用相同 Base64URL 编码。
- 用户注册时的 `invite` 字段仍提交邀请码原文。
- Python SDK 会透明完成管理接口所需编码。

## Python SDK 与开发者文档

- Python SDK 的站点客户端已迁移到 `/v2`，适配新状态码、空响应、分页结构和字段命名。
- `APIError` 公开 `status_code`、`object`、`operation`、`reason`、`params` 与原始响应。
- OAuth 标准错误继续由独立的 `OAuthError` 表达。
- OAuth `TokenSet` 新增 OIDC `id_token`。
- Authorization Code helper 支持 `nonce`。
- 新增 OIDC scope、外部身份、身份提供方和正版角色权限常量。
- 邀请码管理 wrapper 自动执行 Base64URL 编码，调用方仍传入原文。
- 新增 `WebhookVerifier`，验证必需请求头、原始请求体 HMAC、时间戳窗口和事件结构。
- 新增可插拔 `ReplayGuard` 与适合本地开发的 `MemoryReplayGuard`。
- 新增 FastAPI 和 Flask Webhook 接收示例。
- SDK 快速开始、OAuth 流程、权限模型、错误与 Token、API 客户端、OIDC 和 Webhook 文档已经更新。

## 前端与管理体验

- 新增用户身份管理页和 Microsoft 正版角色绑定选择流程。
- OIDC 提供方与 OAuth 应用使用独立创建、编辑页面。
- 管理员 OIDC 页新增“本站 OIDC 信息”卡片，集中展示本站 Issuer、Discovery、Authorization、Token、UserInfo 和 JWKS 地址，并引导管理员前往第三方应用页面登记或审核客户端。
- 提供方回调地址只在创建和编辑页展示，不在列表页重复显示。
- 移除旧 Microsoft 专用提示横幅。
- 应用授权页分别解释 OIDC scope 和站点 API 权限。
- 注册页会根据公开配置显示邮箱后缀、验证码和邀请码要求。
- 邀请码管理页补充任意字符、原文匹配和 Base64URL 传输提示。
- 站点设置、邮件设置、Mojang/Fallback 设置和彩蛋设置会提示未保存更改。

## 配置与部署

### 1. 部署方式已经改变

v3 的部署文件不能原样用于 v4。官方 Docker Compose 现在运行 4 个服务：

~~~text
db
redis
backend
webhook-worker
~~~

`backend` 与 `webhook-worker` 使用同一镜像，但入口不同：

- 主后端继续运行站点 HTTP 服务。
- Worker 使用镜像内的 `/app/webhook-worker`，不暴露端口。
- 新版 Compose 会自动启动两个进程。
- 自建 Docker、systemd、Kubernetes 或其他部署必须显式同时运行两者。
- 只运行主后端不会投递任何 Webhook，不能依赖主后端代替 Worker。

升级时必须同时同步仓库根目录的 `.env.example` 和 `docker-compose.yml`，然后把现有生产配置迁移到新 `.env`，不要只修改 `ELEMENT_SKIN_IMAGE`。

### 2. 新增必需配置

~~~env
OIDC_PRIVATE_KEY=/app/data/oidc-private.pem
OIDC_PUBLIC_KEY=/app/data/oidc-public.pem
IDENTITY_ENCRYPTION_KEY=<使用 openssl rand -base64 32 生成>
WEBHOOK_WORKER_MAX_DATABASE_CONNECTIONS=2
WEBHOOK_WORKER_ACTIVE_INTERVAL_MS=3000
~~~

- OIDC 公私钥文件在配置路径不存在时会自动生成。
- 必须持久化 `./data`，不得删除或替换已投入使用的 Yggdrasil 或 OIDC 私钥。
- `IDENTITY_ENCRYPTION_KEY` 不会自动生成。
- `IDENTITY_ENCRYPTION_KEY` 用于保护外部身份 refresh token 和 OIDC client secret，启用相关功能后必须长期保持不变。
- Worker 的数据库连接数和轮询间隔可通过对应环境变量调整。

### 3. Microsoft 配置迁移

使用旧 Microsoft 导入功能的站点需要在 Azure/Entra 应用中加入新的 Web 回调：

~~~text
${SERVER_API_URL}/v2/auth/oidc/callback
~~~

升级时：

- 完整的 `microsoft_client_id` 和 `microsoft_client_secret` 会迁移为只开放连接能力的 Microsoft OIDC 提供方。
- 只有新提供方创建成功后才会移除旧设置；配置残缺或远端验证失败会终止启动并保留原值。
- 已导入的本站角色、皮肤和披风保持不变。
- 旧流程没有保存用户 Microsoft refresh token，因此用户必须重新连接 Microsoft 身份，之后才能建立正版绑定和同步远端角色。

## 升级步骤

1. 确认当前站点运行正式发布的 v3.0.2。
2. 如果当前不是 v3.0.2，先按照 v3.0.2 的升级说明完成迁移，并验证站点可以正常启动、登录和访问数据；不要从其他版本直接升级 v4。
3. 停止写入，并完整备份 PostgreSQL 数据库、`./data`、材质与首页媒体目录、生产 `.env` 和旧 `docker-compose.yml`。
4. 获取 v4.0.0 的 `.env.example` 与 `docker-compose.yml`，将生产配置迁移到新 `.env`。
5. 使用 `openssl rand -base64 32` 生成一次 `IDENTITY_ENCRYPTION_KEY` 并安全保存。
6. 检查 `OIDC_PRIVATE_KEY`、`OIDC_PUBLIC_KEY` 和 Webhook Worker 配置。
7. 使用 Microsoft 功能时，在 Azure/Entra 应用中加入 `${SERVER_API_URL}/v2/auth/oidc/callback`。
8. 拉取并启动新版服务：

~~~bash
docker compose pull
docker compose up -d
~~~

9. 同时检查 `backend` 与 `webhook-worker` 日志，确认数据库升级、OIDC 密钥加载、Microsoft 配置迁移和 Worker 启动成功。
10. 验证本站登录与注册、外部身份连接、正版同步、OAuth/OIDC 授权、`/v2` API、Webhook 投递和管理员细粒度权限。
11. 更新所有第三方脚本、机器人和客户端；旧 `/v1` 调用不能继续使用。

## 回滚说明

v4.0.0 会改变数据库结构、部署进程和外部集成协议。需要回滚时：

1. 停止 v4 的 `backend` 与 `webhook-worker`。
2. 恢复升级前完整备份的 v3 PostgreSQL 数据库。
3. 恢复升级前的 v3 `.env`、Compose 配置、密钥和数据目录。
4. 再启动原 v3 镜像。

不要让 v3 后端直接连接已经完成 v4 升级的数据库，也不要把 v4 生成的新配置与未恢复的旧数据库混合使用。

## Full Changelog

https://github.com/water2004/element-skin/compare/v3.0.2...v4.0.0
