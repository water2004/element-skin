# Element Skin v3.1.0

> **v3.1.0 是相对 v3.0.2 的不兼容功能更新。**
>
> 本版本新增统一的外部身份管理、Microsoft 正版角色绑定与同步、标准 OpenID Connect Provider、
> OAuth Webhook，以及邮箱后缀策略；同时将站点 JSON API 统一升级为 `/v2`。

---

## 升级前必读

* 站点 JSON API 已统一迁移至 `/v2`，旧版路径不再保留。自行开发的前端、脚本和第三方集成必须同步更新。
* Docker Compose 用户需要同步新版 `.env.example` 和 `docker-compose.yml`，并配置
  `OIDC_PRIVATE_KEY`、`OIDC_PUBLIC_KEY` 与 `IDENTITY_ENCRYPTION_KEY`。OIDC 密钥文件不存在时会自动
  生成；`IDENTITY_ENCRYPTION_KEY` 可使用 `openssl rand -base64 32` 生成，投入使用后不得更换。
* 使用 Microsoft 正版账号功能的站点，需要在 Azure 应用中加入新的 Web 回调地址：
  `${SERVER_API_URL}/v2/auth/oidc/callback`。
* 原有 Microsoft 应用配置会在升级时转为 Microsoft OIDC 身份提供方。已导入的角色和材质保持不变，
  但用户需要重新授权 Microsoft 身份，之后才能绑定和同步正版角色。
* 邀请码管理接口现在使用无填充 Base64URL 传输邀请码；调用管理接口的自定义客户端需要同步适配。

---

## 外部身份与正版角色

* 新增统一的身份管理页。用户可以绑定多个 OIDC 身份，也可以在同一个身份提供方下添加多个账号，
  并查看、重新授权或移除各个身份。
* 管理员可以配置多个外部 OIDC 身份提供方，并分别控制其是否允许登录本站或绑定到已有账户。
* 第三方登录不会省略本站注册要求：没有匹配账户时，用户仍需填写用户名、本站邮箱和密码，并完成
  当前站点启用的邮箱验证及邀请码校验。
* Microsoft 接入已纳入统一的 OIDC 身份流程。临时访问令牌过期不会要求用户重复登录；只有长期授权
  已失效时，身份页才会明确提示重新授权。
* “绑定正版账户”改为从已经绑定的 Microsoft 身份中选择正版角色。用户可以预览、绑定并手动同步远端
  角色资料，身份页也会显示每个 Microsoft 身份关联的正版角色。
* 正版绑定与本站角色保持独立：角色仍可单独编辑或删除，绑定和同步不会把两类资源合并为同一个对象。

---

## OpenID Connect 与第三方应用

* 本站现在可作为标准 OpenID Provider，提供 Discovery、JWKS、ID Token 和 UserInfo，并支持
  `openid`、`profile`、`email`、`offline_access` 标准 scope。
* OIDC scope 与站点 `/v2` 权限相互独立。仅使用“通过本站登录”的应用可以不申请站点业务权限；需要
  读取或修改站点资源时，再按实际用途申请对应的细粒度权限。
* OAuth 应用创建与编辑改为独立页面，补充应用类型、回调地址、权限申请、审核状态和 OIDC 接入指引。
  管理后台会直接展示本站的 OIDC 发现地址和签发方信息。
* OAuth 应用可以订阅其权限范围内的 Webhook 事件。站点提供独立签名密钥、HMAC 验签信息和失败重试，
  可用于跟踪账户、授权、角色、材质、权限及正版白名单等资源变化。
* 修正第三方应用审核、通过、驳回、停用和授权管理中的权限判断；具有对应细粒度权限的管理员现在可以
  正常看到入口并执行操作。

---

## 注册与邮箱策略

* 邮件设置新增邮箱后缀策略，可选择关闭、白名单或黑名单模式。当前规则会公开给注册页，用于提前展示
  可选后缀或提示邮箱不可用；后端仍会执行最终校验。
* 公共站点设置会明确返回是否需要邀请码。需要邀请码时，注册页会显示必填状态和对应输入提示。
* 管理员现在可以创建、查看和删除包含引号、斜杠、反斜杠及其他任意 UTF-8 字符的邀请码。

---

## API、SDK 与文档

* `/v2` 普通 API 使用统一的成功状态、分页结构和错误对象。错误响应现在通过稳定的
  `object`、`operation`、`reason` 与受控参数表达，不再依赖不一致的展示文案。
* OAuth/OpenID Connect 与 Yggdrasil 继续遵循各自的标准协议响应，不与普通站点 API 的错误格式混用。
* Python SDK 已适配 `/v2`、结构化错误、OIDC scope、新版邀请码接口和 Webhook 验签，并补充 OAuth、
  OpenID Connect、Webhook 及管理员调用示例。
* 开发者文档已更新第三方登录、权限申请、Token 生命周期、Webhook 接收和新版 API 的接入说明。

---

**Full Changelog**: https://github.com/water2004/element-skin/compare/v3.0.2...v3.1.0
