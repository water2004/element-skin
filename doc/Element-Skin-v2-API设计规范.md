# Element Skin v2 API 设计规范

> 状态：现行规范
> 适用分支：`dev`
> 更新日期：2026-08-06

## 1. 目标与边界

`/v2` 是 Element Skin 唯一的站点业务 API。此次升级是 breaking change：不注册
`/v1/*`，也不保留 `/me`、`/public`、`/admin`、`/microsoft` 等旧站点路由的别名、
重定向或兼容 handler。

以下协议不属于站点 API，因此保持各自的标准路径和响应格式：

- OAuth 2.1 / OpenID Connect：`/.well-known/*`、`/oauth/*`。
- Yggdrasil / Mojang 兼容协议：`/authserver/*`、`/sessionserver/*` 及既有 Mojang 查询端点。
- 根 Yggdrasil 元数据和公钥：`/`、`/api/publickeys/`。

站点前端、OAuth bearer 客户端和 Python SDK 调用同一组 `/v2` 资源，不维护第二套业务接口。

## 2. 通用约定

### 2.1 认证与 Actor

站点 API 支持两种凭证，进入 service 前都转换为统一的 `PermissionActor`：

```http
Cookie: access_token=<site access token>
Authorization: Bearer <oauth access token>
```

显式提供但无效的凭证返回 `401`，不会降级为 guest。公开接口仍通过 guest actor 检查
`*.public` 权限。前端权限判断只控制展示，最终授权必须在 service 层完成。

OAuth 用户委托权限为以下交集：

```text
用户当前权限 ∩ 应用权限上限 ∩ grant 实际授权权限
```

Client Credentials 使用 `client:{client_id}` 应用主体，不创建用户 grant。

### 2.2 JSON、命名与时间

- JSON 字段使用 `snake_case`。
- 时间字段使用 Unix 毫秒时间戳。
- 请求 JSON 使用 `application/json`；文件使用 `multipart/form-data`。
- 资源中的材质类型字段统一为 `type`；筛选参数和写入参数使用 `texture_type`。
- 不返回仅用于重复 HTTP 成功语义的 `ok`、`success`、`message` 或通用 `data` envelope。

### 2.3 成功响应

| 操作 | 状态与响应 |
| --- | --- |
| 查询单个资源 | `200` 和资源对象 |
| 非分页集合 | `200` 和 `{ "items": [...] }` |
| Cursor 分页集合 | `200` 和统一分页对象 |
| 创建资源 | `201` 和新资源表示或其稳定标识表示 |
| 更新且调用方需要最新资源 | `200` 和更新后的资源 |
| 无额外结果的动作、幂等设置、删除 | `204`，响应体为空 |

Cursor 分页响应固定为：

```json
{
  "items": [],
  "has_next": false,
  "next_cursor": "",
  "page_size": 0
}
```

`next_cursor` 为空字符串表示没有下一页。客户端不得读取数据库内部游标字段。

批量导入的 `success_count`、`failure_count` 是业务统计结果，不是通用成功 envelope。

### 2.4 错误响应

普通站点 API 错误统一为：

```json
{
  "error": {
    "object": "permission",
    "operation": "check",
    "reason": "denied"
  }
}
```

常用状态码为 `400`、`401`、`403`、`404`、`409`、`429` 和 `500`。数据库、密钥、
外部 token 等内部错误不得原样返回。普通错误不携带后端展示文本；前端依据完整三元组本地化。
字段定义、命名规则和错误目录见《V2 错误协议》。OAuth/OIDC 使用 RFC 错误对象，Yggdrasil 使用协议错误对象。

## 3. 外部身份与注册

### 3.1 业务边界

外部身份是 `(provider_id, subject)` 标识的长期登录身份。同一用户可以在同一 provider 下
绑定多个 subject；同一个 `(provider_id, subject)` 只能属于一个本站用户。

Microsoft 使用普通 OIDC provider 和普通外部身份链路。`adapter=microsoft` 只声明该身份可被
本站正版角色功能消费，不创建 Microsoft 专用登录接口。

OIDC claim 只保存为外部身份资料，不替代本站账户字段，也不通过 email 自动匹配本站账户。

### 3.2 管理员配置 provider

管理员通过 `/v2/admin/identity-providers` 管理 provider。服务端保存以下结构化字段：

- `name`、`issuer_url`、`client_id`、加密后的 `client_secret`；
- Discovery 得到的 `authorization_endpoint`、`token_endpoint`、`userinfo_endpoint`、`jwks_uri`；
- `scopes`、`adapter`、`icon_url`、`display_order`；
- `enabled`、`login_enabled`、`link_enabled`。

保存时必须读取并校验 `/.well-known/openid-configuration`，要求 discovery issuer 与配置值完全
一致，并拒绝不安全的非 HTTPS 外部端点。更新时不提交 `client_secret` 表示保留原密钥。

`login_enabled` 只控制已有外部身份的登录；未匹配身份的登录返回 `identity.login.not_linked`
并引导用户先普通登录再绑定，不提供接续注册。`link_enabled` 只控制已登录用户添加或重新连接身份。

OIDC callback 不由管理员配置。服务端按 `SERVER_API_URL`（或对应配置文件中的 `server.api_url`）
派生并在公开、管理员 provider 列表响应的 `redirect_uri` 字段中返回：

```text
{SERVER_API_URL}/v2/auth/oidc/callback
```

管理员页面必须把该地址显示为“请在身份提供方添加 Redirect URI”，不能继续保存旧版
`microsoft_redirect_uri` 或要求管理员手工拼接路径。

Microsoft adapter 必须请求：

```text
openid profile email XboxLive.signin offline_access
```

面向 Xbox/Minecraft 个人账户时，Microsoft provider 使用 consumer tenant 的规范 Issuer：

```text
https://login.microsoftonline.com/9188040d-6c67-4c5b-b112-36a304b66dad/v2.0
```

Entra 应用必须允许个人 Microsoft 账户；推荐的 `signInAudience` 为
`AzureADandPersonalMicrosoftAccount`。

### 3.3 授权入口

```http
POST /v2/identity-authorizations
Content-Type: application/json

{
  "provider_id": "provider-id",
  "intent": "link",
  "identity_id": "optional-existing-identity-id"
}
```

`intent` 仅允许：

- `login`：公开登录已绑定的外部身份；
- `link`：为当前本站用户添加或重新授权身份，需要 `external_identity.create.owned`。

`identity_id` 只允许用于 `link`。省略时表示在该 provider 下添加身份；传入时表示重新连接当前用户
已有的指定身份。服务端把目标 identity 和 subject 写入一次性 state，并可用已有 email 发送
`login_hint`；callback 得到的 subject 必须与目标身份完全一致，否则返回 `409`，不得创建另一个身份。

响应为 `201`：

```json
{
  "authorization_url": "https://issuer.example/authorize?...",
  "expires_in": 600
}
```

服务端生成一次性的 state、nonce 和 PKCE S256 verifier。通用 OIDC 授权只发送完成协议所需的
标准最小参数，不假定上游支持可选的 `prompt` 值；Microsoft adapter 的 `link` 授权额外发送
`prompt=select_account`，允许用户明确选择另一个 Microsoft 账号。重新连接时即使用户在上游选错
账号，也不会改变目标身份或意外新增身份。

连接授权被取消，或重新连接时选择了不匹配的外部账号，callback 均以稳定错误码重定向回身份管理页。
前端说明原有身份未改变，不向用户展示普通 API JSON 错误页。

统一 callback 为：

```text
GET /v2/auth/oidc/callback
```

callback 会校验 state、nonce、PKCE、issuer、签名、audience 和 subject，并一次性消费 state。

### 3.4 登录时没有已绑定的本站账户

若 `(provider_id, subject)` 已绑定，callback 签发本站 cookie 并跳转仪表盘（或原 `return_to`）。
若未绑定，callback 返回稳定错误 `403 identity.login.not_linked`，并重定向：

```text
/login?error_object=identity&error_operation=login&error_reason=not_linked&redirect=<return_to>
```

登录页提示该外部身份尚未绑定本站账号，引导用户先使用邮箱密码完成普通登录，再到
「控制台 → 外部身份」通过 `link` intent 显式绑定。OIDC 登录不创建账户、不签发注册票据，
注册接口不接受任何外部身份字段。

### 3.5 外部 token 生命周期

- 外部 access token 是短期缓存，存 Redis，不进入长期业务表。
- 外部 refresh token 使用 `identity.encryption_key` 经 AES-256-GCM 加密后存数据库。
- 不设置定时刷新任务。只有依赖外部能力的显式操作需要 access token 时才按需刷新。
- 并发刷新由单航班合并；provider 轮换 refresh token 时原子保存新值。
- 凭据以 `active` 或 `reauthorization_required` 保存结构化授权状态，同时记录最近刷新成功和
  失败时间。身份列表返回 `authorization_status`、`last_refresh_at` 和
  `last_refresh_error_at`，供所有 adapter 和前端统一消费。
- refresh 返回 `invalid_grant` 或 `invalid_request` 时清除 access token 缓存，将凭据标记为
  `reauthorization_required`，并以 `409 external identity must be reauthorized` 提醒用户通过
  同一 `link` 流程重新登录。成功重新授权更新原身份和凭据并恢复 `active`，不建立备用链路。
- 暂时网络错误或上游 `5xx` 只记录失败时间并返回 `502`，不得误标记为需要重新授权。
- 能力 adapter 发现一个尚未到本地过期时间的 access token 被上游以 `401` 拒绝时，必须调用
  身份服务的强制刷新入口并且只重试一次；adapter 不直接读取、保存或刷新凭据。
- 管理员删除 provider 时，在同一数据库事务中删除该 provider 的所有外部身份、长期凭据和正版
  绑定关系；对应的本站角色保留，并清除这些身份的 Redis access token。
- 用户注销账号或被管理员删除时，删除其所有外部身份、长期凭据和正版绑定，并清除对应的 Redis
  access token。普通退出登录不删除外部身份，否则该身份将无法继续用于登录。

## 4. 正版角色绑定

正版绑定是本站资源，不是 OIDC scope，也不是 Microsoft 登录结果。它依赖一个
`adapter=microsoft` 的现有外部身份。客户端只选择外部身份，不选择本站角色：服务端读取远端
Minecraft profile 后，以规范化的正版 UUID 作为本站角色 UUID。

边界固定如下：

- 外部身份 API 不创建、修改或删除角色；创建正版绑定是独立的显式操作；
- 角色 API 不隐式创建或删除外部身份；
- 若正版 UUID 对应的本站角色已经属于当前用户，创建绑定复用该角色；
- 若正版 UUID 尚不存在，创建绑定为当前用户创建同 UUID 角色，名称冲突时使用稳定候选名；
- 若正版 UUID 已属于其他用户，返回 `409 official profile UUID belongs to another user`，不得修改
  角色、身份或绑定；
- 同一远端 UUID 在全站只能存在一个有效正版绑定，并由数据库唯一约束保证并发一致性；
- 只有显式 `sync` 会更新本站角色的名称、皮肤和披风；
- 删除绑定保留本站角色和已同步材质；
- 角色仍可独立改名、清除材质或删除；删除角色时数据库级联删除绑定。

创建请求固定为：

```http
POST /v2/users/me/official-profile-bindings
Content-Type: application/json

{
  "identity_id": "microsoft-external-identity-id"
}
```

请求不接受 `profile_id`。角色卡片只显示正版状态；同步和解除绑定操作放在角色详情中。

绑定响应包含 `identity_id`、`profile_id`、远端 UUID/名称/材质元数据、同步时间，以及只读的
`identity` 和 `profile` 摘要。摘要便于界面显示，不改变两个资源各自的写入边界。

同步在数据库事务中写入角色和材质库。下载、图片校验或事务失败时必须清理新文件，且不得留下
部分角色、材质或绑定状态。

## 5. 本站 OAuth 2.1 / OIDC Provider

本站同时是 OAuth Authorization Server 和 OpenID Provider。管理员审核的 OAuth app 就是本站
OIDC client 注册记录，不另建第二套 OIDC client 表或管理入口。

Discovery：

```text
GET /.well-known/oauth-authorization-server
GET /.well-known/openid-configuration
GET /.well-known/oauth-protected-resource
GET /oauth/jwks
```

Discovery 中的 `issuer`、token、userinfo 和 JWKS 端点使用 `server.api_url`；
`authorization_endpoint` 是浏览器交互入口，必须使用 `server.site_url + /oauth/authorize`，不得指向只返回
JSON 的后端路由。未登录用户转到登录页时必须保留完整授权查询参数，登录成功后回到原授权请求。

协议端点：

```text
GET|POST /oauth/authorize
POST     /oauth/device/code
GET|POST /oauth/device
POST     /oauth/token
POST     /oauth/revoke
POST     /oauth/introspect
GET      /oauth/userinfo
```

支持 Authorization Code + PKCE S256、Device Code、Refresh Token 和 Client Credentials。
OIDC 标准 scope 为 `openid`、`profile`、`email`、`offline_access`；请求任意 OIDC scope 时必须包含
`openid`。站点 `/v2` 权限码与 OIDC scope 是两个独立集合，可组合申请。纯 OIDC 登录 client 可以
不配置任何站点权限，因此其 access token 不能调用受保护的 `/v2` API。

ID token 使用本站独立 RSA 密钥签名，包含 pairwise `sub`。`profile` 和 `email` claim 仅在对应
scope 获批时出现。只有 OIDC 授权包含 `offline_access` 时 token endpoint 才签发 refresh token；
纯 OAuth 授权继续按 OAuth token 生命周期签发 refresh token。grant 撤销后关联 access token、
refresh token 和 userinfo 立即失效。

## 6. 站点 API 路由

表中“响应”使用第 2 节约定；具体授权以权限目录和 service 检查为准。

### 6.1 发现、会话和账户

| 方法 | 路径 | 响应 |
| --- | --- | --- |
| GET | `/v2/capabilities` | 能力对象 |
| GET | `/v2/permissions/catalog` | 权限目录 |
| POST | `/v2/auth/login` | `200`，`user_id`、`permissions`，并设置 cookie |
| POST | `/v2/auth/logout` | `204` |
| POST | `/v2/auth/register` | `201`，`id` |
| POST | `/v2/auth/verification-code` | `200`，`ttl` |
| POST | `/v2/auth/password/reset` | `204` |
| POST | `/v2/auth/session/refresh` | `200`，`permissions`，并轮换 cookie |
| GET | `/v2/users/me` | 当前用户 |
| PATCH | `/v2/users/me` | `204` |
| DELETE | `/v2/users/me` | `204` |
| POST | `/v2/users/me/password` | `204` |
| POST | `/v2/users/me/email/verification-code` | `200`，`ttl` |
| PUT | `/v2/users/me/email` | `204` |

### 6.2 身份和正版绑定

| 方法 | 路径 | 响应 |
| --- | --- | --- |
| GET | `/v2/auth/identity-providers` | 公开 provider 的 `items` 和统一 `redirect_uri` |
| POST | `/v2/identity-authorizations` | `201`，授权 URL |
| GET | `/v2/auth/oidc/callback` | `303` 到本站前端 |
| GET | `/v2/users/me/identities` | 身份 `items` |
| PATCH | `/v2/users/me/identities/{identity_id}` | 更新后的身份 |
| DELETE | `/v2/users/me/identities/{identity_id}` | `204` |
| GET | `/v2/users/me/official-profile-bindings` | 绑定 `items` |
| POST | `/v2/users/me/official-profile-bindings` | `201`，新绑定 |
| POST | `/v2/users/me/official-profile-bindings/{binding_id}/sync` | 同步后的绑定 |
| DELETE | `/v2/users/me/official-profile-bindings/{binding_id}` | `204` |

### 6.3 角色和材质

| 方法 | 路径 | 响应 |
| --- | --- | --- |
| GET | `/v2/users/me/profiles` | Cursor 分页 |
| POST | `/v2/users/me/profiles` | `201`，角色标识表示 |
| PATCH | `/v2/users/me/profiles/{profile_id}` | `204` |
| DELETE | `/v2/users/me/profiles/{profile_id}` | `204` |
| DELETE | `/v2/users/me/profiles/{profile_id}/skin` | `204` |
| DELETE | `/v2/users/me/profiles/{profile_id}/cape` | `204` |
| GET | `/v2/users/me/textures` | Cursor 分页 |
| POST | `/v2/users/me/textures` | `201`，`hash`、`type` |
| POST | `/v2/users/me/textures/upload-and-apply` | `201`，`hash`、`type` |
| GET | `/v2/users/me/textures/{hash}/{texture_type}` | 材质资源 |
| PATCH | `/v2/users/me/textures/{hash}/{texture_type}` | 更新后的材质 |
| DELETE | `/v2/users/me/textures/{hash}/{texture_type}` | `204` |
| POST | `/v2/users/me/textures/{hash}/wardrobe` | `204` |
| POST | `/v2/users/me/textures/{hash}/apply` | `204` |

### 6.4 公开、通知和导入

| 方法 | 路径 | 响应 |
| --- | --- | --- |
| GET | `/v2/public/settings` | 公开设置 |
| GET | `/v2/public/homepage-media` | 媒体 `items` |
| GET | `/v2/public/skin-library` | Cursor 分页 |
| GET | `/v2/public/fallback-status` | fallback 状态 |
| GET | `/v2/minecraft/profiles/by-name/{name}` | 公开角色 |
| GET | `/v2/minecraft/profiles/{id}` | 公开角色 |
| GET | `/v2/minecraft/profiles/{id}/textures-property` | textures property |
| POST | `/v2/minecraft/profiles/by-names` | 角色 `items` |
| POST | `/v2/minecraft/session/has-joined` | `joined` 和 `profile` |
| GET | `/v2/notifications` | Cursor 分页 |
| GET | `/v2/notifications/{id}` | 通知资源 |
| POST | `/v2/notifications/{id}/read` | `204` |
| POST | `/v2/notifications/{id}/dismiss` | `204` |
| POST | `/v2/imports/remote-ygg/profiles/preview` | 远端角色 `items` |
| POST | `/v2/imports/remote-ygg/profiles/import` | `201`，角色标识表示 |
| POST | `/v2/imports/remote-ygg/profiles/import-batch` | 批量导入结果 |

`GET /v2/public/settings` 同时公开注册交互必需的 `allow_register`、`require_invite`、
`email_verify_enabled` 和 `email_suffix_policy`。邮箱后缀策略不分页，只返回当前生效名单：

```json
{
  "email_suffix_policy": {
    "mode": "allowlist",
    "suffixes": ["@example.com", "@qq.com"]
  }
}
```

`mode` 只能是 `disabled`、`allowlist` 或 `denylist`。后缀按忽略大小写的字面后缀匹配；
`@example.com` 不匹配 `@sub.example.com`。前端使用公开配置提前校验，后端仍在验证码签发和最终写入时
重复执行同一策略。名单只限制注册和修改账户邮箱，不限制已有账户找回密码。

### 6.5 OAuth app 与 grant 管理

| 方法 | 路径 | 响应 |
| --- | --- | --- |
| GET | `/v2/oauth/apps` | app `items` |
| GET | `/v2/oauth/webhook-events` | 当前 Webhook 事件目录及所需权限 |
| POST | `/v2/oauth/apps` | `201`，新 app；client 与新 endpoint secret 仅在创建响应出现 |
| GET | `/v2/oauth/apps/{client_id}` | app |
| PATCH | `/v2/oauth/apps/{client_id}` | 更新后的 app |
| DELETE | `/v2/oauth/apps/{client_id}` | `204` |
| POST | `/v2/oauth/apps/{client_id}/review-submission` | 更新后的 app |
| POST | `/v2/oauth/apps/{client_id}/secret` | 带新 secret 的 app |
| GET | `/v2/oauth/apps/{client_id}/permissions` | client 权限状态 |
| PUT | `/v2/oauth/apps/{client_id}/permissions/{permission_code}` | `204` |
| DELETE | `/v2/oauth/apps/{client_id}/permissions/{permission_code}` | `204` |
| GET | `/v2/oauth/grants` | grant `items` |
| DELETE | `/v2/oauth/grants/{grant_id}` | `204` |

`redirect_uri` 可为空；只有 Authorization Code 流程要求应用配置并精确匹配回调地址。应用的
`webhook_endpoints` 是最多 5 项的可选完整替换数组，事件选择必须在应用申请权限和应用类型允许的目录内。事件目录分别返回 `delegated_permission` 和 `application_permission`；后者只允许机密应用使用。
Webhook 只异步发送用户 UUID 和资源 ID 等基础信息，接收方通过 `/v2` API 读取当前资源。完整事件、
签名、重试和权限重检规则见《Webhook 设计与开发者契约》。

同一用户对同一应用只允许一条 active grant。用户再次批准同一应用时更新原有逻辑授权；grant 撤销
后关联 access token、refresh token 与授权码按 OAuth 生命周期立即失效。

### 6.6 管理 API

```text
/v2/admin/users/*
/v2/admin/oauth/apps/*
/v2/admin/oauth/grants/*
/v2/admin/profiles/*
/v2/admin/textures/*
/v2/admin/invites/*
/v2/admin/official-whitelist/*
/v2/admin/homepage-media/*
/v2/admin/notifications/*
/v2/admin/settings/*
/v2/admin/identity-providers/*
```

管理 API 与普通站点 API 使用相同响应规则。列表使用 `items` 或统一 Cursor 分页；创建返回 `201`；
更新在需要最新资源时返回 `200`，否则返回 `204`；删除返回 `204`。

邀请码管理的传输格式固定如下：

```text
GET    /v2/admin/invites
POST   /v2/admin/invites
DELETE /v2/admin/invites/{code_base64}
```

手动创建时，客户端将邀请码原文按 UTF-8 编码，再使用无填充 Base64URL 放入 `code_base64`：

```json
{
  "code_base64": "5qyi6L-OLyJc",
  "total_uses": 1,
  "note": "示例"
}
```

上例解码后是 `欢迎/"\`。省略 `code_base64` 表示由服务端生成邀请码；空值、非法 Base64URL、
非法 UTF-8 和包含 U+0000 的文本均返回 `invite_code.decode.invalid`，不提供旧 `code` 明文字段旁路。
删除路径使用相同编码。响应和注册接口中的 `invite` 始终使用解码后的邀请码原文，因此空格、大小写、
引号和斜杠均保持不变。

邮箱设置页通过 `GET /v2/admin/settings/email-suffix-policy` 一次读取模式、白名单和黑名单完整数组，
通过 `PUT /v2/admin/settings/email-suffix-policy` 原子替换完整策略。该资源使用
`site_settings.read.any` 和 `site_settings.update.any` 权限，不提供分页或单条规则旁路。

## 7. 权限

身份和正版绑定新增权限如下：

```text
identity_provider.read.public
identity_provider.read.any
identity_provider.create.any
identity_provider.update.any
identity_provider.delete.any

external_identity.read.owned
external_identity.create.owned
external_identity.update.owned
external_identity.delete.owned

official_profile.read.owned
official_profile.create.owned
official_profile.refresh.owned
official_profile.delete.owned
```

Microsoft adapter 不增加隐式权限。用户能否管理外部身份、创建绑定、读取绑定、同步或解除绑定，
分别由上述权限决定。超级管理员同样通过完整权限集合授权，业务代码不得特判身份名称。

## 8. 数据模型与删除约束

核心表为：

- `identity_providers`：管理员配置与 discovery 快照；
- `external_identities`：本站用户与 provider subject 的长期关联；
- `external_identity_credentials`：加密 refresh token 和已授予 scope；
- `official_profile_bindings`：外部身份与本站角色的正版关联，`remote_uuid` 全局唯一；
- `email_suffix_policy`、`email_suffix_rules`：邮箱后缀模式以及分离保存的白名单、黑名单；
- OAuth client、grant、authorization code、refresh token、pairwise subject 表：本站 provider 状态。
- Webhook endpoint、事件 outbox 和投递表：第三方事件订阅与异步投递状态。

业务字段均结构化存储。原始 OIDC JSON 不作为业务读取来源。provider 被身份引用时不能删除；身份被
正版绑定引用时不能删除；删除用户或角色按 schema 中明确的级联约束清理关联。

从旧版一次性 Microsoft 导入升级时，启动迁移读取 `microsoft_client_id`、
`microsoft_client_secret` 和 `microsoft_redirect_uri`。完整的 client 凭据会被转换为仅启用绑定能力
的 Microsoft provider，secret 使用当前 `identity.encryption_key` 加密；provider 创建和旧设置删除
必须在同一数据库事务中完成。未配置的空默认值直接清理，残缺配置、discovery 失败、加密失败或
provider 冲突均终止启动并保留旧设置。旧版没有持久化用户 refresh token，因此不伪造外部身份或
正版绑定；已导入的本站角色与材质保持不变，用户需要重新授权后自行建立绑定。

## 9. 测试要求

至少覆盖：

- OIDC discovery、issuer、nonce、PKCE、签名和回调 state 的成功及失败路径；
- 同 provider 多身份、重复 subject、跨用户冲突、重新授权和并发刷新；
- OIDC 未匹配账户时精确返回 `identity.login.not_linked`，不创建用户、身份或票据，state 保持一次性；
- 外部 token 加密、Redis access 缓存、refresh 轮换和并发单航班；
- refresh 被拒绝后的结构化状态、重新授权恢复，以及上游 `401` 强制刷新单次重试；
- 指定 `identity_id` 的重新连接必须拒绝错误 subject，并验证失败后身份、凭据和缓存均不变；
- 连接授权取消和目标 subject 不匹配时，callback 必须以精确错误码返回身份管理页；
- 正版绑定按远端 UUID 创建或复用本站角色、跨用户冲突、并发唯一性、显式同步、图片失败清理和
  数据库事务；
- OIDC-only client 的零站点权限、pairwise subject、userinfo、带或不带 `offline_access` 的精确 refresh
  token 行为和 grant 撤销；
- `/v2` 精确 method/path/body/status/response，旧 `/v1` 与旧未版本化站点路径返回 `404`；
- 前端权限入口、API wrapper 精确请求、TypeScript 构建和关键交互。
