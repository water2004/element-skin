# Element Skin 编码规范

本文记录 Element Skin 当前代码库已经形成的编码约定。新增功能应优先贴合这些约定，除非有明确的架构原因需要调整。

## 1. 通用原则

- 保持改动范围清晰。新增功能应放在对应业务模块内，不把临时逻辑散落到全局工具或页面中。
- 优先复用现有抽象。前端优先使用已有 UI 组件、API 层、存储层；后端优先使用已有 `httpapi`、`service`、`database` 分层。
- 数据和程序分离。测试用例数据、fixture、业务常量应放在可维护的位置，不把大量测试数据直接堆在测试流程里。
- 错误路径也要验证。涉及数据库、文件、缓存、权限、token 的改动，需要覆盖失败后不污染状态的场景。
- 默认使用 UTF-8。代码、文档和用户可见文本保持当前项目已有语言风格。

## 2. 前端规范

### 2.1 技术栈与格式化

- 前端使用 Vue 3、TypeScript、Vite、Element Plus、Tailwind CSS。
- Vue 文件默认使用 `<script setup lang="ts">`。
- 格式化遵循现有 Prettier 配置：
  - 不使用分号。
  - 使用单引号。
  - `printWidth` 为 100。
- 新增代码应通过 `npm run build`，涉及测试时运行 `npm run test`。

### 2.2 目录与职责

- `src/api/*` 负责后端 API 封装，统一使用 `src/api/client.ts`。
- 组件和页面不要直接创建 axios 实例，也不要绕过 `apiClient`。
- `src/storage/*` 负责 localStorage、IndexedDB、渲染缓存等浏览器存储抽象。
- 组件和页面不要直接访问 `localStorage`、`sessionStorage`、`indexedDB`，应通过存储模块。
- `src/components/ui/*` 放通用 UI 封装。
- `src/components/common/*` 放通用布局和交互组件。
- `src/components/dashboard/*`、`src/components/admin/*` 放业务页面组件。
- 页面级组件负责数据加载、路由跳转、消息提示和业务流程。
- 子组件通过 props 接收数据，通过 emit 暴露用户意图，不直接调用业务 API。

### 2.3 UI 与样式

- 优先使用 Tailwind class 完成布局、间距、对齐、响应式和常见视觉样式。
- 尽量避免手写 CSS 和 inline style。
- 只有在 Tailwind 难以表达、需要覆盖 Element Plus 内部结构、需要复杂动画或组件级稳定尺寸时，才编写 scoped CSS。
- 需要手写 CSS 时，优先使用组件内有语义的 class，避免大范围全局选择器。
- 全局 Element Plus 覆盖放在 `src/assets/styles/element-overrides.css`。
- 页面视觉应复用已有 CSS 变量，例如 `--color-heading`、`--color-text-light`、`--color-card-background`、`--color-border`。
- 优先使用已有封装组件：
  - `UiCard` 替代直接裸用 `el-card`。
  - `UiButton`、`ActionBar`、`CardActions`、`SearchBar` 等替代重复手写按钮区和工具栏。
- 图标优先使用 Element Plus Icons 或项目已有图标体系，不手写重复 SVG。
- 仪表盘、管理后台等操作型界面应保持克制、清晰、可扫描，不做营销页式大面积装饰。

### 2.4 首页特殊约束

- 首页中心按钮和玻璃半透明按钮必须保持 fixed 布局，并处于顶层覆盖结构中。
- 首页玻璃半透明按钮不能被包进会破坏 backdrop blur 的普通布局容器。
- 首页按钮、标题、footer 的纵向位置应基于顶栏下边缘与 fixed footer 上边缘之间的可用区域计算。
- 调整首页布局时必须同时检查登录态和未登录态。
- 调整首页模糊、玻璃效果、fixed 层级时，应在桌面端和移动端做视觉验证。

### 2.5 API 与类型

- API wrapper 的函数名使用动词开头，例如 `getTextures`、`patchTexture`、`deleteTexture`。
- API 返回类型应在函数签名或 `src/api/types.ts` 中明确表达。
- 后端字段保持 snake_case，前端类型中也保留接口字段的 snake_case，避免在 API 层做隐式重命名。
- 分页接口优先复用 `CursorPageResponse<T>`。
- 401 刷新逻辑集中在 `api/client.ts`，业务组件不要自行实现刷新 token。

### 2.6 前端测试

- API wrapper 测试应保持数据和程序分离，fixture 放在 `src/api/__tests__/fixtures/*`。
- 新增 API wrapper 应补充 exact request 测试，验证 method、path、params、body。
- 存储模块测试放在对应模块的 `__tests__` 目录下。
- 复杂 UI 改动至少通过构建验证；涉及缓存、存储、权限或关键交互时补测试。

## 3. 后端规范

### 3.1 技术栈与格式化

- 后端使用 Go 1.26。
- Go 代码必须通过 `gofmt`。
- 新增后端功能应通过 `go test ./...`，除非明确说明测试环境不可用。
- 不引入重量级框架，优先使用当前 `net/http`、`pgx`、Redis store 模式。

### 3.2 分层

- 后端必须遵循严格分层架构：路由层、业务层、数据库层不得互相越权。
- `internal/httpapi/*` 负责 HTTP 请求解析、认证包装、状态码和 JSON 响应。
- `internal/service/*` 负责业务规则、权限判断、跨 store 协调。
- `internal/database/<domain>/*` 负责 SQL 和持久化细节。
- `internal/model` 放跨层共享的核心数据结构。
- `internal/util` 放无业务归属的通用工具。
- `internal/redisstore` 放 Redis 与内存 store 的统一接口和实现。
- 路由层不得承载业务规则。Handler 只做参数读取、基础格式校验、调用 service、返回响应。
- 路由层原则上不直接编排数据库写入；需要读写多个 store、判断权限或处理状态转换时必须进入 service。
- 业务层不得关心 HTTP 表达，不直接写 `http.ResponseWriter`，不构造路由响应。
- 数据库层只负责 SQL、事务和数据映射，不做权限判断，不返回 HTTP 语义错误。
- database store 不应返回 `util.HTTPError`，HTTP 状态应由 service 或 handler 转换。

### 3.3 HTTP API

- 路由集中在 `internal/httpapi/routes.go` 注册。
- 认证路由使用现有 Auth 包装；新增权限模型时应先抽象统一 principal，再接入路由。
- JSON 请求解析使用 `shared.DecodeJSON`。
- Multipart 文件读取使用 `shared.MultipartFileBytes` 或同类共享 helper。
- 成功响应使用 `util.JSON`。
- 普通错误使用 `util.HTTPError{Status, Detail}`，输出格式为 `{"detail":"..."}`。
- Yggdrasil 协议错误保持现有 `YggError` 格式，不和站点 API 混用。
- 未找到、无权限、非法输入应返回明确的 HTTP 状态，不用 500 掩盖业务错误。

### 3.4 数据库

- 数据库初始化 SQL 维护在 `internal/database/schema.go`。
- 新业务建议新增 `internal/database/<domain>` store，并挂到 `database.DB`。
- Store 方法第一个参数为 `context.Context`。
- PostgreSQL 访问使用 `pgx` / `pgxpool`。
- 事务使用 `Begin`、`defer tx.Rollback(ctx)`、成功后 `tx.Commit(ctx)` 的模式。
- 分页优先使用 cursor pagination，返回 `items`、`has_next`、`next_cursor`、`page_size`。
- 时间字段统一使用毫秒时间戳，优先通过 `database.NowMS()` 获取。
- 删除行为按业务定义决定。允许真删除时，应明确级联或清理策略。

### 3.5 Redis 与缓存

- Redis 访问必须通过 `redisstore.Store` 接口。
- 新 Redis 功能需要同时实现真实 Redis store 和内存 store。
- 测试优先使用内存 Redis；集成测试可使用带独立 prefix 的真实 Redis。
- 缓存失效必须作为业务流程的一部分处理，尤其是用户权限、公开配置、首页媒体等。

## 4. 测试规范

- 所有测试必须断言精确预期结果，不允许只做粗略验证。
- 禁止只检查“没有报错”“返回非空”“长度大于 0”这类弱断言，除非该断言就是业务精确定义。
- 测试应明确验证关键字段、状态码、响应体、数据库状态、缓存状态、文件状态或副作用。
- 对错误路径的测试必须断言精确错误类型、HTTP 状态和响应内容。

### 4.1 后端测试

- 数据库层测试放在对应 `internal/database/<domain>` 包内。
- Service 测试放在对应 `internal/service/<domain>` 包内。
- HTTP 路由测试放在对应 `internal/httpapi/<domain>` 包内。
- 跨模块真实 HTTP 流程放在 `internal/integration`。
- 测试 helper 优先复用 `internal/testutil`。
- 测试名应描述行为，例如 `TestTextureRoutesRejectInvalidInputsWithExactErrors`。
- 重要响应应断言状态码和响应体。
- 数据库测试应断言具体行内容、计数、排序、分页游标和事务结果。
- 失败路径应验证不会留下错误数据库行、文件、Redis key 或权限状态。
- 涉及并发、轮换、缓存一致性的逻辑需要覆盖边界条件。

### 4.2 前端测试

- API wrapper 用 fixture 分组，不把所有 case 堆在一个测试函数里。
- API wrapper 测试必须断言精确 method、path、params、body 和调用次数。
- 存储和缓存逻辑应覆盖 quota、不可用、过期、LRU、清理等边界。
- UI 或 composable 测试应断言具体状态、DOM 文本、事件参数和副作用。
- UI 测试或人工验证应覆盖桌面端和移动端关键布局。
- 首页 fixed、backdrop blur、footer、登录态切换属于高风险视觉点，改动后必须验证。

## 5. 文档规范

- 长期设计文档放在 `doc/`。
- 测试报告和压测报告放在 `reports/`。
- 文档标题用中文，文件名应能表达主题。
- 方案文档应包含背景、目标、非目标、数据模型、API、前端交互、测试计划和待确认问题。
- 代码规范更新时，`doc/编码规范.md` 与根目录 `agents.md` 应保持一致。

## 6. 提交规范

- Commit message 使用英文。
- 标题使用小写前缀，格式为 `<type>: <summary>`。
- 允许的 type：
  - `feat:` 新增用户可见功能。
  - `add:` 新增文件、文档、测试、资源或内部能力；也可用于不适合归入 `feat` 的新增内容。
  - `fix:` 修复缺陷。
  - `refactor:` 重构，不改变外部行为。
  - `perf:` 性能优化。
  - `docs:` 文档变更。
  - `test:` 测试新增或调整。
  - `style:` 代码风格、格式、样式调整，不改变逻辑。
  - `build:` 构建系统、依赖、打包配置变更。
  - `ci:` CI/CD 配置和脚本变更。
  - `chore:` 维护性杂项、配置清理、工具调整等非业务改动。
  - `revert:` 回滚先前提交。
- Summary 使用简短英文祈使句或名词短语，例如：
  - `feat: add developer application dashboard`
  - `add: notice system design`
  - `fix: avoid auth flicker on home page`
  - `docs: update coding standards`
  - `test: cover notice expiration cleanup`
  - `chore: update frontend dependencies`
  - `refactor: split dashboard notice panel`
- 一次提交只表达一个主要意图。
- 不把无关格式化、调试输出、临时文件混入功能提交。

## 7. 禁忌清单

- 不在组件里直接写 axios 请求。
- 不在组件里直接读写 localStorage、sessionStorage 或 IndexedDB。
- 不在业务页面重复实现已有 UI 组件能承担的结构。
- 不在测试函数里堆大量未组织的测试数据。
- 不为第三方 OAuth 复用站点 cookie JWT 或 Yggdrasil token。
- 不在后端 handler 中堆积复杂业务规则。
- 不把数据库错误原样暴露给用户。
- 不在首页玻璃按钮外层加入会破坏 fixed 顶层和 backdrop blur 的布局容器。
