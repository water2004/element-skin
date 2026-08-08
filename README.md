<p align="center">
  <img src="./img/readme-header.svg" width="100%" alt="Element-Skin Header">
</p>

<p align="center">
  面向高并发场景的现代化外置登录与材质平台
</p>

<p align="center">
  <a href="https://deepwiki.com/water2004/element-skin">
    <img src="https://deepwiki.com/badge.svg">
  </a>
  <a href="LICENSE">
    <img src="https://img.shields.io/github/license/water2004/element-skin">
  </a>
  <img src="https://img.shields.io/badge/Vue-3-4FC08D?logo=vue.js&logoColor=white">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white">
  <img src="https://img.shields.io/badge/PostgreSQL-4169E1?logo=postgresql&logoColor=white">
  <img src="https://img.shields.io/badge/Redis-required-DC382D?logo=redis&logoColor=white">
</p>

![](./img/root.png)

## ✨ 功能特性

- **现代化架构**：Go 1.26 后端，使用 PostgreSQL 18 保存结构化业务数据，使用 Redis 处理缓存、限流和短期状态。
- **Yggdrasil 兼容**：支持 Authlib-Injector、启动器和 Minecraft 服务器所需的认证、会话、角色查询、材质签名与服务器加入接口。
- **Fallback 服务**：支持多个外部皮肤站作为 fallback，提供健康检查、角色与材质导入、公钥发现和缓存。
- **用户与资源管理**：支持邮箱验证、密码找回、角色管理、皮肤与披风上传、衣柜、3D 预览和公共皮肤库。
- **外部身份与角色同步**：支持多 OIDC 身份、Microsoft 正版角色绑定与显式同步，以及远程 Yggdrasil 角色导入。
- **细粒度权限**：支持多角色、单项权限覆盖、权限范围、受保护权限主体和按权限展示页面与操作。
- **OAuth 第三方应用**：支持公开应用、机密应用、Authorization Code + PKCE、Device Code、Client Credentials、Refresh Token、撤销、权限审核和按权限订阅的异步 Webhook。
- **通知中心**：统一展示公告、系统消息和 OAuth 事件，支持 Markdown 长公告、短公告、定向投递、未读提醒和过期清理。
- **Minecraft 能力 API**：通过 `/v2/minecraft` 提供公开角色查询、材质属性读取和服务器加入结果校验，不替代 Yggdrasil 协议。
- **首页与仪表盘**：支持普通背景图、Minecraft 全景背景、首页媒体管理、服务状态监测、公告侧栏和节日彩蛋。
- **现代化前端**：Vue 3、Element Plus、Tailwind CSS，支持响应式布局、深色模式、按权限显示导航和移动端访问。
- **浏览器缓存**：统一封装 localStorage 与 IndexedDB，支持材质文件、角色卡片和材质卡片渲染结果的 LRU 缓存与大小限制。
- **Python SDK**：提供 OAuth 流程、权限模型、token 管理和 `/v2` API 调用封装，并附带开发示例与中文文档。
- **安全与部署**：提供 API 限流、CORS、出站请求校验、结构化错误处理和单一 Docker Compose 部署方式。

---

## 🚀 Docker 部署指南 (推荐)

项目现在默认使用 **PostgreSQL 18 + Redis** 并支持自动化初始化。PostgreSQL 保存用户、设置、材质元数据等持久化数据；Redis 负责公开配置/首页媒体缓存、邮件验证码、限流数据和短期用户鉴权缓存等临时状态。

### 1. 准备 `.env`

Docker 部署只使用仓库根目录的 `docker-compose.yml`。复制 `.env.example` 为 `.env`，然后只修改 `.env`：

```bash
cp .env.example .env
```

必须重点修改这些值：

- `ELEMENT_SKIN_IMAGE`：后端镜像，默认 `ghcr.io/water2004/element-skin:latest`
- `JWT_SECRET`：生产环境随机长密钥
- `IDENTITY_ENCRYPTION_KEY`：使用 `openssl rand -base64 32` 生成一次并长期保存
- `DATABASE_PASSWORD` / `REDIS_PASSWORD`：数据库和 Redis 密码
- `SERVER_SITE_URL`：站点外部访问地址
- `SERVER_API_URL`：后端 API 外部访问地址
- `CORS_ALLOW_ORIGINS`：允许访问 API 的前端来源

后端启动时会读取环境变量，并从 `DATABASE_HOST/PORT/USER/PASSWORD/NAME/SSLMODE` 和 `REDIS_HOST/PORT` 派生连接地址。Docker 部署不需要挂载 `config.yaml`，也不维护第二份 Compose 配置。

Compose 会从同一镜像启动 `backend` 和独立的 `webhook-worker`。主站请求只在业务事务内写入轻量
outbox；签名、第三方 HTTP 请求、退避重试和历史清理由 worker 完成。worker 不暴露端口，并使用
独立且最多 5 个连接的数据库池，慢或故障的第三方 endpoint 不占用主站请求线程。

首次启动时如果 Yggdrasil 的 `/app/data/private.pem`、`/app/data/public.pem` 或 OIDC 的
`/app/data/oidc-private.pem`、`/app/data/oidc-public.pem` 不存在，系统会自动生成并保存。请持久化
`./data` 目录，其中 `./data/db` 会挂载到 PostgreSQL 容器的 `/var/lib/postgresql`。后续不要删除或
替换这些私钥。`IDENTITY_ENCRYPTION_KEY` 用于加密 OIDC client secret 和外部 refresh token，配置
身份提供方后不得重新生成，否则已有密文将无法解密。

从 v3.0.x 升级时，后端会把已配置的 `microsoft_client_id` 和 `microsoft_client_secret` 一次性迁移为
只开放绑定能力的 Microsoft OIDC provider。只有 provider 创建成功后才删除旧设置；失败时保留旧值
并终止启动。旧版导入的角色和材质保持不变，但旧流程没有持久化 Microsoft 用户 refresh token，
因此用户仍需重新授权一次。升级前还需要在 Azure 应用中加入新的 Web 回调地址：

```text
${SERVER_API_URL}/v2/auth/oidc/callback
```

**Nginx 主机配置**
只需将 Nginx 的 `root` 指向宿主机的 `./frontend` 目录。

```nginx
server {
    listen 80;
    server_name yourdomain.com;

    # 1. 前端根目录 (index.html, assets, 以及皮肤 static/)
    root /your/path/to/frontend; 
    index index.html;

    location / {
        add_header X-Authlib-Injector-API-Location "http://yourdomain.com/skinapi" always;
        try_files $uri $uri/ /index.html;
    }

    # 2. 后端 API 转发
    location /skinapi/ {
        proxy_pass http://localhost:8000/;
        proxy_set_header Host $host;
    }
    
    # 直接转发不带斜杠的 API 请求
    location = /skinapi {
        return 308 /skinapi/;
    }
}
```
### 2. 启动服务

拉取镜像并启动：

```bash
docker compose pull
docker compose up -d
```

对于希望前端或后端地址部署在子目录的用户，可以通过 `.env` 灵活配置路径：
- **前端路径**: 通过 `VITE_BASE_PATH` 定义前端资源的基础路径
- **后端路径**: 通过 `VITE_API_BASE` 定义后端 API 的基础路径

根据你的路径需求修改 `.env`，然后重新执行 `docker compose up -d`。前端会根据这些参数在容器启动时替换路径，并自动释放到宿主机的 `./frontend` 目录：

| 场景 | `VITE_BASE_PATH` | `VITE_API_BASE` |
|-----|---------|---------|
| **场景 1** | `/skin/` | `/skinapi` |
| **场景 2** | `/skin/` | `/skin/api/` |

需要注意的是，`.env` 中的 `SERVER_SITE_URL` 和 `SERVER_API_URL` 也需要根据实际部署路径进行调整，以确保生成的链接正确。
当 `VITE_API_BASE` 使用 `/skinapi`、`/skin/api` 这类前缀时，Nginx 的 `proxy_pass` 末尾必须带 `/`，这样会把前缀剥掉再转发给后端。例如 `/skinapi/v2/users/me` 会转成后端实际路由 `/v2/users/me`。

**Nginx 主机配置 (对应场景 1)**
```nginx
# 1. 前端静态文件
location /skin/ {
    add_header X-Authlib-Injector-API-Location "http://yourdomain.com/skinapi" always;
    alias /your/path/to/frontend/;
    index index.html;
    try_files $uri $uri/ /skin/index.html;
}
location = /skin {
    alias /your/path/to/frontend/;
    try_files $uri $uri/ /skin/index.html;
}

# 2. 后端 API 转发
location /skinapi/ {
    proxy_pass http://localhost:8000/;
    proxy_set_header Host $host;
}
location = /skinapi {
    return 308 /skinapi/;
}
```

**Nginx 主机配置 (对应场景 2)**
```nginx
# 1. 前端静态文件
location /skin/ {
    add_header X-Authlib-Injector-API-Location "http://yourdomain.com/skin/api" always;
    alias /your/path/to/frontend/;
    index index.html;
    try_files $uri $uri/ /skin/index.html;
}
location = /skin {
    alias /your/path/to/frontend/;
    try_files $uri $uri/ /skin/index.html;
}

# 2. 后端 API 转发 (嵌套路径)
location /skin/api/ {
    proxy_pass http://localhost:8000/;
    proxy_set_header Host $host;
}
location = /skin/api {
    return 308 /skin/api/;
}
```
---

## 🛠️ 本地开发环境

### 本地开发环境

#### 1. 数据库配置 (PostgreSQL 18+)
本地开发需要手动安装并初始化数据库：

1.  **安装 PostgreSQL**: 确保本地已安装 PostgreSQL 18（或 16+）。
2.  **创建数据库**: 使用 `psql` 或 GUI 工具（如 pgAdmin/DBeaver）创建用户和数据库：
    ```sql
    -- 建议创建专用用户和库
    CREATE USER elementskin WITH PASSWORD 'password123';
    CREATE DATABASE elementskin OWNER elementskin;
    ```
3.  **修改配置**: 编辑 `skin-backend/config.yaml` 中的数据库字段：
    ```yaml
    database:
      host: "localhost"
      port: "5432"
      user: "elementskin"
      password: "password123"
      name: "elementskin"
      sslmode: "disable"
    ```
    > 💡 **自动初始化**: 后端在每次启动时会自动同步数据库结构（创建缺失的表及默认配置），无需手动执行 SQL 脚本。

#### 2. Redis 配置
本地开发需要 Redis 运行在 `127.0.0.1:6379`。如果你的 Redis 设置了密码，请同步修改 `skin-backend/config.yaml`：

```yaml
redis:
  host: "127.0.0.1"
  port: "6379"
  password: ""
  db: 0
  key_prefix: "elementskin:"
```

#### 3. 后端 (Go 1.26+)
```bash
cd skin-backend
go run ./cmd/element-skin
```

#### 4. 前端 (Node.js)
```bash
cd element-skin
npm install
npm run dev
```

---

## 📂 项目结构

```text
element-skin/
├── element-skin/       # 前端源码 (Vue 3 + Element Plus)
├── skin-backend/       # Go 后端源码
│   ├── cmd/            # 进程入口
│   ├── internal/       # HTTP、服务、数据库与测试模块
│   └── config.yaml     # 后端配置文件
├── .env.example        # Docker 部署环境变量模板
├── data/               # Docker 持久化数据 (自动生成)
├── frontend/           # Docker 释放的前端静态文件 (自动生成)
├── docker-compose.yml  
└── README.md
```

---

## 🧪 自动化测试

Go 后端采用分层测试架构，确保从底层数据库到顶层 API 的稳定性。

### 测试架构
1.  **数据库层 (`internal/database`)**: 验证 SQL 逻辑、数据迁移及缓存一致性。
2.  **业务逻辑层 (`internal/service`)**: 验证核心业务规则（如注册权限、材质级联更新）。
3.  **HTTP 集成层 (`internal/integration`)**: 使用真实 PostgreSQL 和真实 Redis，模拟真实 HTTP 请求。

### 运行测试
测试会自动创建临时数据库和文件目录，不会影响本地开发数据。

```bash
cd skin-backend
go test ./...
```

### 编写新测试
单元测试使用内存 Redis mock；`internal/integration` 使用真实 Redis，并通过唯一 key 前缀自动清理测试数据，不会清空你的本地 Redis。

## 📈 并发压测结果

最新一次 v3.0.0 压测在本机通过 `skin-backend/cmd/loadtest` 启动隔离测试数据库、真实 Redis key 前缀和进程内 HTTP 服务完成，不会触碰正常运行数据库。命令如下：

```bash
cd skin-backend
LOADTEST_ENABLE=1 LOADTEST_CONCURRENCY=200 LOADTEST_DURATION=1s go test ./cmd/loadtest -run TestRealBackendLoad -count=1 -v
```

测试数据：100 个用户、300 个角色、500 条材质记录、50 个邀请码、1 个预置 Yggdrasil join 会话。固定并发：200；每个场景窗口：1s；数据库连接池：20。当前报告包含公开端点、Cookie、OAuth delegated、Client Credentials、管理员和 Yggdrasil 场景，全部 0 失败。

### v3.0.0 与 v2.4.1 功能对照

下表比较的是同一业务功能在两个版本中的实际接口，不是 Go 与 Python 实现语言对比。v2.4.1 基准取自 Python 2.4.1 压测，v3.0.0 基准取自本次 Go 压测；v2.4.1 使用旧站点路径，v3.0.0 使用 `/v2` 路径，Yggdrasil 协议路径保持不变。

| 功能（v2.4.1 → v3.0.0） | v3.0.0 req/s | v2.4.1 req/s | 变化 | v3.0.0 P95 | v2.4.1 P95 |
| --- | ---: | ---: | ---: | ---: | ---: |
| 公开设置（`/public/settings` → `/v2/public/settings`） | 26733.6 | 1913.7 | 14.0x | 8.4ms | 200.3ms |
| 首页媒体（`/public/homepage-media` → `/v2/public/homepage-media`） | 32634.7 | 2138.0 | 15.3x | 7.8ms | 113.4ms |
| 公开皮肤库（`/public/skin-library` → `/v2/public/skin-library`） | 18196.2 | 777.9 | 23.4x | 16.1ms | 552.6ms |
| 登录（`/site-login` → `/v2/auth/login`） | 311.7 | 42.1 | 7.4x | 890.7ms | 4.58s |
| Yggdrasil 元数据（`/` → `/`） | 26109.0 | 2694.4 | 9.7x | 10.2ms | 110.9ms |
| Yggdrasil authenticate | 289.6 | 42.6 | 6.8x | 1.11s | 4.54s |
| Yggdrasil validate | 16188.3 | 1126.3 | 14.4x | 13.9ms | 422.1ms |
| Yggdrasil profile | 70172.7 | 1782.7 | 39.4x | 4.6ms | 151.1ms |
| Yggdrasil 按名称查询 | 75233.8 | 1827.5 | 41.2x | 4.2ms | 164.2ms |
| Yggdrasil hasJoined | 1976.9 | 250.8 | 7.9x | 158.5ms | 1.36s |
| 当前用户（`/me` → `/v2/users/me`） | 12896.7 | 984.3 | 13.1x | 18.7ms | 384.1ms |
| 我的角色（`/me/profiles` → `/v2/users/me/profiles`） | 17094.2 | 891.2 | 19.2x | 13.4ms | 469.3ms |
| 我的材质（`/me/textures` → `/v2/users/me/textures`） | 17070.5 | 1125.8 | 15.2x | 13.6ms | 361.6ms |
| 材质详情（`/me/textures/{hash}/skin` → `/v2/users/me/textures/{hash}/skin`） | 16641.1 | 1101.1 | 15.1x | 13.8ms | 360.5ms |
| 管理员用户列表（`/admin/users` → `/v2/admin/users`） | 1879.5 | 672.9 | 2.8x | 124.8ms | 780.4ms |
| 管理员用户详情（`/admin/users/{id}` → `/v2/admin/users/{id}`） | 12154.6 | 822.2 | 14.8x | 19.4ms | 510.3ms |
| 管理员用户角色列表（`/admin/users/{id}/profiles` → `/v2/admin/users/{id}/profiles`） | 16390.8 | 1032.5 | 15.9x | 13.8ms | 689.5ms |
| 管理员角色列表（`/admin/profiles` → `/v2/admin/profiles`） | 14260.2 | 809.2 | 17.6x | 17.0ms | 822.5ms |
| 管理员材质列表（`/admin/textures` → `/v2/admin/textures`） | 14997.6 | 793.0 | 18.9x | 17.0ms | 659.7ms |
| 管理员邀请码（`/admin/invites` → `/v2/admin/invites`） | 14821.4 | 915.9 | 16.2x | 16.0ms | 371.8ms |
| 管理员站点设置（`/admin/settings/site` → `/v2/admin/settings/site`） | 2607.6 | 1318.3 | 2.0x | 80.8ms | 890.1ms |

这组对比只能说明固定测试条件下的吞吐和延迟差异，不能把路径迁移本身当作性能原因。3.0.0 额外增加了细粒度权限、Redis 权限缓存和统一 Actor 处理，因此当前用户和管理员列表等复杂权限路径需要重点关注。

### v3.0.0 新增 OAuth 压测场景

v2.4.1 没有对应 OAuth 功能，因此以下场景不参与跨版本对比：

| 场景 | 接口 | 成功 req/s | P95 |
| --- | --- | ---: | ---: |
| OAuth delegated 当前用户 | `/v2/users/me` | 9588.1 | 28.1ms |
| OAuth delegated 角色列表 | `/v2/users/me/profiles` | 13213.5 | 18.9ms |
| OAuth delegated 材质列表 | `/v2/users/me/textures` | 11809.0 | 23.5ms |
| OAuth delegated 材质详情 | `/v2/users/me/textures/{hash}/skin` | 13124.8 | 19.1ms |
| OAuth delegated 管理员用户列表 | `/v2/admin/users` | 1712.3 | 136.3ms |
| OAuth delegated 管理员用户详情 | `/v2/admin/users/{id}` | 9374.1 | 29.7ms |
| OAuth delegated 管理员邀请码 | `/v2/admin/invites` | 11404.4 | 22.5ms |
| Client Credentials 管理员邀请码 | `/v2/admin/invites` | 6709.2 | 42.1ms |
| OAuth delegated 管理员设置 | `/v2/admin/settings/site` | 2525.0 | 83.0ms |

完整报告见 [`reports/concurrency-load-test.md`](reports/concurrency-load-test.md)。压测报告使用隔离 PostgreSQL 数据库和 Redis key 前缀，测试结束后自动清理测试数据。

### Webhook 性能影响

Webhook 压测以同一个 profile 更新接口为负载，使用 50 并发、每阶段 3 秒、四轮平衡轮换、20 个主站数据库连接和独立的 5 连接 Worker 池，对比关闭触发器、无订阅、仅写 outbox、worker 同时运行四种模式。相对变化先在每轮内与该轮基线配对，再取中位数；所有写请求均成功：

| 模式 | 中位成功 req/s | 相对同轮功能前基线中位数 | 中位 P95 |
| --- | ---: | ---: | ---: |
| 关闭触发器（功能前近似基线） | 15236.4 | 0.0% | 5.1ms |
| 启用触发器，无订阅 | 13902.2 | -7.4% | 5.5ms |
| 有订阅，仅写 outbox | 13085.6 | -16.4% | 5.8ms |
| 有订阅，worker 同时运行 | 11860.8 | -18.5% | 6.6ms |

在本机零延迟 `204` 接收端下，1000 个固定事件的紧循环 outbox 展开、HTTP 投递落库和组合吞吐分别为 1225.2、4678.3 和 970.9 events/s；包含当前 500ms 轮询、每批 200 个事件和 50 个投递限制的生产 Worker 持续端到端吞吐只有 104.7 events/s。Worker 同时运行相对同轮“仅写 outbox”再降低 6.5% 主站写吞吐，中位 P95 增加 0.8ms：异步拆分避免了第三方 HTTP 直接阻塞主站请求，但共享 PostgreSQL 的查询和 I/O 竞争仍存在。持续事件速率超过单 Worker 吞吐时会积压，应增加 Worker 实例或调整批次与调度策略；实际投递能力还需结合第三方网络延迟复测。完整方法、原始轮次和限制见 [`reports/webhook-load-test.md`](reports/webhook-load-test.md)。

## 📄 许可证

[MIT License](LICENSE)
