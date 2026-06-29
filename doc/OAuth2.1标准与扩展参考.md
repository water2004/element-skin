# OAuth 2.1 标准与扩展参考

> 参考快照日期：2026-06-29
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

## 3. 本项目目标基线

Element Skin 的 OAuth 实现应以 OAuth 2.1 风格为目标，而不是复刻 OAuth 2.0 的所有历史模式。

第一阶段应实现：

- Authorization Code + PKCE
- Refresh Token Rotation
- Token Revocation
- OAuth Authorization Server Metadata
- 用户授权管理
- 应用权限上限控制
- Access Token 到现有细粒度权限 Actor 的转换

第一阶段可同时设计、视工作量实现：

- Device Authorization Grant
- Token Introspection

第二阶段再考虑：

- Client Credentials
- Dynamic Client Registration
- JWT Access Token Profile
- DPoP
- PAR
- RAR

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
| Token Revocation, RFC 7009 | RFC | https://www.rfc-editor.org/rfc/rfc7009 | 第一阶段实现 |
| Token Introspection, RFC 7662 | RFC | https://www.rfc-editor.org/rfc/rfc7662 | 第一阶段可实现；若 access token 采用 opaque token，则强烈建议实现 |
| JWT Profile for OAuth 2.0 Access Tokens, RFC 9068 | RFC | https://www.rfc-editor.org/rfc/rfc9068 | 第二阶段评估；初期可优先 opaque token + 服务端缓存 |
| Token Exchange, RFC 8693 | RFC | https://www.rfc-editor.org/rfc/rfc8693 | 暂不实现；未来服务间委托可评估 |

实现建议：

- Access Token 初期建议采用 opaque token。
- 服务端保存 token hash，不保存明文 token。
- Token 解析后生成现有权限 Actor。
- Access Token 生命周期应短。
- Refresh Token 必须轮换，旧 token 使用后立即失效。
- Refresh Token 复用应触发授权链路吊销或风险标记。

### 5.2 客户端与服务发现

| 规范 | 状态 | 官方地址 | 本项目取舍 |
| --- | --- | --- | --- |
| Authorization Server Metadata, RFC 8414 | RFC | https://www.rfc-editor.org/rfc/rfc8414 | 第一阶段实现 |
| Dynamic Client Registration, RFC 7591 | RFC | https://www.rfc-editor.org/rfc/rfc7591 | 第二阶段评估 |
| Dynamic Client Registration Management, RFC 7592 | RFC | https://www.rfc-editor.org/rfc/rfc7592 | 第二阶段评估 |
| Protected Resource Metadata, RFC 9728 | RFC | https://www.rfc-editor.org/rfc/rfc9728 | 第二阶段评估 |

实现建议：

- 第一阶段由站内开发者控制台注册应用，不开放完全动态注册。
- 元数据端点应公开授权端点、token 端点、revocation 端点、支持的 grant type、支持的 code challenge method。
- 客户端应区分 confidential client 与 public client。

### 5.3 授权流程扩展

| 规范 | 状态 | 官方地址 | 本项目取舍 |
| --- | --- | --- | --- |
| Device Authorization Grant, RFC 8628 | RFC | https://www.rfc-editor.org/rfc/rfc8628 | 适合启动器、CLI、服务器插件；建议第一阶段或紧随其后实现 |
| Resource Indicators, RFC 8707 | RFC | https://www.rfc-editor.org/rfc/rfc8707 | 第二阶段评估；当前站点资源服务器单一，可暂缓 |
| Rich Authorization Requests, RFC 9396 | RFC | https://www.rfc-editor.org/rfc/rfc9396 | 第二阶段评估；适合复杂授权对象 |

Device Authorization Grant 实现要点：

- 设备端请求 `device_code`、`user_code`、`verification_uri`、`expires_in`、`interval`。
- 用户在浏览器打开验证页面并输入 `user_code`。
- 设备端按 `interval` 轮询 token endpoint。
- 轮询过快必须返回 `slow_down` 或等价错误。
- `device_code` 和 `user_code` 必须有过期时间。

### 5.4 请求保护

| 规范 | 状态 | 官方地址 | 本项目取舍 |
| --- | --- | --- | --- |
| JWT-Secured Authorization Request, RFC 9101 | RFC | https://www.rfc-editor.org/rfc/rfc9101 | 第二阶段评估 |
| Pushed Authorization Requests, RFC 9126 | RFC | https://www.rfc-editor.org/rfc/rfc9126 | 第二阶段评估，优先级高于 JAR |
| Authorization Server Issuer Identification, RFC 9207 | RFC | https://www.rfc-editor.org/rfc/rfc9207 | 若未来多 issuer 或联合登录复杂化，则采用 |

实现建议：

- 第一阶段可使用普通 authorization request，但必须严格校验 `client_id`、`redirect_uri`、`response_type`、`scope`、`state`、`code_challenge`、`code_challenge_method`。
- 未来如果授权参数变复杂，先实现 PAR，再考虑 JAR。

### 5.5 发送方约束与高安全场景

| 规范 | 状态 | 官方地址 | 本项目取舍 |
| --- | --- | --- | --- |
| Mutual-TLS Client Authentication and Certificate-Bound Access Tokens, RFC 8705 | RFC | https://www.rfc-editor.org/rfc/rfc8705 | 暂不实现；运维成本较高 |
| DPoP, RFC 9449 | RFC | https://www.rfc-editor.org/rfc/rfc9449 | 第二阶段评估；适合 public client 的 token replay 防护 |

实现建议：

- 第一阶段优先使用短生命周期 access token、refresh token rotation、token hash 存储和权限版本缓存。
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

OAuth scope 不应另起一套字符串体系。Element Skin 应直接使用现有权限 catalog 中的 permission code 作为 OAuth scope。

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

第一阶段建议端点：

```text
GET  /.well-known/oauth-authorization-server
GET  /oauth/authorize
POST /oauth/token
POST /oauth/revoke
GET  /oauth/consents
DELETE /oauth/consents/{grant_id}
GET  /developer/oauth/apps
POST /developer/oauth/apps
GET  /developer/oauth/apps/{client_id}
PATCH /developer/oauth/apps/{client_id}
DELETE /developer/oauth/apps/{client_id}
```

Device Flow 端点：

```text
POST /oauth/device/code
GET  /oauth/device
POST /oauth/device/confirm
```

可选端点：

```text
POST /oauth/introspect
POST /oauth/par
```

## 11. Grant Type 支持矩阵

| Grant Type | 第一阶段 | 说明 |
| --- | --- | --- |
| `authorization_code` | 必须 | 必须配合 PKCE |
| `refresh_token` | 必须 | 必须轮换 |
| `client_credentials` | 暂缓 | 管理员/受信应用场景再开放 |
| `urn:ietf:params:oauth:grant-type:device_code` | 建议 | 启动器、CLI、插件体验好 |
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

## 15. 与 OpenID Connect 的边界

OAuth 负责授权，不负责身份登录语义。若第三方应用需要“用皮肤站登录”，应另行设计 OpenID Connect。

OAuth-only 第一阶段可以提供：

- `/me` 或 `/oauth/userinfo` 风格的资源接口
- 使用 `profile.read.self` 权限读取用户资料

但不要把 OAuth access token 当作 ID token 使用。

如果以后支持 OpenID Connect，需要补充：

- `openid` scope
- ID Token
- UserInfo endpoint
- JWKS endpoint
- OIDC Discovery
- nonce 校验

## 16. 本项目推荐实现顺序

第一批：

1. OAuth 数据模型与迁移。
2. 开发者应用管理页。
3. Authorization Code + PKCE。
4. 授权确认页。
5. Token endpoint。
6. Refresh Token Rotation。
7. Revocation endpoint。
8. OAuth token middleware。
9. OAuth Actor 与权限 bitset 裁剪。
10. 用户授权管理页。
11. 审计日志。
12. 测试与 loadtest。

第二批：

1. Device Authorization Grant。
2. Token Introspection。
3. 管理员撤销任意应用授权。
4. OAuth 应用风险提示。
5. 权限缓存优化。

第三批：

1. Client Credentials。
2. Dynamic Client Registration。
3. PAR。
4. DPoP。
5. JWT Access Token Profile。
6. OpenID Connect。

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
