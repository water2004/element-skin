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
  "detail": "permission denied"
}
```

常用状态码为 `400`、`401`、`403`、`404`、`409`、`429` 和 `500`。数据库、密钥、
外部 token 等内部错误不得原样返回。OAuth/OIDC 使用 RFC 错误对象，Yggdrasil 使用协议错误对象。

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
- `enabled`、`login_enabled`、`link_enabled`、`registration_enabled`。

保存时必须读取并校验 `/.well-known/openid-configuration`，要求 discovery issuer 与配置值完全
一致，并拒绝不安全的非 HTTPS 外部端点。更新时不提交 `client_secret` 表示保留原密钥。

Microsoft adapter 必须请求：

```text
openid profile email XboxLive.signin offline_access
```

### 3.3 授权入口

```http
POST /v2/identity-authorizations
Content-Type: application/json

{
  "provider_id": "provider-id",
  "intent": "login"
}
```

`intent` 仅允许：

- `login`：公开登录或进入完整注册流程；
- `link`：为当前本站用户添加或重新授权身份，需要 `external_identity.create.owned`。

响应为 `201`：

```json
{
  "authorization_url": "https://issuer.example/authorize?...",
  "expires_in": 600
}
```

服务端生成一次性的 state、nonce 和 PKCE S256 verifier。`link` 额外发送标准
`prompt=select_account`，允许同一 provider 选择另一个账号。

统一 callback 为：

```text
GET /v2/auth/oidc/callback
```

callback 会校验 state、nonce、PKCE、issuer、签名、audience 和 subject，并一次性消费 state。

### 3.4 登录后没有本站账户

若 `(provider_id, subject)` 已绑定，callback 签发本站 cookie 并跳转仪表盘。若未绑定且 provider
允许注册，callback 只签发短期、一次性的 `identity_ticket`，然后跳转：

```text
/register?identity_ticket=...&provider_id=...
```

它不会自动创建用户。注册仍提交完整本站表单：

```json
{
  "email": "local@example.com",
  "password": "local-password",
  "username": "LocalName",
  "code": "required-when-enabled",
  "invite": "required-when-enabled",
  "identity_ticket": "one-time-ticket"
}
```

邮箱格式、密码、用户名唯一性、邮箱验证码、邀请注册和并发约束与普通注册完全相同。OIDC email、
display name 或 email_verified 不补全也不覆盖这些字段。失败时 ticket、验证码和邀请码使用次数均不
应被错误消费；成功时在同一注册事务中创建用户、权限主体、外部身份和长期凭据。

### 3.5 外部 token 生命周期

- 外部 access token 是短期缓存，存 Redis，不进入长期业务表。
- 外部 refresh token 使用 `identity.encryption_key` 经 AES-256-GCM 加密后存数据库。
- 不设置定时刷新任务。只有依赖外部能力的显式操作需要 access token 时才按需刷新。
- 并发刷新由单航班合并；provider 轮换 refresh token 时原子保存新值。
- refresh 被拒绝时要求用户通过同一 `link` 流程重新授权，不建立备用链路。

## 4. 正版角色绑定

正版绑定是本站资源，不是 OIDC scope，也不是 Microsoft 登录结果。它依赖一个
`adapter=microsoft` 的现有外部身份，同时关联一个现有本站角色。

边界固定如下：

- 外部身份 API 不创建、修改或删除角色；
- 角色 API 不隐式创建或删除外部身份；
- 创建绑定只读取远端角色元数据并建立关联，不修改本站角色；
- 只有显式 `sync` 会更新本站角色的名称、皮肤和披风；
- 删除绑定保留本站角色和已同步材质；
- 角色仍可独立改名、清除材质或删除；删除角色时数据库级联删除绑定。

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
scope 获批时出现。grant 撤销后关联 access token、refresh token 和 userinfo 立即失效。

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
| GET | `/v2/auth/identity-providers` | 公开 provider 的 `items` |
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
| POST | `/v2/oauth/apps` | `201`，新 app；secret 仅在创建/轮换响应出现 |
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

### 6.6 管理 API

```text
/v2/admin/users/*
/v2/admin/oauth/apps/*
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
- `official_profile_bindings`：外部身份与本站角色的正版关联；
- `email_suffix_policy`、`email_suffix_rules`：邮箱后缀模式以及分离保存的白名单、黑名单；
- OAuth client、grant、authorization code、refresh token、pairwise subject 表：本站 provider 状态。

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
- OIDC 未匹配账户时完整注册字段、验证码、邀请码、ticket 重放与失败回滚；
- 外部 token 加密、Redis access 缓存、refresh 轮换和并发单航班；
- 正版绑定与角色 API 解耦、显式同步、图片失败清理和数据库事务；
- OIDC-only client 的零站点权限、pairwise subject、userinfo、refresh 和 grant 撤销；
- `/v2` 精确 method/path/body/status/response，旧 `/v1` 与旧未版本化站点路径返回 `404`；
- 前端权限入口、API wrapper 精确请求、TypeScript 构建和关键交互。
