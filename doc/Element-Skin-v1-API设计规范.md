# Element Skin v1 API 设计规范

> 状态：现行迁移规范
>
> 目标读者：Element Skin 前端、后端、OAuth 应用开发者、站点管理员、第三方启动器/工具作者
>
> 参考代码快照：`dev-next`，2026-06-30

## 1. 设计目标

本规范定义 Element Skin 的第一版稳定站点 API。它服务两个目标：

1. 让站点前端、第三方应用、未来 OAuth 客户端使用同一套站点能力 API。
2. 借 breaking change 机会整理当前分散在 `/me`、`/public`、`/notices`、`/microsoft`、`/remote-ygg`、`/admin` 下的站点路由。

本规范一经确认，后续实现应以此为公开契约，不应随意调整路径、字段含义或权限语义。必须变更时，应通过新版本 API 或明确迁移策略处理。

## 2. 现有路由调研结论

当前后端站点能力分布如下：

| 当前分组 | 当前路径前缀 | 说明 | v1 处理 |
| --- | --- | --- | --- |
| 站点会话 | `/site-login`、`/site-logout`、`/register`、`/reset-password` | 登录、登出、注册、验证码、密码重置 | 迁移到 `/v1/auth/*` |
| 当前用户账号 | `/me` | 当前登录用户资料、密码、自身账号删除 | 迁移到 `/v1/users/me/*` |
| 当前用户角色 | `/me/profiles` | 用户自己的 Minecraft 角色 | 迁移到 `/v1/users/me/profiles/*` |
| 当前用户材质 | `/me/textures`、`/textures/upload` | 用户自己的材质、衣柜、材质应用 | 迁移到 `/v1/users/me/textures/*` |
| 公开站点能力 | `/public` | 公开设置、首页媒体、皮肤库、fallback 状态 | 迁移到 `/v1/public/*` |
| 用户通知 | `/notices` | 用户通知和公告读取、已读、忽略 | 迁移到 `/v1/notifications/*` |
| Microsoft 导入 | `/microsoft` | Microsoft 正版角色导入流程 | 迁移到 `/v1/imports/microsoft/*` |
| 远程 Yggdrasil 导入 | `/remote-ygg` | 从远程 Yggdrasil 数据导入角色 | 迁移到 `/v1/imports/remote-ygg/*` |
| 管理后台 | `/admin` | 用户、角色、材质、邀请码、白名单、首页媒体、公告、设置 | 迁移到 `/v1/admin/*` |
| Yggdrasil 协议 | `/authserver`、`/sessionserver`、部分 `/api` | 被启动器、游戏服务器、用户程序调用 | 保持原路径 |

## 3. 版本与兼容承诺

v1 API 的 base path 是：

```text
/v1
```

所有站点业务能力都应放在 `/v1` 下。OAuth 标准协议端点、Yggdrasil/Mojang 兼容端点不放入 `/v1`。

本次迁移是 breaking change。旧站点 API 不作为长期兼容接口保留：

```text
/me
/public
/notices
/microsoft
/remote-ygg
/admin
/site-login
/site-logout
/register
/send-verification-code
/reset-password
/textures/upload
```

实现迁移时，旧站点 API 可以在短期开发窗口内存在，但最终交付不应同时维护新旧两套站点 API。

## 4. 不迁移的 Yggdrasil / Mojang 兼容端点

以下端点不是站点 API，它们遵循 Yggdrasil/Mojang 兼容协议，被启动器、游戏服务器和外部用户程序直接调用，必须保持当前路径：

```text
GET    /
POST   /authserver/authenticate
POST   /authserver/refresh
POST   /authserver/validate
POST   /authserver/invalidate
POST   /authserver/signout
POST   /sessionserver/session/minecraft/join
GET    /sessionserver/session/minecraft/hasJoined
GET    /sessionserver/session/minecraft/profile/{uuid}
GET    /api/users/profiles/minecraft/{playerName}
GET    /users/profiles/minecraft/{playerName}
GET    /api/profiles/minecraft/{playerName}
POST   /api/profiles/minecraft
GET    /api/minecraft/profile/lookup/name/{playerName}
GET    /minecraft/profile/lookup/name/{playerName}
PUT    /api/user/profile/{uuid}/{texture_type}
DELETE /api/user/profile/{uuid}/{texture_type}
```

这些端点的权限语义仍由 Yggdrasil token、服务器 join 逻辑和现有 Yggdrasil 权限模型控制，不参与 `/v1` 站点 API 命名迁移。

## 5. 认证模型

v1 站点 API 支持统一 Actor 模型。请求进入后端后，认证层将凭证解析为 Actor，业务服务只检查 Actor 的权限，不关心凭证来源。

第一阶段凭证：

```text
Cookie: access_token=<site_session_token>
```

OAuth 引入后的凭证：

```text
Authorization: Bearer <oauth_access_token>
```

认证优先级：

1. 若存在合法 Bearer OAuth token，使用 OAuth Actor。
2. 否则若存在合法 Cookie access token，使用 Web Session Actor。
3. 两者都不存在时返回 `401`。

公开端点不要求认证。

### 5.1 OAuth Actor 权限裁剪

OAuth token 的最终权限必须是以下三者交集：

```text
用户当前有效权限
∩ OAuth 应用允许申请的权限
∩ 用户实际授权的权限
```

因此，OAuth 应用调用 v1 API 时仍使用本文档列出的同一组权限 code。

## 6. 通用协议约定

### 6.1 Content Type

JSON 请求：

```http
Content-Type: application/json
```

文件上传请求：

```http
Content-Type: multipart/form-data
```

JSON 响应：

```http
Content-Type: application/json; charset=utf-8
```

### 6.2 时间格式

除非特别说明，时间字段均为 Unix 毫秒时间戳：

```json
{
  "created_at": 1793232000000
}
```

### 6.3 错误格式

站点 API 的错误响应统一为：

```json
{
  "detail": "permission denied"
}
```

常用状态码：

| 状态码 | 含义 |
| --- | --- |
| `200` | 请求成功，响应体为 JSON |
| `204` | 请求成功，无响应体 |
| `400` | 请求格式、参数或业务输入错误 |
| `401` | 未认证或凭证无效 |
| `403` | 已认证但权限不足 |
| `404` | 资源不存在或对当前 Actor 不可见 |
| `409` | 资源冲突 |
| `429` | 请求被限流 |
| `500` | 服务端内部错误 |

Yggdrasil 协议端点可返回 Yggdrasil 标准错误格式，不适用本节。

### 6.4 Cursor 分页

分页列表统一使用 cursor 分页：

请求 query：

```text
limit=20
cursor=<opaque_cursor>
```

响应：

```json
{
  "items": [],
  "has_next": false,
  "next_cursor": null,
  "page_size": 20
}
```

规则：

- `limit` 默认 `20`。
- `limit` 最小 `1`。
- `limit` 最大 `100`。
- `cursor` 是不透明字符串，客户端不得解析或构造。
- `next_cursor` 为 `null` 或空值时表示没有下一页。

### 6.5 Bool 输入

JSON 中布尔字段应使用 true/false。部分历史字段可接受 `0`、`1`、`"true"`、`"false"`，但 v1 文档只承诺标准 JSON bool。

### 6.6 材质类型与模型

材质类型：

```text
skin
cape
```

皮肤模型：

```text
default
slim
```

## 7. 数据对象

### 7.1 User

```json
{
  "id": "user_id",
  "email": "user@example.com",
  "display_name": "Steve",
  "roles": ["user"],
  "permissions": ["profile.read.owned"],
  "avatar_hash": null,
  "banned_until": null,
  "profile_count": 1,
  "texture_count": 2,
  "preferred_language": "zh-CN"
}
```

### 7.2 Profile

```json
{
  "id": "profile_uuid",
  "name": "Steve",
  "model": "default",
  "texture_model": "default",
  "skin_hash": null,
  "cape_hash": null,
  "user_id": "owner_user_id",
  "owner_email": "owner@example.com",
  "owner_display_name": "Owner"
}
```

### 7.3 Texture

```json
{
  "hash": "texture_hash",
  "type": "skin",
  "model": "default",
  "note": "My skin",
  "is_public": true,
  "uploader": "user_id",
  "uploader_name": "Steve",
  "created_at": 1793232000000,
  "usage_count": 1
}
```

### 7.4 Notice

```json
{
  "id": "notice_id",
  "type": "announcement",
  "title": "Title",
  "summary": "Short text",
  "content_markdown": "Markdown body",
  "display_mode": "detail",
  "level": "info",
  "link_text": "",
  "link_url": "",
  "audience": "users",
  "enabled": true,
  "pinned": false,
  "dismissible": true,
  "starts_at": null,
  "ends_at": null,
  "created_by": "user_id",
  "created_at": 1793232000000,
  "updated_at": 1793232000000
}
```

用户侧通知视图额外包含：

```json
{
  "read": false,
  "read_at": null,
  "dismissed_at": null
}
```

### 7.5 HomepageMedia

```json
{
  "id": "media_id",
  "type": "image",
  "title": "Homepage",
  "storage_path": "media.webp",
  "overlay_opacity_light": 0.45,
  "overlay_opacity_dark": 0.45,
  "start_yaw": 0,
  "start_pitch": 0,
  "yaw_speed_dps": 4,
  "pitch_speed_dps": 0,
  "sort_order": 1,
  "enabled": true,
  "duration_ms": 6000,
  "created_at": 1793232000000,
  "updated_at": 1793232000000
}
```

## 8. 权限命名说明

v1 API 使用现有权限 catalog。权限 code 格式为：

```text
resource.action.scope
```

本文档列出的权限 code 均来自当前后端 catalog 与 route handler，不重新发明权限名。

OAuth 应用申请 scope 时也使用这些权限 code。

## 9. 认证与会话 API

### 9.1 登录

```http
POST /v1/auth/login
```

请求：

```json
{
  "email": "user@example.com",
  "password": "Password123"
}
```

响应：

```json
{
  "user_id": "user_id",
  "permissions": ["account.read.self"]
}
```

副作用：

- 设置 HttpOnly `access_token` cookie。
- 设置 HttpOnly `refresh_token` cookie。

认证：公开。

限流：按登录场景限流。

替代旧端点：

```text
POST /site-login
```

### 9.2 登出

```http
POST /v1/auth/logout
```

响应：

```json
{
  "ok": true
}
```

副作用：

- 若存在 refresh token，则撤销它。
- 清除 `access_token` 与 `refresh_token` cookie。

替代旧端点：

```text
POST /site-logout
```

### 9.3 注册

```http
POST /v1/auth/register
```

请求：

```json
{
  "email": "user@example.com",
  "password": "Password123",
  "username": "Steve",
  "invite": "INVITE_CODE",
  "code": "123456"
}
```

字段：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `email` | 是 | 注册邮箱 |
| `password` | 是 | 密码 |
| `username` | 是 | 初始显示名/角色名来源 |
| `invite` | 否 | 邀请码 |
| `code` | 否 | 邮箱验证码 |

响应：

```json
{
  "id": "user_id"
}
```

认证：公开。

替代旧端点：

```text
POST /register
```

### 9.4 发送验证码

```http
POST /v1/auth/verification-code
```

请求：

```json
{
  "email": "user@example.com",
  "type": "register"
}
```

`type` 取值：

```text
register
reset
```

响应：

```json
{
  "ok": true,
  "ttl": 300
}
```

认证：公开。

替代旧端点：

```text
POST /send-verification-code
```

### 9.5 重置密码

```http
POST /v1/auth/password/reset
```

请求：

```json
{
  "email": "user@example.com",
  "password": "NewPassword123",
  "code": "123456"
}
```

响应：

```json
{
  "ok": true
}
```

认证：公开。

替代旧端点：

```text
POST /reset-password
```

### 9.6 刷新站点会话

```http
POST /v1/auth/session/refresh
```

请求凭证：

```text
Cookie refresh_token
```

响应：

```json
{
  "permissions": ["account.read.self"]
}
```

副作用：

- 轮换站点 refresh token。
- 设置新的 `access_token` 与 `refresh_token` cookie。

说明：

- OAuth refresh token 不使用此端点。
- OAuth refresh 应使用 `/oauth/token` 的 `refresh_token` grant。

替代旧端点：

```text
POST /me/refresh-token
```

## 10. 当前用户账号 API

### 10.1 获取当前用户

```http
GET /v1/users/me
```

权限：

```text
account.read.self
```

响应：`User`。

替代旧端点：

```text
GET /me
```

### 10.2 修改当前用户

```http
PATCH /v1/users/me
```

权限：

```text
account.update.self
```

请求：

```json
{
  "email": "new@example.com",
  "display_name": "New Name",
  "preferred_language": "zh-CN",
  "avatar_hash": null
}
```

所有字段均可选。

响应：

```json
{
  "ok": true
}
```

副作用：

- 当前用户认证缓存失效。

替代旧端点：

```text
PATCH /me
```

### 10.3 注销当前账号

```http
DELETE /v1/users/me
```

权限：

```text
account.delete.self
```

响应：

```json
{
  "ok": true
}
```

限制：

- 持有受保护角色的用户不能删除自己的账号。

替代旧端点：

```text
DELETE /me
```

### 10.4 修改当前用户密码

```http
POST /v1/users/me/password
```

权限：

```text
account_password.update.self
```

请求：

```json
{
  "old_password": "OldPassword123",
  "new_password": "NewPassword123"
}
```

响应：

```json
{
  "ok": true,
  "message": "密码修改成功"
}
```

副作用：

- 当前用户认证缓存失效。

替代旧端点：

```text
POST /me/password
```

## 11. 当前用户角色 API

### 11.1 列出自己的角色

```http
GET /v1/users/me/profiles
```

权限：

```text
profile.read.owned
```

Query：

| 参数 | 说明 |
| --- | --- |
| `limit` | 分页大小 |
| `cursor` | 下一页游标 |

响应：

```json
{
  "items": [],
  "has_next": false,
  "next_cursor": null,
  "page_size": 20
}
```

替代旧端点：

```text
GET /me/profiles
```

### 11.2 创建自己的角色

```http
POST /v1/users/me/profiles
```

权限：

```text
profile.create.owned
```

请求：

```json
{
  "name": "Steve",
  "model": "default"
}
```

响应：

```json
{
  "id": "profile_uuid",
  "name": "Steve",
  "model": "default"
}
```

替代旧端点：

```text
POST /me/profiles
```

### 11.3 修改自己的角色

```http
PATCH /v1/users/me/profiles/{profile_id}
```

权限：

```text
profile.update.owned
```

请求：

```json
{
  "name": "Alex"
}
```

响应：

```json
{
  "ok": true
}
```

替代旧端点：

```text
PATCH /me/profiles/{pid}
```

### 11.4 删除自己的角色

```http
DELETE /v1/users/me/profiles/{profile_id}
```

权限：

```text
profile.delete.owned
```

响应：

```json
{
  "ok": true
}
```

替代旧端点：

```text
DELETE /me/profiles/{pid}
```

### 11.5 清除角色材质

```http
DELETE /v1/users/me/profiles/{profile_id}/skin
DELETE /v1/users/me/profiles/{profile_id}/cape
```

权限：

```text
texture.clear.owned
```

响应：

```json
{
  "ok": true
}
```

替代旧端点：

```text
DELETE /me/profiles/{pid}/skin
DELETE /me/profiles/{pid}/cape
```

## 12. 当前用户材质与衣柜 API

### 12.1 列出自己的材质

```http
GET /v1/users/me/textures
```

权限：

```text
texture.read.owned
```

Query：

| 参数 | 说明 |
| --- | --- |
| `limit` | 分页大小 |
| `cursor` | 下一页游标 |
| `texture_type` | `skin` 或 `cape` |

响应：分页 `Texture`。

替代旧端点：

```text
GET /me/textures
```

### 12.2 上传自己的材质

```http
POST /v1/users/me/textures
```

权限：

```text
texture.create.owned
```

如果上传时设置公开，还需要：

```text
texture.update_visibility.owned
```

请求：`multipart/form-data`

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `file` | 是 | PNG 皮肤/披风文件，最大 16 MiB |
| `texture_type` | 否 | `skin` 或 `cape`，默认 `skin` |
| `note` | 否 | 材质备注 |
| `model` | 否 | `default` 或 `slim` |
| `is_public` | 否 | 是否公开 |

响应：

```json
{
  "hash": "texture_hash",
  "texture_type": "skin"
}
```

替代旧端点：

```text
POST /me/textures
```

### 12.3 获取自己的材质详情

```http
GET /v1/users/me/textures/{hash}/{texture_type}
```

权限：

```text
texture.read.owned
```

响应：`Texture`。

替代旧端点：

```text
GET /me/textures/{hash}/{texture_type}
```

### 12.4 修改自己的材质

```http
PATCH /v1/users/me/textures/{hash}/{texture_type}
```

请求：

```json
{
  "note": "New note",
  "model": "slim",
  "is_public": false
}
```

权限：

| 字段 | 权限 |
| --- | --- |
| `note` | `texture.update_metadata.owned` |
| `model` | `texture.update_metadata.owned` |
| `is_public` | `texture.update_visibility.owned` |

响应：

```json
{
  "ok": true
}
```

替代旧端点：

```text
PATCH /me/textures/{hash}/{texture_type}
```

### 12.5 删除自己的材质

```http
DELETE /v1/users/me/textures/{hash}/{texture_type}
```

权限：

```text
texture.delete.owned
```

响应：

```json
{
  "ok": true
}
```

替代旧端点：

```text
DELETE /me/textures/{hash}/{texture_type}
```

### 12.6 加入自己的衣柜

```http
POST /v1/users/me/textures/{hash}/wardrobe
```

权限：

```text
wardrobe_entry.add.owned
```

Query：

| 参数 | 说明 |
| --- | --- |
| `texture_type` | `skin` 或 `cape` |

响应：

```json
{
  "ok": true
}
```

替代旧端点：

```text
POST /me/textures/{hash}/add
```

### 12.7 应用材质到自己的角色

```http
POST /v1/users/me/textures/{hash}/apply
```

权限：

```text
texture.apply.owned
```

请求：

```json
{
  "profile_id": "profile_uuid",
  "texture_type": "skin"
}
```

响应：

```json
{
  "ok": true
}
```

替代旧端点：

```text
POST /me/textures/{hash}/apply
```

### 12.8 上传并应用材质

```http
POST /v1/users/me/textures/upload-and-apply
```

权限：

```text
texture.create.owned
texture.apply.owned
```

如果上传时设置公开，还需要：

```text
texture.update_visibility.owned
```

请求：`multipart/form-data`

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `file` | 是 | PNG 文件，最大 16 MiB |
| `uuid` | 是 | 目标角色 ID，沿用 authlib-injector 字段名 |
| `texture_type` | 是 | `skin` 或 `cape` |
| `model` | 否 | `default` 或 `slim` |
| `is_public` | 否 | 是否公开 |

响应：

```json
{
  "ok": true,
  "hash": "texture_hash",
  "type": "skin"
}
```

替代旧端点：

```text
POST /textures/upload
```

## 13. 公开站点 API

### 13.1 公开站点设置

```http
GET /v1/public/settings
```

认证：公开。

响应：`SiteSettings`。

替代旧端点：

```text
GET /public/settings
```

### 13.2 公开首页媒体

```http
GET /v1/public/homepage-media
```

认证：公开。

响应：`HomepageMedia[]`，仅返回 enabled 项。

替代旧端点：

```text
GET /public/homepage-media
```

### 13.3 公开 fallback 状态

```http
GET /v1/public/fallback-status
```

认证：公开。

响应：

```json
{
  "endpoints": [],
  "retention_ms": 86400000,
  "generated_at": 1793232000000
}
```

替代旧端点：

```text
GET /public/fallback-status
```

### 13.4 公开皮肤库

```http
GET /v1/public/skin-library
```

认证：公开。

Query：

| 参数 | 说明 |
| --- | --- |
| `limit` | 分页大小 |
| `cursor` | 下一页游标 |
| `texture_type` | `skin` 或 `cape` |
| `q` | 搜索关键词 |
| `sort` | `latest` 或 `most_used` |

响应：分页 `Texture`。

替代旧端点：

```text
GET /public/skin-library
```

## 14. 通知 API

通知系统包含公告和系统消息。公告是通知的一种，不应单独设计 `/notices` 作为 v1 主路径。

### 14.1 列出当前用户通知

```http
GET /v1/notifications
```

权限：

```text
notice.read.owned
```

Query：

| 参数 | 说明 |
| --- | --- |
| `limit` | 分页大小 |
| `cursor` | 下一页游标 |
| `type` | `announcement` 或 `system` |
| `include_read` | 是否包含已读，默认包含；`false` 表示排除已读 |
| `dashboard` | 仪表盘摘要模式；`true` 时默认只取公告 |

响应：分页 `NoticeView`。

替代旧端点：

```text
GET /notices
```

### 14.2 获取通知详情

```http
GET /v1/notifications/{notification_id}
```

权限：

```text
notice.read.owned
```

响应：`NoticeView`。

副作用：

- 将该通知标记为已读。

替代旧端点：

```text
GET /notices/{id}
```

### 14.3 标记通知已读

```http
POST /v1/notifications/{notification_id}/read
```

权限：

```text
notice.read.owned
```

响应：`204 No Content`。

替代旧端点：

```text
POST /notices/{id}/read
```

### 14.4 忽略通知

```http
POST /v1/notifications/{notification_id}/dismiss
```

权限：

```text
notice.dismiss.owned
```

响应：`204 No Content`。

限制：

- 仅 `dismissible=true` 的通知可忽略。

替代旧端点：

```text
POST /notices/{id}/dismiss
```

## 15. Microsoft 正版角色导入 API

Microsoft 是角色导入能力，不是账号绑定能力，因此归入 `/v1/imports/microsoft`。

### 15.1 获取 Microsoft 授权 URL

```http
GET /v1/imports/microsoft/auth-url
```

权限：

```text
microsoft_import.start.owned
```

响应：

```json
{
  "auth_url": "https://login.live.com/oauth20_authorize.srf?...",
  "state": "opaque_state"
}
```

替代旧端点：

```text
GET /microsoft/auth-url
```

### 15.2 Microsoft 回调

```http
GET /v1/imports/microsoft/callback
```

Query：

| 参数 | 说明 |
| --- | --- |
| `code` | Microsoft authorization code |
| `state` | 站点生成的 state |
| `error` | Microsoft 返回的错误 |

响应：

- 成功：重定向到站点前端角色管理页，并携带 `ms_token`。
- 失败：重定向到站点前端角色管理页，并携带错误标记。

配置迁移：

当前默认 `microsoft_redirect_uri` 应从：

```text
{api_url}/microsoft/callback
```

迁移为：

```text
{api_url}/v1/imports/microsoft/callback
```

替代旧端点：

```text
GET /microsoft/callback
```

### 15.3 读取 Microsoft 角色资料

```http
POST /v1/imports/microsoft/profile
```

权限：

```text
microsoft_import.read_profile.owned
```

请求：

```json
{
  "ms_token": "one_time_profile_token"
}
```

响应：

```json
{
  "profile": {
    "id": "minecraft_profile_id",
    "name": "Steve",
    "skins": [],
    "capes": []
  },
  "has_game": true,
  "import_token": "one_time_import_token"
}
```

替代旧端点：

```text
POST /microsoft/get-profile
```

### 15.4 导入 Microsoft 角色

```http
POST /v1/imports/microsoft/profile/import
```

权限：

```text
microsoft_import.create_profile.owned
```

请求：

```json
{
  "ms_token": "one_time_import_token"
}
```

响应：

```json
{
  "profile": {},
  "textures": []
}
```

具体响应沿用 import service 当前结果结构。

替代旧端点：

```text
POST /microsoft/import-profile
```

## 16. 远程 Yggdrasil 导入 API

这组 API 是站点导入工具，不是 Yggdrasil 协议端点，因此迁移到 `/v1`。

### 16.1 预览远程角色

```http
POST /v1/imports/remote-ygg/profiles/preview
```

权限：

```text
profile.create.owned
```

请求：

```json
{
  "api_url": "https://remote.example.com/api/yggdrasil",
  "username": "steve@example.com",
  "password": "remote_password"
}
```

响应：

```json
{
  "profiles": [
    {
      "id": "remote_profile_id",
      "name": "Steve"
    }
  ]
}
```

说明：

- `api_url` 是远程 Yggdrasil 服务地址，不是本站 API 地址。
- 后端应使用远程服务认证结果列出可导入角色。
- 响应只返回导入预览所需的角色摘要，不返回远程服务密码、远程 access token 或其他敏感凭证。

替代旧端点：

```text
POST /remote-ygg/get-profiles
```

### 16.2 导入单个远程角色

```http
POST /v1/imports/remote-ygg/profiles/import
```

权限：

```text
profile.create.owned
texture.create.owned
```

请求：

```json
{
  "api_url": "https://remote.example.com/api/yggdrasil",
  "username": "steve@example.com",
  "password": "remote_password",
  "profile_id": "remote_profile_id",
  "profile_name": "Steve"
}
```

响应：

```json
{
  "id": "profile_uuid",
  "name": "Steve"
}
```

替代旧端点：

```text
POST /remote-ygg/import-profile
```

### 16.3 批量导入远程角色

```http
POST /v1/imports/remote-ygg/profiles/import-batch
```

权限：

```text
profile.create.owned
texture.create.owned
```

请求：

```json
{
  "api_url": "https://remote.example.com/api/yggdrasil",
  "username": "steve@example.com",
  "password": "remote_password",
  "profiles": [
    {
      "profile_id": "remote_profile_id",
      "profile_name": "Steve"
    }
  ]
}
```

响应：

```json
{
  "items": [],
  "success_count": 1,
  "failure_count": 0,
  "failed": []
}
```

替代旧端点：

```text
POST /remote-ygg/import-profiles
```

## 17. 管理员用户 API

### 17.1 列出用户

```http
GET /v1/admin/users
```

权限：

```text
user.read.any
```

Query：

| 参数 | 说明 |
| --- | --- |
| `limit` | 分页大小，默认 15 |
| `cursor` | 下一页游标 |
| `q` | 搜索关键词 |

响应：分页 `User`。

替代旧端点：

```text
GET /admin/users
```

### 17.2 获取用户详情

```http
GET /v1/admin/users/{user_id}
```

权限：

```text
account.read.any
```

响应：`User`。

### 17.3 删除用户

```http
DELETE /v1/admin/users/{user_id}
```

权限：

```text
account.delete.any
```

限制：

- 不能删除自己。
- 不能修改受保护角色持有者，除非 Actor 拥有 `permission_protected.manage.any`。

响应：

```json
{
  "ok": true
}
```

### 17.4 获取用户角色列表

```http
GET /v1/admin/users/{user_id}/profiles
```

权限：

```text
profile.read.any
```

Query：

| 参数 | 说明 |
| --- | --- |
| `limit` | 分页大小 |
| `cursor` | 下一页游标 |

响应：分页 `Profile`。

### 17.5 获取用户权限状态

```http
GET /v1/admin/users/{user_id}/permissions
```

权限：由 permission service 内部按当前 Actor 校验。

响应：

```json
{
  "roles": [],
  "effective_permissions": [],
  "overrides": [],
  "catalog": {
    "permissions": [],
    "roles": []
  }
}
```

### 17.6 授予/撤销角色

```http
PUT    /v1/admin/users/{user_id}/roles/{role_id}
DELETE /v1/admin/users/{user_id}/roles/{role_id}
```

权限：

```text
permission.grant.any
permission.revoke.any
```

管理受保护角色还需要：

```text
permission_protected.manage.any
```

响应：

```json
{
  "ok": true,
  "role_id": "admin"
}
```

### 17.7 设置/清除用户单项权限覆盖

```http
PUT    /v1/admin/users/{user_id}/permissions/{permission_code}
DELETE /v1/admin/users/{user_id}/permissions/{permission_code}
```

PUT 请求：

```json
{
  "effect": "allow"
}
```

`effect` 取值：

```text
allow
deny
```

权限：由 permission service 内部按当前 Actor 校验。

响应：

```json
{
  "ok": true,
  "permission_code": "notice.create.any",
  "effect": "allow"
}
```

### 17.8 封禁/解封用户

```http
POST /v1/admin/users/{user_id}/ban
POST /v1/admin/users/{user_id}/unban
```

权限：

```text
account.ban.any
account.unban.any
```

封禁请求：

```json
{
  "banned_until": 1793232000000
}
```

语义：

- 当前账号封禁语义是禁止加入 Minecraft 服务器。
- 它不是禁止网页登录。

响应：

```json
{
  "ok": true,
  "banned_until": 1793232000000
}
```

解封响应：

```json
{
  "ok": true
}
```

### 17.9 管理员重置用户密码

```http
POST /v1/admin/users/password/reset
```

权限：

```text
account.update.any
```

请求：

```json
{
  "user_id": "user_id",
  "new_password": "NewPassword123"
}
```

响应：

```json
{
  "ok": true
}
```

替代旧端点：

```text
POST /admin/users/reset-password
```

## 18. 管理员角色 API

### 18.1 列出全部角色

```http
GET /v1/admin/profiles
```

权限：

```text
profile.read.any
```

Query：

| 参数 | 说明 |
| --- | --- |
| `limit` | 分页大小 |
| `cursor` | 下一页游标 |
| `q` | 搜索关键词 |

响应：分页 `Profile`。

### 18.2 修改角色

```http
PATCH /v1/admin/profiles/{profile_id}
```

权限：

```text
profile.update.any
```

请求：

```json
{
  "name": "NewName"
}
```

响应：

```json
{
  "ok": true
}
```

### 18.3 删除角色

```http
DELETE /v1/admin/profiles/{profile_id}
```

权限：

```text
profile.delete.any
```

响应：

```json
{
  "ok": true
}
```

### 18.4 设置/清除角色材质

```http
PATCH /v1/admin/profiles/{profile_id}/skin
PATCH /v1/admin/profiles/{profile_id}/cape
```

权限：

```text
profile.update.any
```

请求：

```json
{
  "hash": "texture_hash"
}
```

清除材质：

```json
{
  "hash": null
}
```

响应：

```json
{
  "ok": true
}
```

## 19. 管理员材质 API

### 19.1 列出全部材质

```http
GET /v1/admin/textures
```

权限：

```text
texture.read.any
```

Query：

| 参数 | 说明 |
| --- | --- |
| `limit` | 分页大小 |
| `cursor` | 下一页游标 |
| `q` | 搜索关键词 |
| `type` | `skin` 或 `cape` |

响应：分页 `Texture`。

### 19.2 修改材质

```http
PATCH /v1/admin/textures/{hash}
```

请求：

```json
{
  "type": "skin",
  "note": "New note",
  "model": "slim",
  "is_public": true
}
```

Query：

| 参数 | 说明 |
| --- | --- |
| `type` | 可替代 body 内的 `type`，默认 `skin` |

权限：

| 字段 | 权限 |
| --- | --- |
| `note` | `texture.update_metadata.any` |
| `model` | `texture.update_metadata.any` |
| `is_public` | `texture.update_visibility.any` |

响应：

```json
{
  "ok": true
}
```

### 19.3 删除材质

```http
DELETE /v1/admin/textures/{hash}
```

权限：

```text
texture.delete.any
```

Query：

| 参数 | 说明 |
| --- | --- |
| `type` | `skin` 或 `cape`，默认 `skin` |
| `user_id` | 指定用户材质记录 |
| `force` | `true` 表示强制删除 |

响应：

```json
{
  "success": true
}
```

## 20. 管理员邀请码 API

### 20.1 列出邀请码

```http
GET /v1/admin/invites
```

权限：

```text
invite.read.any
```

Query：

| 参数 | 说明 |
| --- | --- |
| `limit` | 分页大小，默认 15 |
| `cursor` | 下一页游标 |

响应：分页 `Invite`。

### 20.2 创建邀请码

```http
POST /v1/admin/invites
```

权限：

```text
invite.create.any
```

请求：

```json
{
  "code": "INVITE",
  "total_uses": 5,
  "note": "For tester"
}
```

字段：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `code` | 否 | 不传时服务端生成 |
| `total_uses` | 否 | 正整数，默认 1 |
| `note` | 否 | 备注 |

响应：

```json
{
  "code": "INVITE",
  "total_uses": 5,
  "note": "For tester"
}
```

### 20.3 删除邀请码

```http
DELETE /v1/admin/invites/{code}
```

权限：

```text
invite.delete.any
```

响应：

```json
{
  "ok": true
}
```

## 21. 管理员官方白名单 API

### 21.1 列出官方白名单用户

```http
GET /v1/admin/official-whitelist
```

权限：

```text
official_whitelist.read.any
```

Query：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `endpoint_id` | 是 | fallback endpoint ID |

响应：

```json
{
  "items": []
}
```

### 21.2 添加官方白名单用户

```http
POST /v1/admin/official-whitelist
```

权限：

```text
official_whitelist.add.any
```

请求：

```json
{
  "username": "Steve",
  "endpoint_id": 1
}
```

响应：

```json
{
  "ok": true
}
```

### 21.3 移除官方白名单用户

```http
DELETE /v1/admin/official-whitelist/{username}
```

权限：

```text
official_whitelist.remove.any
```

Query：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `endpoint_id` | 是 | fallback endpoint ID |

响应：

```json
{
  "ok": true
}
```

## 22. 管理员首页媒体 API

### 22.1 列出首页媒体

```http
GET /v1/admin/homepage-media
```

权限：

```text
homepage_media.read.any
```

响应：`HomepageMedia[]`，包含启用和禁用项。

### 22.2 上传首页图片

```http
POST /v1/admin/homepage-media/image
```

权限：

```text
homepage_media.create.any
```

请求：`multipart/form-data`

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `file` | 是 | `.png`、`.jpg`、`.jpeg`、`.webp`，最大 5 MiB |
| `duration_ms` | 否 | 1000 到 60000，默认 6000 |
| `overlay_opacity_light` | 否 | 0 到 0.9，默认 0.45 |
| `overlay_opacity_dark` | 否 | 0 到 0.9，默认 0.45 |

响应：`HomepageMedia`。

### 22.3 上传首页全景图

```http
POST /v1/admin/homepage-media/panorama
```

权限：

```text
homepage_media.create.any
```

请求：`multipart/form-data`

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `file` | 是 | `.zip`，最大 50 MiB |
| `duration_ms` | 否 | 1000 到 60000，默认 9000 |
| `overlay_opacity_light` | 否 | 0 到 0.9，默认 0.45 |
| `overlay_opacity_dark` | 否 | 0 到 0.9，默认 0.45 |
| `start_yaw` | 否 | -360 到 360 |
| `start_pitch` | 否 | -89 到 89 |
| `yaw_speed_dps` | 否 | -90 到 90 |
| `pitch_speed_dps` | 否 | -90 到 90 |

ZIP 要求：

- 必须只包含 `panorama_0.png` 到 `panorama_5.png`。
- 文件必须在 zip 根目录。
- 单个面最大 5 MiB。

响应：`HomepageMedia`。

### 22.4 修改首页媒体

```http
PATCH /v1/admin/homepage-media/{id}
```

权限：

```text
homepage_media.update.any
```

请求：

```json
{
  "title": "Title",
  "enabled": true,
  "duration_ms": 6000,
  "overlay_opacity_light": 0.45,
  "overlay_opacity_dark": 0.45,
  "start_yaw": 0,
  "start_pitch": 0,
  "yaw_speed_dps": 4,
  "pitch_speed_dps": 0
}
```

响应：`HomepageMedia`。

### 22.5 首页媒体排序

```http
PATCH /v1/admin/homepage-media/reorder
```

权限：

```text
homepage_media.update.any
```

请求：

```json
{
  "ids": ["media_id_1", "media_id_2"]
}
```

响应：

```json
{
  "ok": true
}
```

### 22.6 删除首页媒体

```http
DELETE /v1/admin/homepage-media/{id}
```

权限：

```text
homepage_media.delete.any
```

响应：

```json
{
  "ok": true
}
```

## 23. 管理员通知 API

管理员通知 API 管理所有通知类型。公告是 `type=announcement` 的通知。

### 23.1 列出通知

```http
GET /v1/admin/notifications
```

权限：

```text
notice.read.any
```

Query：

| 参数 | 说明 |
| --- | --- |
| `limit` | 分页大小 |
| `cursor` | 下一页游标 |
| `type` | `announcement` 或 `system` |
| `status` | `all`、`enabled`、`disabled`、`scheduled`、`expired` |

响应：分页 `Notice`。

替代旧端点：

```text
GET /admin/notices
```

### 23.2 创建通知

```http
POST /v1/admin/notifications
```

权限：

```text
notice.create.any
```

请求：

```json
{
  "type": "announcement",
  "title": "Title",
  "summary": "Summary",
  "content_markdown": "Markdown body",
  "display_mode": "detail",
  "level": "info",
  "link_text": "",
  "link_url": "",
  "audience": "users",
  "enabled": true,
  "pinned": false,
  "dismissible": true,
  "starts_at": null,
  "ends_at": null
}
```

字段约束：

| 字段 | 约束 |
| --- | --- |
| `type` | `announcement` 或 `system`，默认 `announcement` |
| `title` | 必填，最多 80 个 Unicode 字符 |
| `summary` | 最多 160 个 Unicode 字符；长公告必填 |
| `content_markdown` | 最多 20 KiB；长公告必填，短公告可为空 |
| `display_mode` | `inline` 或 `detail`，默认 `inline` |
| `level` | `info`、`success`、`warning`、`danger`，默认 `info` |
| `audience` | `users` 或 `admins`，默认 `users` |
| `link_text` / `link_url` | 必须同时提供或同时为空 |
| `link_url` | 仅允许站内 `/path`、`http://` 或 `https://` |
| `starts_at` | Unix 毫秒或 null |
| `ends_at` | Unix 毫秒或 null，必须大于 `starts_at` |

响应：`Notice`。

替代旧端点：

```text
POST /admin/notices
```

### 23.3 删除通知

```http
DELETE /v1/admin/notifications/{notification_id}
```

权限：

```text
notice.delete.any
```

响应：`204 No Content`。

替代旧端点：

```text
DELETE /admin/notices/{id}
```

### 23.4 替换通知

```http
PATCH /v1/admin/notifications/{notification_id}
```

权限：

```text
notice.update.any
```

请求：同创建通知，所有字段均可选。

语义：

`PATCH` 不是原地修改通知，而是执行一次替换发布。服务端必须按以下流程处理：

```text
DELETE /v1/admin/notifications/{old_id}
POST   /v1/admin/notifications
```

原因：

- 通知是面向用户投递的消息记录。
- 直接修改历史通知会造成用户已读、忽略、审计和历史语义混乱。
- 新内容应作为新通知发布。

当前旧端点：

```text
PATCH /admin/notices/{id}
```

迁移为 `PATCH /v1/admin/notifications/{notification_id}`，但 v1 的语义必须是替换发布，不允许保留旧通知记录并直接覆盖内容字段。

## 24. 管理员设置 API

### 24.1 读取站点设置

```http
GET /v1/admin/settings/site
```

权限：

```text
site_settings.read.any
```

响应：设置键值对象。

### 24.2 保存站点设置

```http
POST /v1/admin/settings/site
```

权限：

```text
site_settings.update.any
```

请求：设置键值对象。

响应：

```json
{
  "ok": true
}
```

副作用：

- 设置缓存失效。
- 公开设置缓存失效。

### 24.3 读取设置分组

```http
GET /v1/admin/settings/{group}
```

权限：

```text
site_settings.read.any
```

响应：设置键值对象。

### 24.4 保存设置分组

```http
POST /v1/admin/settings/{group}
```

权限：

```text
site_settings.update.any
```

请求：设置键值对象。

响应：

```json
{
  "ok": true
}
```

公开缓存失效分组：

```text
site
fallback
email
easter_eggs
```

## 25. OAuth 标准端点

OAuth 协议端点不放入 `/v1`，以符合生态工具和标准发现习惯。

v1 OAuth 协议端点：

```text
GET  /.well-known/oauth-authorization-server
GET  /.well-known/oauth-protected-resource
GET  /oauth/authorize
POST /oauth/token
POST /oauth/revoke
POST /oauth/introspect
```

支持 grant type：

```text
authorization_code
refresh_token
```

不支持：

```text
password
implicit
```

Device Authorization Grant、Client Credentials、DPoP、PAR、JAR、RAR 等扩展见 `doc/OAuth2.1标准与扩展参考.md`。

## 26. OAuth 应用与授权管理 API

OAuth 应用和授权管理是站点业务能力，因此放入 `/v1`。

### 26.1 开发者应用

```http
GET    /v1/developer/oauth/apps
POST   /v1/developer/oauth/apps
GET    /v1/developer/oauth/apps/{client_id}
PATCH  /v1/developer/oauth/apps/{client_id}
DELETE /v1/developer/oauth/apps/{client_id}
POST   /v1/developer/oauth/apps/{client_id}/secret/rotate
```

权限：

```text
oauth_app.read.owned
oauth_app.create.owned
oauth_app.update.owned
oauth_app.delete.owned
```

说明：

- OAuth 应用资源必须进入权限 catalog。
- 应用允许申请的权限上限应使用现有 permission code。

### 26.2 当前用户授权

```http
GET    /v1/users/me/oauth/grants
GET    /v1/users/me/oauth/grants/{grant_id}
DELETE /v1/users/me/oauth/grants/{grant_id}
```

权限：

```text
oauth_grant.read.owned
oauth_grant.revoke.owned
```

### 26.3 管理员 OAuth 管理

```http
GET    /v1/admin/oauth/apps
GET    /v1/admin/oauth/apps/{client_id}
PATCH  /v1/admin/oauth/apps/{client_id}
GET    /v1/admin/oauth/grants
DELETE /v1/admin/oauth/grants/{grant_id}
```

权限：

```text
oauth_app.read.any
oauth_app.update.any
oauth_grant.read.any
oauth_grant.revoke.any
```

## 27. 能力发现 API

### 27.1 站点能力

```http
GET /v1/capabilities
```

认证：公开。

响应：

```json
{
  "api_version": "v1",
  "site_name": "Element Skin",
  "site_url": "https://skin.example.com",
  "api_url": "https://skin.example.com",
  "features": {
    "skin_library": true,
    "oauth": true,
    "device_code": false,
    "microsoft_import": true,
    "remote_ygg_import": true
  },
  "upload_limits": {
    "texture_max_bytes": 16777216,
    "homepage_image_max_bytes": 5242880,
    "homepage_panorama_max_bytes": 52428800
  },
  "texture_types": ["skin", "cape"],
  "skin_models": ["default", "slim"]
}
```

### 27.2 权限目录

```http
GET /v1/permissions/catalog
```

认证：公开可读。

响应结构：

```json
{
  "permissions": [
    {
      "code": "profile.read.owned",
      "description": "读取自己的角色",
      "resource": "profile",
      "resource_description": "角色",
      "action": "read",
      "action_description": "读取",
      "scope": "owned",
      "scope_description": "自有业务资源",
      "delegable": true,
      "admin_delegable": false,
      "protected": false
    }
  ],
  "roles": []
}
```

说明：

- `delegable` 表示 OAuth 应用能否申请该权限。
- `admin_delegable` 表示是否属于管理员能力下放。
- `protected` 表示是否涉及受保护权限主体。

## 28. 路由迁移表

| 旧路径 | 新路径 |
| --- | --- |
| `POST /site-login` | `POST /v1/auth/login` |
| `POST /site-logout` | `POST /v1/auth/logout` |
| `POST /register` | `POST /v1/auth/register` |
| `POST /send-verification-code` | `POST /v1/auth/verification-code` |
| `POST /reset-password` | `POST /v1/auth/password/reset` |
| `POST /me/refresh-token` | `POST /v1/auth/session/refresh` |
| `GET /me` | `GET /v1/users/me` |
| `PATCH /me` | `PATCH /v1/users/me` |
| `DELETE /me` | `DELETE /v1/users/me` |
| `POST /me/password` | `POST /v1/users/me/password` |
| `GET /me/profiles` | `GET /v1/users/me/profiles` |
| `POST /me/profiles` | `POST /v1/users/me/profiles` |
| `PATCH /me/profiles/{pid}` | `PATCH /v1/users/me/profiles/{profile_id}` |
| `DELETE /me/profiles/{pid}` | `DELETE /v1/users/me/profiles/{profile_id}` |
| `DELETE /me/profiles/{pid}/skin` | `DELETE /v1/users/me/profiles/{profile_id}/skin` |
| `DELETE /me/profiles/{pid}/cape` | `DELETE /v1/users/me/profiles/{profile_id}/cape` |
| `GET /me/textures` | `GET /v1/users/me/textures` |
| `POST /me/textures` | `POST /v1/users/me/textures` |
| `GET /me/textures/{hash}/{texture_type}` | `GET /v1/users/me/textures/{hash}/{texture_type}` |
| `PATCH /me/textures/{hash}/{texture_type}` | `PATCH /v1/users/me/textures/{hash}/{texture_type}` |
| `DELETE /me/textures/{hash}/{texture_type}` | `DELETE /v1/users/me/textures/{hash}/{texture_type}` |
| `POST /me/textures/{hash}/add` | `POST /v1/users/me/textures/{hash}/wardrobe` |
| `POST /me/textures/{hash}/apply` | `POST /v1/users/me/textures/{hash}/apply` |
| `POST /textures/upload` | `POST /v1/users/me/textures/upload-and-apply` |
| `GET /public/settings` | `GET /v1/public/settings` |
| `GET /public/homepage-media` | `GET /v1/public/homepage-media` |
| `GET /public/fallback-status` | `GET /v1/public/fallback-status` |
| `GET /public/skin-library` | `GET /v1/public/skin-library` |
| `GET /notices` | `GET /v1/notifications` |
| `GET /notices/{id}` | `GET /v1/notifications/{notification_id}` |
| `POST /notices/{id}/read` | `POST /v1/notifications/{notification_id}/read` |
| `POST /notices/{id}/dismiss` | `POST /v1/notifications/{notification_id}/dismiss` |
| `GET /microsoft/auth-url` | `GET /v1/imports/microsoft/auth-url` |
| `GET /microsoft/callback` | `GET /v1/imports/microsoft/callback` |
| `POST /microsoft/get-profile` | `POST /v1/imports/microsoft/profile` |
| `POST /microsoft/import-profile` | `POST /v1/imports/microsoft/profile/import` |
| `POST /remote-ygg/get-profiles` | `POST /v1/imports/remote-ygg/profiles/preview` |
| `POST /remote-ygg/import-profile` | `POST /v1/imports/remote-ygg/profiles/import` |
| `POST /remote-ygg/import-profiles` | `POST /v1/imports/remote-ygg/profiles/import-batch` |
| `GET /admin/users` | `GET /v1/admin/users` |
| `GET /admin/users/{user_id}` | `GET /v1/admin/users/{user_id}` |
| `GET /admin/users/{user_id}/profiles` | `GET /v1/admin/users/{user_id}/profiles` |
| `GET /admin/users/{user_id}/permissions` | `GET /v1/admin/users/{user_id}/permissions` |
| `PUT /admin/users/{user_id}/roles/{role_id}` | `PUT /v1/admin/users/{user_id}/roles/{role_id}` |
| `DELETE /admin/users/{user_id}/roles/{role_id}` | `DELETE /v1/admin/users/{user_id}/roles/{role_id}` |
| `PUT /admin/users/{user_id}/permissions/{permission_code}` | `PUT /v1/admin/users/{user_id}/permissions/{permission_code}` |
| `DELETE /admin/users/{user_id}/permissions/{permission_code}` | `DELETE /v1/admin/users/{user_id}/permissions/{permission_code}` |
| `DELETE /admin/users/{user_id}` | `DELETE /v1/admin/users/{user_id}` |
| `POST /admin/users/{user_id}/ban` | `POST /v1/admin/users/{user_id}/ban` |
| `POST /admin/users/{user_id}/unban` | `POST /v1/admin/users/{user_id}/unban` |
| `POST /admin/users/reset-password` | `POST /v1/admin/users/password/reset` |
| `GET /admin/profiles` | `GET /v1/admin/profiles` |
| `PATCH /admin/profiles/{profile_id}` | `PATCH /v1/admin/profiles/{profile_id}` |
| `DELETE /admin/profiles/{profile_id}` | `DELETE /v1/admin/profiles/{profile_id}` |
| `PATCH /admin/profiles/{profile_id}/skin` | `PATCH /v1/admin/profiles/{profile_id}/skin` |
| `PATCH /admin/profiles/{profile_id}/cape` | `PATCH /v1/admin/profiles/{profile_id}/cape` |
| `GET /admin/textures` | `GET /v1/admin/textures` |
| `PATCH /admin/textures/{hash}` | `PATCH /v1/admin/textures/{hash}` |
| `DELETE /admin/textures/{hash}` | `DELETE /v1/admin/textures/{hash}` |
| `GET /admin/invites` | `GET /v1/admin/invites` |
| `POST /admin/invites` | `POST /v1/admin/invites` |
| `DELETE /admin/invites/{code}` | `DELETE /v1/admin/invites/{code}` |
| `GET /admin/official-whitelist` | `GET /v1/admin/official-whitelist` |
| `POST /admin/official-whitelist` | `POST /v1/admin/official-whitelist` |
| `DELETE /admin/official-whitelist/{username}` | `DELETE /v1/admin/official-whitelist/{username}` |
| `GET /admin/homepage-media` | `GET /v1/admin/homepage-media` |
| `POST /admin/homepage-media/image` | `POST /v1/admin/homepage-media/image` |
| `POST /admin/homepage-media/panorama` | `POST /v1/admin/homepage-media/panorama` |
| `PATCH /admin/homepage-media/reorder` | `PATCH /v1/admin/homepage-media/reorder` |
| `PATCH /admin/homepage-media/{id}` | `PATCH /v1/admin/homepage-media/{id}` |
| `DELETE /admin/homepage-media/{id}` | `DELETE /v1/admin/homepage-media/{id}` |
| `GET /admin/notices` | `GET /v1/admin/notifications` |
| `POST /admin/notices` | `POST /v1/admin/notifications` |
| `PATCH /admin/notices/{id}` | `PATCH /v1/admin/notifications/{notification_id}`，语义为删除旧通知后创建替代通知 |
| `DELETE /admin/notices/{id}` | `DELETE /v1/admin/notifications/{notification_id}` |
| `GET /admin/settings/site` | `GET /v1/admin/settings/site` |
| `POST /admin/settings/site` | `POST /v1/admin/settings/site` |
| `GET /admin/settings/{group}` | `GET /v1/admin/settings/{group}` |
| `POST /admin/settings/{group}` | `POST /v1/admin/settings/{group}` |

## 29. 实现注意事项

### 29.1 API URL 配置

当前后端公开设置中已经包含 `api_url`。迁移后：

- `api_url` 仍表示站点 API 根地址，不自动包含 `/v1`。
- 前端 API client 应统一拼接 `/v1/...`。
- Microsoft redirect URI 默认值必须同步迁移到 `/v1/imports/microsoft/callback`。

### 29.2 前端迁移

前端所有 API client 应迁移到 v1 路径。

页面路由不受本规范影响，页面路径如下：

```text
/dashboard
/admin
/notifications
```

这些是 SPA 页面路径，不是后端 API。

### 29.3 测试迁移

以下测试必须同步更新：

- `skin-backend/internal/httpapi/*`
- `skin-backend/internal/integration/*`
- `skin-backend/cmd/loadtest/*`
- `element-skin/src/api/__tests__/*`

Yggdrasil 相关测试不应迁移路径。

### 29.4 Loadtest

loadtest 应覆盖 v1 路径：

```text
/v1/public/settings
/v1/auth/login
/v1/users/me
/v1/users/me/profiles
/v1/users/me/textures
/v1/admin/users
/v1/admin/profiles
/v1/admin/textures
/v1/admin/invites
/v1/admin/settings/site
```

Yggdrasil loadtest 路径保持原样。

## 30. OAuth 资源权限

OAuth 应用和授权是站点能力资源。v1 实现 OAuth 时必须新增以下资源：

```text
oauth_app
oauth_grant
oauth_token
```

权限：

```text
oauth_app.read.owned
oauth_app.create.owned
oauth_app.update.owned
oauth_app.delete.owned
oauth_app.read.any
oauth_app.update.any

oauth_grant.read.owned
oauth_grant.revoke.owned
oauth_grant.read.any
oauth_grant.revoke.any

oauth_token.revoke.owned
oauth_token.introspect.any
```

这些权限名应进入权限模型文档，并通过 catalog 测试固定。

## 31. 迁移确认项

本规范确认以下 breaking change：

1. 当前用户资源统一迁移到 `/v1/users/me/*`。
2. 用户通知统一迁移到 `/v1/notifications/*`。
3. Microsoft 导入统一迁移到 `/v1/imports/microsoft/*`。
4. 远端 Yggdrasil 导入统一迁移到 `/v1/imports/remote-ygg/*`。
5. 管理员通知的 `PATCH` 保留为 v1 端点，但语义固定为替换发布。
6. OAuth 标准端点不放入 `/v1`。
7. `/v1/permissions/catalog` 作为未来 OAuth scope 发现基础。
8. 旧站点 API 不长期保留；Yggdrasil 与 Mojang 兼容端点不在本次迁移范围内。
