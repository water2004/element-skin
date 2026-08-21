# OAuth 2.1 标准与扩展参考

> 参考快照日期：2026-08-06
>
> 本文档用于 Element Skin 的 OAuth 能力设计与实现参考。它不复制 RFC/Internet-Draft 全文，而是收录官方规范入口、规范定位、实现要点、安全要求和本项目取舍。实现时以官方 IETF/RFC 文档为最终依据。

## 1. OAuth 2.1 当前状态

OAuth 2.1 目前仍是 IETF Internet-Draft，不是正式 RFC。截至本文档编写时，最新公开版本是 `draft-ietf-oauth-v2-1-15`，日期为 2026-03-02。

官方入口：

- IETF Datatracker: https://datatracker.ietf.org/doc/draft-ietf-oauth-v2-1/
- 当前 HTML 草案: https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1
- OAuth.net 摘要: https://oauth.net/2.1/

OAuth 2.1 的定位是整理 OAuth 2.0 体系，把长期安全最佳实践合并进核心规范，并移除不再推荐的授权方式。

## 2. OAuth 2.1 与 OAuth 2.0 的关键差异

OAuth 2.1 保留的核心能力：

- Authorization Code Grant
- PKCE
- Refresh Token
- Client Credentials Grant
- Bearer Token
- Token Endpoint
- Authorization Endpoint
- Resource Owner 与 Client 的委托授权模型

OAuth 2.1 删除或不再定义的能力：

- Implicit Grant
- Resource Owner Password Credentials Grant
- Bearer Token in URI Query

OAuth 2.1 强化的要求：

- Authorization Code Flow 必须使用 PKCE。
- Redirect URI 必须精确匹配。
- Public Client 的 Refresh Token 必须轮换或被发送方约束。
- Access Token 是授权凭证，不是登录身份凭证。
- 浏览器、移动端、桌面端、CLI 等客户端都应避免直接接触用户密码。
- 客户端类型、公私客户端能力、回调地址和 token 生命周期必须被明确建模。

## 3. 本项目当前基线

Element Skin 已按 OAuth 2.1 风格实现 Authorization Code + PKCE、Device Authorization、
Refresh Token Rotation、Token Revocation、Token Introspection、Client Credentials、服务发现、
用户授权管理、应用权限上限，以及 access token 到细粒度权限 Actor 的转换。本站同时在同一套
client、grant 和 token 链路上提供 OpenID Connect。

当前 access token 使用 opaque token 和 Redis 短期存储；refresh token、grant 和 client 等长期
授权状态进入 PostgreSQL。尚未承诺的扩展包括 Dynamic Client Registration、JWT Access Token
Profile、DPoP、PAR 和 RAR，新增前必须独立设计和评审。

明确不实现：

- Implicit Grant
- Password Grant
- 通过 URL query 传递 bearer access token

## 4. 核心规范清单

| 规范 | 状态 | 官方地址 | 对本项目的意义 |
| --- | --- | --- | --- |
| OAuth 2.1 Authorization Framework | Internet-Draft | https://datatracker.ietf.org/doc/draft-ietf-oauth-v2-1/ | 新实现的目标基线 |
| OAuth 2.0 Authorization Framework, RFC 6749 | RFC | https://www.rfc-editor.org/rfc/rfc6749 | OAuth 2.1 的历史基础；仅作为背景参考 |
| OAuth 2.0 Bearer Token Usage, RFC 6750 | RFC | https://www.rfc-editor.org/rfc/rfc6750 | Bearer access token 的资源服务器验证规则 |
| OAuth 2.0 Security Best Current Practice, RFC 9700 | RFC | https://www.rfc-editor.org/rfc/rfc9700 | 必须遵循的安全底线 |
| PKCE, RFC 7636 | RFC | https://www.rfc-editor.org/rfc/rfc7636 | Authorization Code Flow 的必选能力 |

## 5. 推荐扩展清单

### 5.1 Token 管理

| 规范 | 状态 | 官方地址 | 本项目取舍 |
| --- | --- | --- | --- |
| Token Revocation, RFC 7009 | RFC | https://www.rfc-editor.org/rfc/rfc7009 | 已实现 |
| Token Introspection, RFC 7662 | RFC | https://www.rfc-editor.org/rfc/rfc7662 | 已实现 |
| JWT Profile for OAuth 2.0 Access Tokens, RFC 9068 | RFC | https://www.rfc-editor.org/rfc/rfc9068 | 未实现；当前使用 opaque token + Redis |
| Token Exchange, RFC 8693 | RFC | https://www.rfc-editor.org/rfc/rfc8693 | 暂不实现；未来服务间委托可评估 |

实现建议：

- Access Token 当前采用 opaque token。
- 服务端保存 token hash，不保存明文 token。
- Token 解析后生成现有权限 Actor。
- Access Token 生命周期应短。
- Refresh Token 必须轮换，旧 token 使用后立即失效。
- Refresh Token 复用应触发授权链路吊销或风险标记。

### 5.2 客户端与服务发现

| 规范 | 状态 | 官方地址 | 本项目取舍 |
| --- | --- | --- | --- |
| Authorization Server Metadata, RFC 8414 | RFC | https://www.rfc-editor.org/rfc/rfc8414 | 已实现 |
| Dynamic Client Registration, RFC 7591 | RFC | https://www.rfc-editor.org/rfc/rfc7591 | 未实现 |
| Dynamic Client Registration Management, RFC 7592 | RFC | https://www.rfc-editor.org/rfc/rfc7592 | 未实现 |
| Protected Resource Metadata, RFC 9728 | RFC | https://www.rfc-editor.org/rfc/rfc9728 | 已实现 |

实现建议：

- 当前由站内开发者控制台注册应用，不开放动态注册。
- 元数据端点应公开授权端点、token 端点、revocation 端点、支持的 grant type、支持的 code challenge method。
- 客户端应区分 confidential client 与 public client。

### 5.3 授权流程扩展

| 规范 | 状态 | 官方地址 | 本项目取舍 |
| --- | --- | --- | --- |
| Device Authorization Grant, RFC 8628 | RFC | https://www.rfc-editor.org/rfc/rfc8628 | 已实现，供启动器、CLI 和服务器插件使用 |
| Resource Indicators, RFC 8707 | RFC | https://www.rfc-editor.org/rfc/rfc8707 | 未实现；当前站点资源服务器单一 |
| Rich Authorization Requests, RFC 9396 | RFC | https://www.rfc-editor.org/rfc/rfc9396 | 未实现；适合复杂授权对象 |

Device Authorization Grant 实现要点：

- 设备端请求 `device_code`、`user_code`、`verification_uri`、`expires_in`、`interval`。
- 用户在浏览器打开验证页面并输入 `user_code`。
- 设备端按 `interval` 轮询 token endpoint。
- 轮询过快必须返回 `slow_down` 或等价错误。
- `device_code` 和 `user_code` 必须有过期时间。

### 5.4 请求保护

| 规范 | 状态 | 官方地址 | 本项目取舍 |
| --- | --- | --- | --- |
| JWT-Secured Authorization Request, RFC 9101 | RFC | https://www.rfc-editor.org/rfc/rfc9101 | 未实现 |
| Pushed Authorization Requests, RFC 9126 | RFC | https://www.rfc-editor.org/rfc/rfc9126 | 未实现，后续优先级高于 JAR |
| Authorization Server Issuer Identification, RFC 9207 | RFC | https://www.rfc-editor.org/rfc/rfc9207 | 若未来多 issuer 或联合登录复杂化，则采用 |

实现建议：

- 当前使用普通 authorization request，并严格校验 `client_id`、`redirect_uri`、`response_type`、`scope`、`state`、`code_challenge`、`code_challenge_method`。
- 未来如果授权参数变复杂，先实现 PAR，再考虑 JAR。

### 5.5 发送方约束与高安全场景

| 规范 | 状态 | 官方地址 | 本项目取舍 |
| --- | --- | --- | --- |
| Mutual-TLS Client Authentication and Certificate-Bound Access Tokens, RFC 8705 | RFC | https://www.rfc-editor.org/rfc/rfc8705 | 暂不实现；运维成本较高 |
| DPoP, RFC 9449 | RFC | https://www.rfc-editor.org/rfc/rfc9449 | 未实现；适合 public client 的 token replay 防护 |

实现建议：

- 当前使用短生命周期 access token、refresh token rotation、token hash 存储和权限版本缓存。
- 如果开放高权限管理员 OAuth 授权，应优先评估 DPoP 或其他 sender-constrained token 方案。

### 5.6 客户端类型最佳实践

| 规范 | 状态 | 官方地址 | 本项目取舍 |
| --- | --- | --- | --- |
| OAuth 2.0 for Native Apps, RFC 8252 | RFC | https://www.rfc-editor.org/rfc/rfc8252 | 支持桌面启动器、移动端时必须参考 |
| OAuth 2.0 for Browser-Based Apps | Internet-Draft | https://datatracker.ietf.org/doc/draft-ietf-oauth-browser-based-apps/ | 若开放纯前端第三方应用必须参考 |

实现建议：

- 桌面启动器应使用 Authorization Code + PKCE 或 Device Authorization Grant。
- Public client 不能依赖 client secret 保密。
- Browser-based client 不应长期持有高权限 refresh token。

## 6. Element Skin OAuth 权限模型映射

访问 `/v2` 业务资源的 OAuth scope 直接使用权限 catalog 中的 permission code。OIDC 协议 scope
`openid`、`profile`、`email`、`offline_access` 与站点 permission code 分开解析和保存；它们控制
ID Token、UserInfo 和 refresh token 等协议行为，本身不授予任何 `/v2` 业务权限。

令牌最终权限必须按以下规则裁剪：

```text
token_permissions =
  user_effective_permissions
  ∩ client_allowed_permissions
  ∩ grant_approved_permissions
```

其中：

- `user_effective_permissions` 是用户当前有效权限。
- `client_allowed_permissions` 是 OAuth 应用被允许申请的权限上限。
- `grant_approved_permissions` 是用户在授权页实际同意授予的权限。

服务层不应知道请求来自网页登录还是 OAuth。HTTP 层应把请求解析为统一 Actor：

```text
Actor {
  subject_type: user | oauth_user | oauth_client
  user_id
  client_id
  grant_id
  permissions_bitset
  permission_version
}
```

业务服务继续只检查 Actor 权限，不检查 OAuth 细节。

## 7. 应用注册模型

OAuth 应用需要至少包含：

- 应用 ID
- 应用名称
- 应用描述
- 开发者用户 ID
- 应用类型：public 或 confidential
- Redirect URI 列表
- 允许申请的权限集合
- 是否允许申请管理员权限
- 是否允许离线访问
- 是否启用
- 创建时间
- 更新时间
- 最后使用时间

Confidential client 需要：

- client secret hash
- secret 创建时间
- secret 轮换时间
- secret 失效时间

Public client 不应持久依赖 client secret。

## 8. 用户授权模型

用户授权需要至少包含：

- 授权 ID
- 用户 ID
- 应用 ID
- 已授权权限集合
- 是否允许 refresh token
- 授权创建时间
- 授权更新时间
- 最后使用时间
- 授权版本
- 是否已撤销

用户必须能在站内管理页面看到并撤销授权。

撤销授权后：

- 所有关联 refresh token 失效。
- 所有关联 access token 失效或在极短时间内自然过期。
- 对应权限缓存失效。
- grant 记录保留 30 天用于用户可见历史和排错审计；30 天后由系统维护任务删除 grant 及其授权码、refresh token、权限关联记录。

Element Skin 将 grant 的可用生命周期绑定到长期凭证：超过 token 签发保护期后，如果 grant 已不存在未撤销且未过期的 refresh token，也不存在仍在有效期内的 authorization code，系统维护任务会自动撤销 grant。refresh token 过期且未轮换时，应用必须重新发起用户授权，不允许在授权管理中长期留下 active 的无凭证 grant。

## 9. Token 模型

Authorization Code 需要：

- code hash
- client ID
- user ID
- redirect URI
- code challenge
- code challenge method
- approved permissions
- expires_at
- used_at

Access Token 需要：

- token hash
- user ID，可为空
- client ID
- grant ID，可为空
- permissions bitset
- permission version
- expires_at
- revoked_at

Refresh Token 需要：

- token hash
- user ID
- client ID
- grant ID
- family ID
- parent token ID，可为空
- expires_at
- used_at
- revoked_at
- rotated_to token ID，可为空

Device Code 需要：

- device_code hash
- user_code hash
- client ID
- requested permissions
- approved permissions
- expires_at
- interval
- status：pending、approved、denied、expired
- last_poll_at

## 10. Endpoint 参考

当前协议端点：

```text
GET      /.well-known/oauth-authorization-server
GET      /.well-known/openid-configuration
GET      /.well-known/oauth-protected-resource
GET|POST /oauth/authorize
POST     /oauth/device/code
GET|POST /oauth/device
POST     /oauth/token
POST     /oauth/revoke
POST     /oauth/introspect
GET      /oauth/jwks
GET      /oauth/userinfo
```

应用和授权记录复用站点 `/v2` API 管理：

```text
GET|POST          /v2/oauth/apps
GET|PATCH|DELETE  /v2/oauth/apps/{client_id}
GET               /v2/oauth/grants
DELETE            /v2/oauth/grants/{grant_id}
```

`/oauth/par` 不在当前协议中。

## 11. Grant Type 支持矩阵

| Grant Type | 当前状态 | 说明 |
| --- | --- | --- |
| `authorization_code` | 已实现 | 必须配合 PKCE S256 |
| `refresh_token` | 已实现 | 每次使用均轮换 |
| `client_credentials` | 已实现 | 使用管理员审核后的 `client:{client_id}` 应用主体 |
| `urn:ietf:params:oauth:grant-type:device_code` | 已实现 | 面向启动器、CLI 和插件 |
| `password` | 禁止 | OAuth 2.1 不保留 |
| `implicit` | 禁止 | OAuth 2.1 不保留 |

## 12. 安全要求清单

必须满足：

- Authorization Code 必须一次性使用。
- Authorization Code 必须短有效期。
- PKCE 只接受 `S256`。
- Redirect URI 必须精确匹配预注册 URI。
- `state` 必须原样返回。
- Token endpoint 不接受 query token。
- Refresh Token 必须轮换。
- Token 存储只保存 hash。
- 撤销授权必须使 refresh token 失效。
- OAuth 应用禁用后必须拒绝新授权和 token 刷新。
- 权限变更必须使相关 token 权限缓存失效。
- 管理员权限必须显式允许应用申请，并在授权页明确展示。
- 用户不能授权自己没有的权限。
- 应用不能申请超出上限的权限。

建议满足：

- Access Token 短生命周期，建议 5 到 15 分钟。
- Refresh Token 按应用类型设定生命周期。
- 高权限授权要求二次确认。
- 高权限 OAuth 应用操作写入审计日志。
- 管理员可查看和撤销 OAuth 应用授权。
- 设备码轮询需要限速。
- OAuth 授权页需要按权限分类展示，而不是平铺字符串。

## 13. 与现有权限系统的结合

OAuth 不新增业务权限判断分支。所有接口仍通过现有权限 middleware/service 检查 Actor。

需要新增的能力：

- OAuth token 认证 middleware。
- OAuth token 到 Actor 的转换器。
- OAuth 权限缓存。
- OAuth 授权变更时的权限缓存失效。
- 应用权限上限管理。
- 用户授权管理。

权限缓存建议 key：

```text
oauth_actor:{user_id}:{client_id}:{grant_id}:{permission_version}
```

缓存值：

```text
permissions_bitset
expires_at
subject_metadata
```

失效来源：

- 用户角色变更
- 用户权限覆盖变更
- OAuth 应用权限上限变更
- 用户授权权限变更
- 授权撤销
- 应用禁用

### 13.1 Client Credentials 与应用主体

Client Credentials Grant 不代表用户，不得复用用户委托授权表。

应用自身必须成为独立权限主体：

```text
permission_subjects.id = client:{client_id}
permission_subjects.kind = client
```

Client Credentials token 必须单独存储：

```text
Redis oauth:access:{token_hash}
```

Client Credentials access token 使用 Redis 短期存储，不写入 PostgreSQL。该 token 不包含 `user_id`、不包含 `grant_id`，只能恢复：

```text
subject_id = client:{client_id}
session_kind = client_credentials
entrypoint = api
client_id = client_id
```

Client Credentials 的最终权限：

```text
client subject effective permissions
∩ session_permission_bitset(client_credentials, api)
∩ delegated_client_permissions
∩ requested_token_scope
∩ app status active
```

`requested_token_scope` 只能缩小应用主体已经拥有的权限，不能申请新权限。

管理员审核通过后，直接给 `client:{client_id}` 逐项授予 permission。Client Credentials 场景不使用用户授权页，也不使用 `delegated_permission_grants`；`delegated_client_permissions` 是开发者提交并经审核的应用权限上限，必须与客户端主体有效权限取交集。

Client Credentials 首批站点能力包括 Minecraft 服务端查询和经管理员审核下放的站点管理能力：

```http
POST /v2/minecraft/session/has-joined
GET  /v2/admin/invites
POST /v2/admin/invites
DELETE /v2/admin/invites/{id}
```

对应权限示例：

```text
minecraft_session.hasjoined.server
invite.read.any
invite.create.any
invite.delete.any
```

`server` scope 表示受审核服务端或外置插件客户端能力，不等于 `any`，也不等于 Yggdrasil `bound_profile`。`any` scope 只表示资源范围是全站资源；是否可由 Client Credentials 使用，仍取决于管理员是否已将该 permission 授予 `client:{client_id}`。

## 14. 与管理员能力下放的关系

管理员能力可以通过 OAuth 下放，但必须满足三层限制：

```text
管理员本人拥有权限
∩ 应用被管理员允许申请权限
∩ 管理员在授权页同意授予权限
```

受保护权限不应默认允许 OAuth 委托。建议单独引入应用级控制项：

- 应用是否可申请普通用户权限
- 应用是否可申请普通管理员权限
- 应用是否可申请受保护权限

受保护权限申请、授权、使用都应写入审计日志。

## 15. OpenID Connect 实现边界

本站已在同一 OAuth client、grant 和 token 链路上实现 OpenID Connect，不建立第二套客户端或授权记录。

当前实现包括：

- `openid`、`profile`、`email`、`offline_access` scope；
- RS256 ID Token 与独立 OIDC 签名密钥；
- pairwise `sub`；
- UserInfo、JWKS 与 OIDC Discovery；
- Authorization Code 请求中的 nonce 会绑定到授权码并写入 ID Token；
- `authorization_endpoint` 指向站点前端授权页，登录前后完整保留授权请求；
- OIDC Authorization Code 只有请求并获批 `offline_access` 时才签发 refresh token；
- grant 撤销后 refresh token、access token 和 UserInfo 立即失效。已签发的自包含 ID Token 不做
  在线撤销，依靠短有效期自然过期。

OIDC scope 与 Element Skin `/v2` permission code 必须分开。纯 OIDC client 可以拥有零项站点权限；它可以取得 ID Token/UserInfo，但 access token 不能调用受保护站点 API。OAuth access token 也不得当作 ID Token 使用。

## 16. 当前实现与后续扩展

当前已实现 Authorization Code + PKCE S256、Device Code、Refresh Token Rotation、Revocation、Introspection、Client Credentials、OIDC、OAuth Actor 权限裁剪、应用/授权管理页和短期 access token 存储。

以下能力不在当前协议承诺内，新增前必须独立设计和评审：

1. Dynamic Client Registration。
2. Pushed Authorization Requests（PAR）。
3. DPoP 或 mTLS sender-constrained token。
4. JWT Access Token Profile。

## 17. 实现验收标准

功能验收：

- 第三方应用可以通过 Authorization Code + PKCE 获取 token。
- 用户可以精确选择授权权限。
- 应用不能申请超出上限的权限。
- 用户不能授予自己没有的权限。
- OAuth token 可以调用对应权限的 API。
- OAuth token 不能调用未授权 API。
- 用户可撤销授权。
- 撤销后 refresh token 立即失效。
- 应用禁用后 token 刷新失败。

安全验收：

- code 重放失败。
- redirect URI 不精确匹配失败。
- 缺少 PKCE 失败。
- 错误 PKCE verifier 失败。
- refresh token 重放触发失效策略。
- token 明文不落库。
- 高权限授权有明显提示。

性能验收：

- OAuth token 鉴权路径不应每次重建完整权限图。
- 有效权限缓存必须随权限版本失效。
- loadtest 必须覆盖普通用户 token、管理员 token、撤销后 token、权限变更后 token。

## 18. 主要官方资料入口

- OAuth 2.1 Draft: https://datatracker.ietf.org/doc/draft-ietf-oauth-v2-1/
- OAuth 2.1 Overview: https://oauth.net/2.1/
- OAuth 2.0 Specs Map: https://oauth.net/2/
- RFC 9700 Security BCP: https://www.rfc-editor.org/rfc/rfc9700
- RFC 7636 PKCE: https://www.rfc-editor.org/rfc/rfc7636
- RFC 7009 Revocation: https://www.rfc-editor.org/rfc/rfc7009
- RFC 7662 Introspection: https://www.rfc-editor.org/rfc/rfc7662
- RFC 8414 Authorization Server Metadata: https://www.rfc-editor.org/rfc/rfc8414
- RFC 8628 Device Authorization Grant: https://www.rfc-editor.org/rfc/rfc8628
- RFC 9068 JWT Access Token Profile: https://www.rfc-editor.org/rfc/rfc9068
- RFC 9101 JAR: https://www.rfc-editor.org/rfc/rfc9101
- RFC 9126 PAR: https://www.rfc-editor.org/rfc/rfc9126
- RFC 9207 Issuer Identification: https://www.rfc-editor.org/rfc/rfc9207
- RFC 9396 Rich Authorization Requests: https://www.rfc-editor.org/rfc/rfc9396
- RFC 9449 DPoP: https://www.rfc-editor.org/rfc/rfc9449
- RFC 8252 Native Apps: https://www.rfc-editor.org/rfc/rfc8252
- Browser-Based Apps Draft: https://datatracker.ietf.org/doc/draft-ietf-oauth-browser-based-apps/
