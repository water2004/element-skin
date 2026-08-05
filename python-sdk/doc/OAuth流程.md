# OAuth 流程

Element Skin 使用 OAuth 2.1 风格的授权流程。SDK 明确区分“用户委托流程”和
“应用自身流程”，因为两者允许请求的权限范围不同。

## Authorization Code + PKCE

当用户可以通过浏览器授权，且应用可以接收重定向回调时，使用该流程。

```python
from element_skin_sdk import OAuthClient
from element_skin_sdk.permissions import AccountScopes, ProfileScopes

oauth = OAuthClient(
    base_url="https://skin.example.com",
    client_id="client-id",
    redirect_uri="https://app.example.com/callback",
)

session = oauth.authorization_url(
    [AccountScopes.READ_SELF, ProfileScopes.READ_OWNED],
    state="csrf-state",
)
```

生成的授权地址包含：

- `response_type=code`
- `client_id`
- `redirect_uri`
- `scope`
- `state`
- `code_challenge`
- `code_challenge_method=S256`

回调后交换 token：

```python
tokens = oauth.exchange_code(
    code="returned-code",
    code_verifier=session.code_verifier,
)
```

SDK 会把构造 `OAuthClient` 时配置的 `redirect_uri` 一并发送到 token 端点；它必须与
生成授权请求时的回调地址完全一致。若一次流程临时使用了其他回调地址，也应在
`exchange_code(..., redirect_uri="...")` 中传入同一个地址。

## OpenID Connect

第三方应用只需要“使用皮肤站登录”时，可以申请 OIDC scope 而不申请任何 `/v2` 权限：

```python
from element_skin_sdk.permissions import OIDCScopes

session = oauth.authorization_url(
    [OIDCScopes.OPENID, OIDCScopes.PROFILE, OIDCScopes.EMAIL]
)
```

SDK 会为包含 `openid` 的授权请求生成并保存 `session.nonce`；调用方必须保存整个 session，
并在验证 ID Token claim 时核对 nonce。token 响应中的 ID Token 位于：

```python
tokens.id_token
```

`profile`、`email`、`offline_access` 不能脱离 `openid` 单独请求。OIDC scope 与站点 permission
code 可以组合，但语义独立；纯 OIDC token 不因此获得受保护 `/v2` API 权限。

机密客户端可以在构造函数或单次请求中传入 `client_secret`：

```python
tokens = oauth.exchange_code(
    code="returned-code",
    code_verifier=session.code_verifier,
    client_secret="client-secret",
)
```

## 授权确认辅助方法

自定义授权确认页面或编写集成测试时，可以调用：

```python
request_params = {
    "response_type": "code",
    "client_id": "client-id",
    "redirect_uri": "https://app.example.com/callback",
    "scope": "account.read.self",
    "state": session.state,
    "code_challenge": session.code_challenge,
    "code_challenge_method": "S256",
}

info = oauth.authorization_info(request_params)
decision = oauth.approve_authorization(request_params)
```

这两个接口需要 SDK 持有已登录用户的 access token。

## Device Code Flow

CLI、启动器或不方便接收回调的设备可以使用 Device Code Flow。

```python
from element_skin_sdk.permissions import ProfileScopes

device = oauth.start_device_flow([ProfileScopes.READ_OWNED])

print(device.user_code)
print(device.verification_uri_complete)

tokens = oauth.poll_device_token(device.device_code)
```

`poll_device_token` 会处理 OAuth 的 `authorization_pending` 和 `slow_down`
响应。超过超时时间后会抛出 `TimeoutError`。

如果只需要轮询一次：

```python
tokens = oauth.exchange_device_code(device.device_code)
```

## Client Credentials

Client Credentials 用于应用自身访问，经管理员审核后获得应用主体权限。典型场景是
服务端 Minecraft session 检查。

```python
from element_skin_sdk.permissions import MinecraftScopes

oauth = OAuthClient(
    base_url="https://skin.example.com",
    client_id="server-client",
    client_secret="client-secret",
)

tokens = oauth.client_credentials(
    [MinecraftScopes.SESSION_HASJOINED_SERVER]
)
```

SDK 允许 Client Credentials 请求 `public`、`server` 和经管理员审核授予应用主体的
`any` 范围权限。类似 `account.read.self` 的用户委托权限会被拒绝。

```python
from element_skin_sdk.permissions import InviteScopes

tokens = oauth.client_credentials(
    [InviteScopes.READ_ANY, InviteScopes.CREATE_ANY]
)
```

`any` 权限不是用户授权。应用必须先在站点中申请对应权限，通过管理员审核后，再由
管理员授予应用主体可用的 app-only 权限，否则服务端会拒绝签发或使用 token。

## Refresh Token

```python
tokens = oauth.refresh("refresh-token")
```

请求更窄的用户委托范围：

```python
tokens = oauth.refresh(
    "refresh-token",
    scopes=["account.read.self"],
)
```

## Revoke

```python
oauth.revoke("access-or-refresh-token")
```

## Introspection

```python
result = oauth.introspect("access-token")
```

`introspect` 直接返回服务端响应字典。
