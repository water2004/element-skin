# element-skin

一个基于Vue 3 + Element Plus的Minecraft皮肤站前端项目，支持Yggdrasil协议和微软正版登录。

## 主要特性

### 用户功能
- ✅ **用户注册/登录** - 支持邮箱密码注册和邀请码系统
- ✅ **角色管理** - 创建、删除和管理多个游戏角色
- ✅ **材质库** - 上传、管理皮肤和披风
- ✅ **3D预览** - 实时预览皮肤和披风效果
- ✅ **微软正版登录** - 通过微软账户导入正版角色、皮肤和披风
- ✅ **个人资料** - 修改密码、显示名称

### 管理员功能
- ✅ **站点设置** - 配置站点名称、注册开关、邀请码设置
- ✅ **用户管理** - 查看、封禁、解封用户
- ✅ **邀请码管理** - 创建单次或多次使用的邀请码
- ✅ **微软OAuth配置** - 配置Azure应用的Client ID
- ✅ **用户详情** - 查看用户的角色、材质、封禁状态

### 技术特性
- 🎨 **Element Plus** - 优雅的UI组件库
- 🚀 **Vue 3** - 使用Composition API
- 📱 **响应式设计** - 适配各种屏幕尺寸
- 🎭 **流畅动画** - 使用CSS3过渡和动画
- 🔐 **JWT认证** - 安全的用户认证系统
- 📦 **Yggdrasil协议** - 兼容Minecraft客户端验证

## 技术栈

- **Vue 3** - 渐进式JavaScript框架
- **TypeScript** - 类型安全
- **Element Plus** - Vue 3 UI组件库
- **Vite** - 下一代前端构建工具
- **Vue Router** - 官方路由管理
- **Axios** - HTTP客户端
- **Pinia** - 状态管理（可选）

## Recommended IDE Setup

[VS Code](https://code.visualstudio.com/) + [Vue (Official)](https://marketplace.visualstudio.com/items?itemName=Vue.volar) (and disable Vetur).

## Recommended Browser Setup

- Chromium-based browsers (Chrome, Edge, Brave, etc.):
  - [Vue.js devtools](https://chromewebstore.google.com/detail/vuejs-devtools/nhdogjmejiglipccpnnnanhbledajbpd) 
  - [Turn on Custom Object Formatter in Chrome DevTools](http://bit.ly/object-formatters)
- Firefox:
  - [Vue.js devtools](https://addons.mozilla.org/en-US/firefox/addon/vue-js-devtools/)
  - [Turn on Custom Object Formatter in Firefox DevTools](https://fxdx.dev/firefox-devtools-custom-object-formatters/)

## Type Support for `.vue` Imports in TS

TypeScript cannot handle type information for `.vue` imports by default, so we replace the `tsc` CLI with `vue-tsc` for type checking. In editors, we need [Volar](https://marketplace.visualstudio.com/items?itemName=Vue.volar) to make the TypeScript language service aware of `.vue` types.

## Customize configuration

See [Vite Configuration Reference](https://vite.dev/config/).

## Project Setup

```sh
npm install
```

### Compile and Hot-Reload for Development

```sh
npm run dev
```

### 调试节日彩蛋

开发环境下会在浏览器控制台暴露 `window.elementSkinEasterEggs`，可以手动启动、停止或按日期刷新彩蛋：

```js
await elementSkinEasterEggs.start('spring-festival')
elementSkinEasterEggs.stop()
await elementSkinEasterEggs.refreshAt('2026-02-17')
elementSkinEasterEggs.setDisabled(false)
elementSkinEasterEggs.list()
```

当前可用 ID：

- `spring-festival`：春节，农历正月初一
- `april-fools`：愚人节，4 月 1 日
- `qingming`：清明，4 月 4 日至 4 月 5 日
- `children-day`：儿童节，6 月 1 日
- `dragon-boat`：端午节，农历五月初五
- `christmas`：圣诞节，12 月 24 日至 12 月 25 日

农历彩蛋使用浏览器内置 Chinese calendar 判断日期；如果手动测试发现没有效果，先确认服务端公开设置允许该彩蛋，且个人资料里的“关闭彩蛋”没有开启。

### Type-Check, Compile and Minify for Production

```sh
npm run build
```

### Lint with [ESLint](https://eslint.org/)

```sh
npm run lint
```
