# Element Skin v3.0.1

> **v3.0.1 是 v3.0.0 的稳定性修复版本。**
>
> 本版本没有数据库 schema、迁移流程或配置项变更，可以从 v3.0.0 直接升级。

---

# 修复

* 修复首页全景图 cubemap 面映射错误造成的镜像与方向异常。该修复由 [PR #10](https://github.com/water2004/element-skin/pull/10) 合并。
* 修复 fallback 官方白名单响应未正确解包，导致白名单列表无法显示的问题。
* 修复“不限次数”邀请码被错误创建为一次性邀请码的问题。
* 修复 Python SDK 分页参数名称与后端不一致，导致分页大小设置无效的问题。
* 补全 Microsoft 与远程 Yggdrasil 导入接口的前端响应类型。
* 修正管理员材质删除接口的前端响应类型，使其与既有的 `{"success":true}` API 契约一致。

---

# OAuth

* 授权码现在与发起授权时的 `redirect_uri` 严格绑定，防止授权码被用于其他回调地址。
* Authorization Code 换取 token 时必须提交相同的 `redirect_uri`；错误请求不会消耗有效授权码。
* Python SDK 已同步新契约，并修正授权流程文档与请求参数。

---

# 测试

后端、前端和 Python SDK 测试均已通过；Python SDK 保持 100% 行覆盖率与分支覆盖率。

---

# 升级说明

从 v3.0.0 升级时不需要执行额外数据库迁移，也不需要修改配置。使用 Authorization Code Flow 的第三方应用需要升级 Python SDK，或在 token 请求中补充与授权请求完全一致的 `redirect_uri`。

---

**Full Changelog**: https://github.com/water2004/element-skin/compare/v3.0.0...v3.0.1
