# 错误与 Token

## 异常层级

```text
ElementSkinError
├── ValidationError
│   └── InvalidScope
├── APIError
│   ├── AuthenticationError
│   ├── PermissionDenied
│   └── NotFound
└── OAuthError
```

## 站点 API 错误

普通 `/v2` API 返回结构化错误：

```json
{
  "error": {
    "object": "identity",
    "operation": "link",
    "reason": "conflict",
    "params": {}
  }
}
```

SDK 根据 HTTP 状态选择 `APIError` 子类，并原样暴露机器字段：

```python
from element_skin_sdk.exceptions import PermissionDenied

try:
    api.me()
except PermissionDenied as exc:
    print(exc.status_code)
    print(exc.object)
    print(exc.operation)
    print(exc.reason)
    print(exc.params)
    print(exc.response_body)
```

`str(exc)` 返回便于日志使用的 `object.operation.reason`，不返回面向最终用户的文案。调用方应依据三个独立字段实现本地化与恢复动作，不解析该日志字符串。

## OAuth 错误

OAuth 协议端点使用独立的 RFC 错误对象：

```json
{
  "error": "authorization_pending"
}
```

SDK 将其映射为独立的 `OAuthError`：

```python
from element_skin_sdk.exceptions import OAuthError

try:
    oauth.exchange_device_code("device-code")
except OAuthError as exc:
    print(exc.status_code)
    print(exc.error)
```

## TokenSet

OAuth token 响应用 `TokenSet` 表示：

```python
tokens.access_token
tokens.token_type
tokens.expires_in
tokens.scope
tokens.refresh_token
tokens.permissions
tokens.id_token
```

refresh token 有效期由服务端配置，并在每次刷新时轮换。应用必须持久化每次响应返回的新 `refresh_token`。如果 refresh token 已过期或被撤销，`OAuthClient.refresh()` 会收到 `OAuthError`；应用必须重新执行 Authorization Code 或 Device Code 授权流程。

## TokenStore

`MemoryTokenStore` 适合短生命周期进程或测试：

```python
from element_skin_sdk import MemoryTokenStore, OAuthClient

store = MemoryTokenStore()
oauth = OAuthClient(
    "https://skin.example.com",
    "client-id",
    token_store=store,
)
```

CLI 可使用 `FileTokenStore("tokens.json")` 持久化 token。文件存储会写入结构化 JSON，并尝试将文件权限设置为 `0600`。需要更强保护时，应实现 `TokenStore` 并接入系统钥匙串或其他密钥存储。
