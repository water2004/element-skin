# 错误与 Token

## 异常层级

```text
ElementSkinError
├── ValidationError
│   └── InvalidScope
└── APIError
    ├── AuthenticationError
    ├── PermissionDenied
    ├── NotFound
    └── OAuthError
```

## 站点 API 错误

Element Skin 站点 API 通常返回：

```json
{"detail":"message"}
```

SDK 会映射为 `APIError` 的子类：

```python
from element_skin_sdk.exceptions import PermissionDenied

try:
    api.me()
except PermissionDenied as exc:
    print(exc.status_code)
    print(exc.detail)
    print(exc.response_body)
```

## OAuth 错误

OAuth 协议端点返回：

```json
{
  "error": "authorization_pending",
  "error_description": "authorization pending"
}
```

SDK 会映射为 `OAuthError`：

```python
from element_skin_sdk.exceptions import OAuthError

try:
    oauth.exchange_device_code("device-code")
except OAuthError as exc:
    print(exc.error)
    print(exc.detail)
```

## TokenSet

OAuth token 响应用 `TokenSet` 表示。

```python
tokens.access_token
tokens.token_type
tokens.expires_in
tokens.scope
tokens.refresh_token
tokens.permissions
tokens.id_token
```

refresh token 有效期为服务端配置的授权生命周期，当前站点实现为 30 天并在每次刷新时轮换。应用必须持久化每次响应返回的新 `refresh_token`。如果 refresh token 已过期或被撤销，`OAuthClient.refresh()` 会收到 `OAuthError`；服务端随后会自动撤销失去有效凭证的 grant，应用必须重新执行 Authorization Code 或 Device Code 授权流程。

## MemoryTokenStore

适合短生命周期进程或测试：

```python
from element_skin_sdk import MemoryTokenStore, OAuthClient

store = MemoryTokenStore()
oauth = OAuthClient(
    "https://skin.example.com",
    "client-id",
    token_store=store,
)
```

成功获取 token 后，SDK 默认会保存 token。单次调用可通过 `store=False` 跳过保存。

## FileTokenStore

CLI 需要持久化 token 时可以使用：

```python
from element_skin_sdk import FileTokenStore

store = FileTokenStore("tokens.json")
```

文件存储会写入结构化 JSON，并尝试将文件权限设置为 `0600`。

如果应用需要更强的保护，应实现 `TokenStore`，接入系统钥匙串或其他密钥存储。
