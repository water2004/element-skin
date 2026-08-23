# Element Skin v4.0.0

> **v4.0.0 是相对 v3.0.2 的不兼容功能更新。**
>
> 本版本建立了统一的外部身份与正版角色体系，为本站 OAuth 增加标准 OpenID Connect 和 Webhook，
> 并将站点 JSON API 升级为 `/v2`。产品版本是 v4.0.0，API 协议版本仍为 v2，两者相互独立。

---

## 升级前必读

### 站点 API

* 所有站点 JSON API 已从 `/v1` 迁移至 `/v2`，旧 `/v1` 及未版本化的站点路径不再保留。自定义前端、
  脚本、机器人和第三方集成必须同步升级。
* `/v2` 同时调整了成功状态码、集合与分页结构、空响应、错误对象及部分字段命名；不能只把请求路径中的
  `v1` 替换成 `v2`。
* OAuth 2.1、OpenID Connect 和 Yggdrasil 不属于站点 JSON API，继续使用各自的标准端点和响应协议。

### 部署配置

* Docker Compose 用户必须同步新版 `.env.example` 与 `docker-compose.yml`，新增以下配置：

  ```env
  OIDC_PRIVATE_KEY=/app/data/oidc-private.pem
  OIDC_PUBLIC_KEY=/app/data/oidc-public.pem
  IDENTITY_ENCRYPTION_KEY=<使用 openssl rand -base64 32 生成>
  WEBHOOK_WORKER_MAX_DATABASE_CONNECTIONS=2
  WEBHOOK_WORKER_ACTIVE_INTERVAL_MS=3000
  ```

* OIDC 签名密钥文件在指定路径不存在时会自动生成。请继续持久化 `./data`，并且不要删除或替换已经投入
  使用的 Yggdrasil 或 OIDC 私钥。
* `IDENTITY_ENCRYPTION_KEY` 不会自动生成。它用于保护外部身份授权信息和 OIDC 客户端密钥，配置身份
  提供方后必须长期保持不变。
* 新版 Compose 会自动启动 Webhook 投递服务。自建部署若需要使用 Webhook，必须同时运行镜像中的
  `/app/webhook-worker`；只启动主后端不会发送 Webhook。

### Microsoft 升级

* 使用 Microsoft 正版账号功能的站点，需要在 Azure/Entra 应用中加入新的 Web 回调地址：

  ```text
  ${SERVER_API_URL}/v2/auth/oidc/callback
  ```

* 完整的旧版 `microsoft_client_id` 与 `microsoft_client_secret` 会在升级时转为只开放绑定能力的
  Microsoft OIDC 身份提供方；残缺或无法验证的旧配置需要管理员修正后才能启动。
* 旧版已导入的本站角色、皮肤和披风保持不变。由于旧流程没有保存用户的长期 Microsoft 授权，用户需要
  重新连接 Microsoft 身份，之后才能建立正版绑定并同步远端角色。

### 权限与邀请码集成

* 旧 `microsoft_import.*` 权限已移除，替换为 `external_identity.*` 与 `official_profile.*`。内置用户和
  管理员权限已更新，自定义角色及申请过旧权限的 OAuth 应用需要重新检查权限配置。
* OAuth 应用审核新增独立的 `oauth_app.review.any`。通过、驳回、停用和重新启用应用不再由
  `oauth_app.update.any` 隐式授权。
* 邀请码管理 API 不再接收明文 `code` 字段或明文删除路径，改用 UTF-8 字节的无填充 Base64URL；注册
  API 中的 `invite` 仍然提交邀请码原文。

新增及替换后的身份相关权限为：

```text
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
```

---

## 外部 OIDC 身份

### 管理员配置

* 新增 OIDC 身份提供方管理。管理员可以配置任意兼容 OIDC Discovery 的第三方端点，也可以选择
  Microsoft 适配器以启用本站正版角色能力。
* 每个提供方可以配置名称、Issuer、Client ID、Client Secret、scope、图标和显示顺序，并分别控制：
  提供方整体状态、是否允许登录、是否允许已登录用户绑定或重新连接。
* 保存提供方时会验证 Issuer 的 Discovery 信息；外部端点必须使用安全的 HTTPS 地址。
* 回调地址由 `SERVER_API_URL` 统一生成，并在提供方创建和编辑页展示，不再要求管理员维护 Microsoft
  专用回调配置。
* OIDC 管理页新增“本站 OIDC 信息”卡片，展示本站 Discovery、Issuer、Authorization、Token、
  UserInfo 与 JWKS 地址，并引导管理员前往第三方应用页面登记或审核客户端。

### 用户身份管理

* 新增独立的身份管理页，按提供方分组展示用户连接的外部账号、头像、显示名称、邮箱、外部账号标识与
  当前授权状态。
* 同一用户可以在同一个提供方下连接多个不同账号，并为每个身份设置只在本站显示的标签。
* 用户可以添加身份、重新连接长期授权已经失效的身份，或删除不再使用的身份。重新连接时会校验仍是
  原来的外部账号，选错账号不会替换身份或意外创建新身份。
* 登录页会展示管理员开放的外部登录方式。已经绑定的身份可以直接登录本站；没有匹配账户时不会按邮箱
  自动关联或自动创建用户，而是进入完整注册流程。
* 外部登录完成后的注册仍要求填写本站用户名、邮箱、密码，并完成当前启用的邮箱验证码和邀请码校验；
  OIDC 返回的邮箱、显示名或 `email_verified` 不会代替本站账户字段。
* 临时访问令牌按需更新，不要求用户频繁登录。长期授权失效后，身份页会显示“需要重新连接”，并保留
  原身份和已有正版角色关系，成功重新连接后即可继续使用。
* 删除外部身份会解除它关联的正版绑定，但不会删除本站角色、皮肤或披风；普通退出本站也不会移除已连接
  身份。

---

## Microsoft 正版角色绑定与同步

* 旧版一次性“导入 Microsoft 角色”改为长期、可管理的正版角色绑定。Microsoft 只是统一 OIDC 身份的
  一种能力适配，不再使用独立登录或独立身份接口。
* 点击“绑定正版”时，用户从已经连接且状态正常的 Microsoft 身份中选择账号；未连接账号时会直接引导
  前往身份管理页。
* 本站按远端正版 UUID 建立角色关系：UUID 已属于当前用户时复用该角色，不存在时创建对应角色，已经
  属于其他用户时明确拒绝，不会覆盖其他人的角色。
* 建立绑定不会自动反复覆盖本站数据。只有用户在角色详情中点击“同步正版数据”时，才会用远端名称、
  皮肤和披风更新本站角色，并在覆盖前给出确认提示。
* 正版角色会在角色卡片和详情中显示标识；身份管理页也会展示每个 Microsoft 身份当前绑定的本站角色，
  包括本站名称与远端名称不一致的情况。
* 正版绑定、外部身份和本站角色保持独立业务边界。角色仍可改名、清除材质或删除；解除正版绑定会保留
  角色和已经同步的材质，删除角色则只清理对应绑定关系。

---

## 本站作为 OpenID Provider

* 本站现有 OAuth 客户端体系已支持标准 OpenID Connect，不需要再创建另一套 OIDC 客户端或审核入口。
* 新增 OpenID Discovery、JWKS、RS256 ID Token 和 UserInfo，并支持标准 scope：
  `openid`、`profile`、`email`、`offline_access`。
* `profile` 和 `email` claim 只有在应用申请并由用户授权对应 scope 时才会提供；用户标识使用面向客户端
  隔离的 pairwise `sub`。
* OIDC scope 与 Element Skin `/v2` 权限完全分开。只需要“使用本站登录”的应用可以申请零项站点 API
  权限；需要读取或修改角色、材质等资源时，再申请对应权限码。
* OIDC Authorization Code 授权只有包含并获批 `offline_access` 时才签发 refresh token。撤销授权后，
  相关 access token、refresh token 和 UserInfo 访问会一起失效。
* 授权确认页会分开解释 OpenID Connect scope 与站点 API 权限；未登录用户完成登录后会返回原授权请求，
  不再丢失 OAuth/OIDC 参数。
* 第三方应用仍需登记精确回调地址并通过应用审核。Authorization Code 流程使用 PKCE S256；原有
  Device Code、Client Credentials、Refresh Token、撤销和 Introspection 能力继续可用。

---

## OAuth 应用与授权管理

* 第三方应用的创建和编辑从弹窗改为完整页面，可在同一处维护基本信息、公开或机密应用类型、可选 OAuth
  回调地址、站点 API 权限和 Webhook endpoints。
* OAuth 回调地址现在按授权方式选填：Authorization Code 必须配置，只有 Device Code 或 Client
  Credentials 的应用可以留空。OAuth 回调与 Webhook 接收地址是两个独立设置。
* 只使用 OIDC 登录的应用可以不申请任何站点 API 权限；应用类型或权限改变时，页面会同步收窄可选权限
  和 Webhook 事件。
* Client Secret 与 Webhook signing secret 均在生成后只展示一次，页面提供明确提示和复制入口。
* 同一用户对同一应用只保留一条有效授权；用户再次批准会更新原授权，而不是产生多条并行 grant。
  撤销授权会立即使关联凭据失效。
* 修正应用查看、修改、删除、审核以及授权查看和撤销之间的权限边界。只拥有部分管理权限的用户可以进入
  对应页面，但只能看到并执行其权限允许的操作。

---

## Webhook

### 应用配置

* OAuth 应用可以配置最多 5 个可独立启停的 Webhook endpoint，并为每个 endpoint 选择不同事件。
* endpoint 只能使用公网 HTTPS 地址，不接受内网、回环或链路本地地址，也不会跟随 HTTP 重定向。
* endpoint 可配置的事件由应用类型和应用申请权限决定；真正投递时还会检查用户授权与当前有效权限。
  权限被移除、应用或 endpoint 被停用、用户撤销授权后，后续投递也会停止。
* 公开应用和机密应用都可以通过用户委托订阅资源事件；需要 `*.read.any` 的管理型事件只允许机密应用以
  Client Credentials 权限订阅。

### 事件目录

当前公开事件包括：

* 账户：`account.created`、`account.updated`、`account.deleted`；
* OAuth 授权：`oauth_grant.created`、`oauth_grant.updated`、`oauth_grant.revoked`；
* 正版白名单：`official_whitelist.added`、`official_whitelist.removed`；
* 权限：`permission.updated`；
* 角色：`profile.created`、`profile.updated`、`profile.deleted`；
* 材质：`texture.created`、`texture.updated`、`texture.deleted`。

`GET /v2/oauth/webhook-events` 会返回事件说明，以及每个事件可使用的用户委托权限和机密应用权限，
客户端无需维护另一份事件目录。

### 投递与验签契约

* Webhook 使用 `POST application/json`，事件只携带事件类型、用户 UUID 和资源 ID 等基础标识。接收方
  验签后应使用自己的 OAuth access token 调用 `/v2` 获取资源当前状态。
* 每个 endpoint 会获得只显示一次的独立 `signing_secret`。签名使用
  `HMAC-SHA256(signing_secret, timestamp + "." + raw_body)`，请求同时携带稳定事件 ID、投递 ID、
  毫秒时间戳和 `v1=` 签名头，对应 `Webhook-Id`、`Webhook-Delivery`、`Webhook-Timestamp` 与
  `Webhook-Signature`。
* 任意 `2xx` 视为成功，单次请求最多等待 10 秒。网络错误、超时或非 `2xx` 会指数退避重试，最多
  12 次且总时长不超过 72 小时。
* Webhook 是至少一次投递，不保证全局顺序，也可能重复。接收方应先验签和持久接收，再按事件 ID 做
  业务幂等，并尽快返回 `2xx`。事件不是资源快照，接收时对应资源也可能已经被再次修改或删除。

### Python SDK 支持

* 新增 `WebhookVerifier`，可验证必需请求头、原始请求体 HMAC、时间戳窗口和事件结构，并提供细分的
  Header、Signature、Timestamp、Payload 与 Replay 异常。
* 新增可插拔 `ReplayGuard` 接口及适合本地开发的 `MemoryReplayGuard`，便于接收方实现重放防护和
  durable inbox。
* 新增 FastAPI 与 Flask Webhook 接收示例，并说明生产环境的验签、快速响应、持久去重与异步处理流程。

---

## 邮箱后缀、注册与邀请码

* 邮件设置新增“账户邮箱后缀策略”，支持关闭、仅允许白名单、拒绝黑名单三种模式；白名单和黑名单均为
  不分页的完整后缀列表。
* 后缀按忽略大小写的字面值匹配，例如 `@example.com` 不会自动匹配 `@sub.example.com`。策略适用于
  新用户注册和修改账户邮箱，不影响已有账户找回密码。
* 注册页和修改邮箱弹窗会读取公开策略：白名单模式提供允许后缀的选择，黑名单模式会直接提示不可用，
  无效邮箱不会继续请求验证码；后端仍执行最终校验。
* 公共设置现在明确提供 `allow_register`、`require_invite`、`email_verify_enabled` 和当前生效的
  `email_suffix_policy`。注册配置加载失败时，前端会阻止提交，避免遗漏站点要求。
* 站点要求邀请码时，注册页会显示必填项并执行必填校验；不要求时不显示输入框。
* 管理员可以创建、查看和删除包含空格、引号、斜杠、反斜杠及其他任意 UTF-8 字符的邀请码。前端会提示
  邀请码按原文校验，保留大小写、空格和符号。
* 管理 API 的 `POST /v2/admin/invites` 使用可选 `code_base64`；省略时仍由服务端生成邀请码。
  `DELETE /v2/admin/invites/{code_base64}` 使用相同的无填充 Base64URL 编码。

---

## `/v2` API 协议

* JSON 字段统一使用 `snake_case`，时间统一使用 Unix 毫秒时间戳。材质资源中的类型字段统一为 `type`，
  筛选和写入参数使用 `texture_type`。
* 单资源查询直接返回资源对象；非分页集合返回 `{ "items": [...] }`；Cursor 分页固定返回
  `items`、`has_next`、`next_cursor` 与 `page_size`。
* 创建资源统一使用 `201`；无额外结果的动作、幂等设置和删除使用 `204` 空响应；不再返回仅用于表示
  HTTP 成功的 `ok`、`success`、`message` 或通用 `data` envelope。
* 普通 `/v2` API 错误统一为稳定的 `error.object`、`error.operation`、`error.reason` 和可选安全参数。
  不再把后端展示文本、底层错误或不稳定的字符串作为客户端判断依据。
* 明确提供但无效的 Cookie 或 Bearer 凭据会返回 `401`，不会降级为访客继续访问公开资源。
* OAuth/OIDC 标准端点继续返回 RFC 错误对象；Yggdrasil 继续返回协议规定的错误格式，调用方不要混用
  三套错误处理逻辑。

---

## Python SDK 与开发者文档

* Python SDK 的站点客户端已迁移至 `/v2`，并适配新的状态码、空响应、分页结构和字段命名。
* `APIError` 现在公开 `status_code`、`object`、`operation`、`reason`、`params` 与原始响应；OAuth 错误
  继续由独立的 `OAuthError` 表达。
* OAuth `TokenSet` 新增 OIDC `id_token`，Authorization Code helper 支持 `nonce`，并新增标准 OIDC
  scope 以及外部身份、身份提供方和正版角色权限常量。
* 邀请码管理 wrapper 会自动完成 Base64URL 编码，业务代码仍可直接传入邀请码原文。
* SDK 快速开始、OAuth 流程、权限模型、错误与 Token、API 客户端及 Webhook 接收文档均已更新，并补充
  代表用户授权和管理员 Client Credentials 的示例。

---

## 管理与交互改进

* 身份提供方和 OAuth 应用使用独立创建、编辑页面，减少长表单在弹窗中的状态丢失和误操作。
* OIDC、OAuth、身份、正版角色和注册页面会按细粒度权限显示入口与操作，不再依赖粗粒度管理员身份。
* 站点设置、邮件设置、Mojang/Fallback 设置和彩蛋设置会标记“有未保存更改”，重新加载和保存操作的
  状态提示更加明确。
* 邀请码表单补充字符和数值输入提示；注册页会明确说明外部身份验证完成后仍需填写的本站信息。

---

**Full Changelog**: https://github.com/water2004/element-skin/compare/v3.0.2...v4.0.0
