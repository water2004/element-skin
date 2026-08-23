# API 客户端

`ElementSkinAPI` 是常用 Element Skin `/v2` 接口的同步封装。

## 构造客户端

使用 access token：

```python
from element_skin_sdk import ElementSkinAPI

api = ElementSkinAPI(
    "https://skin.example.com",
    access_token="access-token",
)
```

使用 `TokenSet`：

```python
api = ElementSkinAPI("https://skin.example.com", token=tokens)
```

显式传入权限：

```python
api = ElementSkinAPI(
    "https://skin.example.com",
    access_token="access-token",
    permissions=("account.read.self",),
)
```

## 当前用户

```python
me = api.me()
```

`me` 返回服务端原始 JSON。当前响应包含 `protected` 字段，表示当前用户主体是否为受保护主体：

```python
from element_skin_sdk import UserInfo

info = UserInfo.from_mapping(api.me())
print(info.protected)
```

接口：

```text
GET /v2/users/me
```

所需权限：

```text
account.read.self
```

### 重设邮箱

邮箱不能通过普通账号资料更新直接修改。应用必须先向新邮箱发送验证码，再提交新邮箱和验证码：

```python
sent = api.request_email_change_code("new@example.com")
print(sent["ttl"])

api.change_email("new@example.com", "EMAIL123")
```

接口：

```text
POST /v2/users/me/email/verification-code
PUT  /v2/users/me/email
```

两个接口都需要：

```text
account.update.self
```

Authorization Code 或 Device Code 获取的用户委托 token 可以调用这两个接口；应用自身的 Client Credentials token 没有当前用户，不能调用。

## 角色

```python
profiles = api.list_profiles(cursor=None, page_size=20)
created = api.create_profile("Steve", model="default")
updated = api.update_profile("profile-id", name="Alex", model="slim")
api.delete_profile("profile-id")
```

接口：

```text
GET    /v2/users/me/profiles
POST   /v2/users/me/profiles
PATCH  /v2/users/me/profiles/{profile_id}
DELETE /v2/users/me/profiles/{profile_id}
```

## 材质

```python
textures = api.list_textures(texture_type="skin", page_size=20)
texture = api.get_texture("texture-hash", "skin")
updated = api.update_texture("texture-hash", "skin", note="Main skin")
api.delete_texture("texture-hash", "skin")
```

接口：

```text
GET    /v2/users/me/textures
GET    /v2/users/me/textures/{hash}/{texture_type}
PATCH  /v2/users/me/textures/{hash}/{texture_type}
DELETE /v2/users/me/textures/{hash}/{texture_type}
```

## 衣柜操作

```python
api.add_texture_to_wardrobe("texture-hash", texture_type="skin")
api.apply_texture("texture-hash", profile_id="profile-id", texture_type="skin")
```

接口：

```text
POST /v2/users/me/textures/{hash}/wardrobe
POST /v2/users/me/textures/{hash}/apply
```

## Minecraft 能力 API

这些接口是 `/v2/minecraft` 下的站点能力 API，不是 Yggdrasil 协议端点。

```python
profile = api.minecraft_profile("Steve")
profiles = api.minecraft_profiles(["Steve", "Alex"])
joined = api.minecraft_has_joined(
    username="Steve",
    server_id="server-hash",
    ip="127.0.0.1",
)
```

接口：

```text
GET  /v2/minecraft/profiles/by-name/{name}
POST /v2/minecraft/profiles/by-names
POST /v2/minecraft/session/has-joined
```

`minecraft_has_joined` 需要应用自身 token 具备：

```text
minecraft_session.hasjoined.server
```

## 邀请码管理

SDK 接收和返回邀请码原文，并在管理接口边界自动完成 UTF-8、无填充 Base64URL 编码：

```python
page = api.list_invites(page_size=15)
created = api.create_invite('欢迎/"\\', total_uses=None, note="长期邀请码")
api.delete_invite(created["code"])
```

`total_uses=None` 表示不限次数；省略 `code` 时由服务端生成邀请码：

```python
generated = api.create_invite(note="自动生成")
```

对应接口和权限：

| SDK 方法 | 接口 | 权限 |
| --- | --- | --- |
| `list_invites` | `GET /v2/admin/invites` | `invite.read.any` |
| `create_invite` | `POST /v2/admin/invites` | `invite.create.any` |
| `delete_invite` | `DELETE /v2/admin/invites/{code_base64}` | `invite.delete.any` |

低层客户端或自定义集成可以使用 `encode_invite_code(code)` 得到相同的传输值。请不要自行裁剪
邀请码原文；空格、大小写、引号和斜杠都属于邀请码的一部分。
