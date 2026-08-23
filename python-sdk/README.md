# Element Skin Python SDK

`element-skin-sdk` 是 Element Skin 的 Python SDK，用于接入 OAuth 2.1 流程、
`/v2` 站点能力 API 和 Webhook 接收。

SDK 主要解决四件事：

- 封装 Authorization Code + PKCE、OpenID Connect、Device Code Flow、Client Credentials、
  刷新、撤销和 introspection 等流程。
- 提供权限常量和本地权限校验，避免第三方应用到处手写权限字符串。
- 提供常用 `/v2` API 的同步 Python 调用入口。
- 验证 Webhook HMAC、时间戳和事件结构，并提供可插拔的重放领取接口。

## 安装

本地开发：

```bash
pip install -e .[test]
```

运行时依赖：

```bash
pip install httpx
```

## 最小示例

```python
from element_skin_sdk import ElementSkinAPI, OAuthClient, UserInfo
from element_skin_sdk.permissions import ProfileScopes

oauth = OAuthClient(
    base_url="https://skin.example.com",
    client_id="app_id",
    redirect_uri="https://app.example.com/callback",
)

session = oauth.authorization_url([ProfileScopes.READ_OWNED])
print(session.authorization_url)

tokens = oauth.exchange_code(
    code="code-from-callback",
    code_verifier=session.code_verifier,
)

api = ElementSkinAPI("https://skin.example.com", token=tokens)
profiles = api.list_profiles()
current_user = UserInfo.from_mapping(api.me())
print(current_user.protected)

# 重设邮箱需要 account.update.self，并由新邮箱接收验证码。
api.request_email_change_code("new@example.com")
api.change_email("new@example.com", "EMAIL123")
```

## 文档

- [SDK 文档入口](doc/README.md)
- [快速开始](doc/快速开始.md)
- [OAuth 流程](doc/OAuth流程.md)
- [权限模型](doc/权限模型.md)
- [API 客户端](doc/API客户端.md)
- [错误与Token](doc/错误与Token.md)
- [Webhook 接收与验签](doc/Webhook接收.md)
- [测试规范](doc/测试规范.md)

## 验证

SDK 要求 100% 行覆盖率和 100% 分支覆盖率：

```bash
python -m coverage run -m pytest
python -m coverage report -m
```
