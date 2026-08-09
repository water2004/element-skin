# Element Skin Python SDK 文档

本目录面向第三方 Python 应用开发者，说明如何使用 `element-skin-sdk` 接入
Element Skin 的 OAuth 和 `/v2` API。

## 阅读顺序

1. [快速开始](快速开始.md)
2. [OAuth 流程](OAuth流程.md)
3. [权限模型](权限模型.md)
4. [API 客户端](API客户端.md)
5. [错误与 Token](错误与Token.md)
6. [Webhook 接收与验签](Webhook接收.md)
7. [测试规范](测试规范.md)

## 包结构

```text
element_skin_sdk
├── OAuthClient             OAuth 2.1 流程封装
├── ElementSkinAPI          `/v2` API 客户端
├── permissions             权限常量和校验器
├── oauth                   PKCE 与 token 存储
├── webhook                 事件验签、解析与可插拔重放防护
├── models                  token、权限等数据模型
└── exceptions              SDK 异常层级
```

## 已支持的 OAuth 流程

| 流程 | SDK 方法 | 典型应用 |
| --- | --- | --- |
| Authorization Code + PKCE | `authorization_url`、`exchange_code` | Web 应用、桌面应用、可打开浏览器的 CLI |
| Device Code Flow | `start_device_flow`、`poll_device_token` | CLI、启动器、无法方便接收回调的设备 |
| Client Credentials | `client_credentials` | 经管理员审核的应用自身能力 |
| Refresh Token | `refresh` | 长期用户委托访问 |
| Revoke | `revoke` | 退出登录或取消连接 |
| Introspection | `introspect` | token 调试或管理侧校验 |

## 已封装的 API 分组

当前同步客户端覆盖常用用户 API 和 Minecraft 能力 API：

- `GET /v2/users/me`
- `POST /v2/users/me/email/verification-code`
- `PUT /v2/users/me/email`
- `GET/POST/PATCH/DELETE /v2/users/me/profiles`
- `GET/PATCH/DELETE /v2/users/me/textures/{hash}/{texture_type}`
- `POST /v2/users/me/textures/{hash}/wardrobe`
- `POST /v2/users/me/textures/{hash}/apply`
- `GET /v2/minecraft/profiles/by-name/{name}`
- `POST /v2/minecraft/profiles/by-names`
- `POST /v2/minecraft/session/has-joined`

后续增加更多 `/v2` wrapper 不需要改变 OAuth 行为。

## Webhook 接收

`WebhookVerifier` 验证原始请求体的 HMAC、时间戳、必需请求头和事件结构；`ReplayGuard` 为生产方提供原子 durable inbox 接口，`MemoryReplayGuard` 仅用于测试和单进程示例。详见 [Webhook 接收与验签](Webhook接收.md)。
