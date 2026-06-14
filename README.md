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

- **✅ 极致性能**: 后端基于 Go 重构，使用 PostgreSQL + Redis 支撑高并发读写路径。
- **✅ 现代化数据库**: 使用 **PostgreSQL 18** 作为主存储，Go 后端通过高性能 PostgreSQL 驱动与连接池访问数据。
- **✅ 完整协议支持**: 完美实现 Yggdrasil API，无缝对接 Authlib-Injector 等主流加载器。
- **✅ 完整的Fallback机制**: 支持多个第三方服务作为数据源，允许其他其他皮肤站的用户进入服务器。
- **✅ 正版登录支持**: 集成 Mojang 官方认证服务，允许正版用户直接使用 Minecraft 账号登录。
- **✅ 皮肤管理**: 支持皮肤/披风上传，集成 SkinView3D 提供丝滑的 3D 实时预览。
- **✅ 完善的用户系统**: 包含邮箱验证、注册验证码、密码找回流程（支持 SMTP）。
- **✅ 强大的管理后台**: 响应式设计，支持用户管理、邀请码机制、轮播图配置及邮件服务测试。
- **✅ 安全与防护**: 内置 API 速率限制 (Rate Limiting) 及多种安全防护机制。
- **✅ 灵活部署**: 既支持 Docker 一键部署，也支持复杂的子目录 (Sub-path) 架构。

---

## 🚀 Docker 部署指南 (推荐)

项目现在默认使用 **PostgreSQL 18 + Redis** 并支持自动化初始化。PostgreSQL 保存用户、设置、材质元数据等持久化数据；Redis 负责公开配置/轮播缓存、邮件验证码、限流数据和短期用户鉴权缓存等临时状态。

### 1. 准备配置文件

在宿主机创建`docker-compose.yml`文件，内容如下：

**docker-compose.yml**
```yaml
version: '3.8'
services:
  db:
    image: postgres:18-alpine
    restart: always
    environment:
      POSTGRES_USER: elementskin
      POSTGRES_PASSWORD: password123 #⚠️ 生产环境请修改密码
      POSTGRES_DB: elementskin
    volumes:
      - ./data/db:/var/lib/postgresql
    ports:
      - "5432:5432" # 在迁移完成后可以关闭这个端口暴露
  redis:
    image: redis:8-alpine
    restart: always
    command: ["redis-server", "--appendonly", "yes", "--requirepass", "password123"]
    volumes:
      - ./data/redis:/data
    healthcheck:
      test: ["CMD", "redis-cli", "-a", "password123", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5
  backend:
    image: ghcr.io/water2004/element-skin:latest
    container_name: element-skin
    restart: unless-stopped
    environment:
      - VITE_BASE_PATH=${VITE_BASE_PATH:-/}    # 👈 前端部署路径 (如 /skin/)
      - VITE_API_BASE=${VITE_API_BASE:-/skinapi} # 👈 后端 API 路径 (如 /skinapi)
    depends_on:
      db:
        condition: service_healthy
      redis:
        condition: service_healthy
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./frontend:/app/frontend           # 前端、皮肤、轮播图全部在这里
      - ./data:/app/data                   # 密钥文件
    ports:
      - "8000:8000"
```

> 💡 **动态路径配置**: 镜像支持在启动时通过环境变量动态修改路径。修改后直接 `docker compose restart` 即可生效，无需重新构建。

在宿主机创建 `config.yaml` 文件。这是系统运行的核心配置。

```yaml
# Element-Skin 配置文件

jwt:
  secret: "dev-secret-please-change-to-a-very-long-string-in-production"  # ⚠️ 生产环境必须修改为随机长字符串

# RSA 密钥配置 (系统会自动生成并持久化)
keys:
  private_key: "/app/data/private.pem"
  public_key: "/app/data/public.pem"

database:
  # 格式: postgresql://用户名:密码@db:5432/数据库名?sslmode=disable
  dsn: "postgresql://elementskin:password123@db:5432/elementskin?sslmode=disable" #⚠️ 用户名和密码请确保与 PostgreSQL 环境变量一致
  max_connections: 20

redis:
  addr: "redis:6379"
  password: "password123" # ⚠️ 与 docker compose 中 Redis 密码一致，生产环境请修改
  db: 0
  key_prefix: "elementskin:"
  public_cache_ttl_seconds: 60
  auth_cache_ttl_seconds: 30

textures:
  directory: "/app/frontend/static/textures"

carousel:
  directory: "/app/frontend/static/carousel"

server:
  host: "0.0.0.0"
  port: 8000
  # ⚠️ 如果前端部署在子目录, 这里也需要修改 (如 /skin/)
  root_path: "/skinapi" 
  # ⚠️ 站点的外部访问地址
  site_url: "http://yourdomain.com" 
  # ⚠️ 后端 API 外部访问地址
  api_url: "http://yourdomain.com/skinapi" 

# CORS 跨域配置
cors:
  allow_origins: ["*"] # ⚠️ 生产环境请根据实际情况限制来源
  allow_credentials: true
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
        proxy_pass http://localhost:8000;
        proxy_set_header Host $host;
    }
    
    # 直接转发不带斜杠的 API 请求
    location = /skinapi {
        proxy_pass http://localhost:8000/skinapi/;
        proxy_set_header Host $host;
    }
}
```
最后，启动 Docker：

```bash
docker compose up -d
```

对于希望前端或后端地址部署在子目录的用户，可以通过参数灵活配置路径：
- **前端路径**: 通过 `VITE_BASE_PATH` 定义前端资源的基础路径
- **后端路径**: 通过 `VITE_API_BASE` 定义后端 API 的基础路径

根据你的路径需求，在启动时传入环境变量。前端会根据这些参数编译，并自动释放到宿主机的 `./frontend` 目录：

| 场景 | 前端路径 | 后端路径 | 启动命令 |
|-----|---------|---------|---------|
| **场景 1** | `/skin/` | `/skinapi` | `VITE_BASE_PATH=/skin/ docker compose up -d` |
| **场景 2** | `/skin/` | `/skin/api/` | `VITE_BASE_PATH=/skin/ VITE_API_BASE=/skin/api docker compose up -d` |

需要注意的是，`config.yaml` 中的 `server.site_url` 和 `server.api_url` 也需要根据实际部署路径进行调整，以确保生成的链接正确。

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
    proxy_pass http://localhost:8000;
    proxy_set_header Host $host;
}
location = /skinapi {
    proxy_pass http://localhost:8000/skinapi/;
    proxy_set_header Host $host;
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
    proxy_pass http://localhost:8000;
    proxy_set_header Host $host;
}
location = /skin/api {
    proxy_pass http://localhost:8000/skin/api/;
    proxy_set_header Host $host;
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
3.  **修改配置**: 编辑 `skin-backend/config.yaml` 中的 `database.dsn`：
    ```yaml
    database:
      # 必须匹配：.../数据库名?sslmode...
      dsn: "postgresql://elementskin:password123@localhost:5432/elementskin?sslmode=disable"
    ```
    > 💡 **自动初始化**: 后端在每次启动时会自动同步数据库结构（创建缺失的表及默认配置），无需手动执行 SQL 脚本。

#### 2. Redis 配置
本地开发需要 Redis 运行在 `127.0.0.1:6379`。如果你的 Redis 设置了密码，请同步修改 `skin-backend/config.yaml`：

```yaml
redis:
  addr: "127.0.0.1:6379"
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
├── config.yaml         # 配置文件
├── pgdata/             # 数据库物理存储 (自动生成)
├── docker-compose.yml  
└── README.md
```

## 📋 功能状态

### 核心功能
- [x] 完整的yggdrasil协议支持
- [x] 用户注册与登录
- [x] 用户材质上传
- [x] 游戏角色管理
- [x] 邮箱验证码与密码找回
- [x] 邀请码注册机制
- [x] Mojang服务fallback机制
- [x] 用户封禁与解封
- [x] 公共皮肤库
- [x] 用户材质管理
  - [x] 允许用户删除自己上传到公共库的材质
  - [x] 允许用户配置已有的材质信息, 如模型类型等
  - [x] 公共皮肤库添加材质名称
  - [x] 公共皮肤库按名称搜索
  - [x] 公共皮肤库按上传时间排序
- [x] 多个fallback服务支持
- [x] 导入第三方皮肤站的角色和材质数据

### 安全与性能
- [x] PostgreSQL 数据库模块
- [x] JWT认证机制
- [x] API速率限制
- [x] Redis 缓存、限流、邮件验证码与短期鉴权缓存
- [x] 管理员设置细粒度API
- [x] 数据库性能优化
- [x] PostgreSQL 连接池
- [x] Redis缓存支持
- [ ] 材质存储优化（如使用云存储或CDN）

### 前端优化
- [x] 响应式设计
- [x] 深色模式支持
- [x] 页脚信息（如站点名称、版权信息等）
- [ ] 国际化 (i18n) 支持
- [ ] 移动端适配优化
- [x] 前端性能优化（如图片懒加载、代码分割等）

### 端点与集成
- [ ] 移动端 App 认证接口
- [ ] 第三方登录（GitHub、微博等）
- [ ] 批量材质导入工具

### 测试
- [x] Go 分层自动化测试框架
- [x] 数据库层 (Database Layer) 核心接口覆盖
- [x] 业务逻辑层 (Service Layer) 核心规则覆盖
- [x] API 接口层 (HTTP Integration Layer) 核心流程覆盖
- [x] 固定并发压测覆盖公开接口、用户中心、管理后台与 Yggdrasil 常用端点

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

最新一次压测在本机通过 `skin-backend/cmd/loadtest` 启动隔离测试数据库、真实 Redis key 前缀和进程内 HTTP 服务完成，不会触碰正常运行数据库。命令如下：

```bash
cd skin-backend
LOADTEST_ENABLE=1 LOADTEST_CONCURRENCY=200 LOADTEST_DURATION=1s go test ./cmd/loadtest -run TestRealBackendLoad -count=1 -v
```

测试数据：100 个用户、300 个角色、500 条材质记录、50 个邀请码、1 个预置 Yggdrasil join 会话。固定并发：200；每个场景窗口：1s；数据库连接池：20。

| 场景 | Go 成功 req/s | Python 成功 req/s | 提升 | Go P95 | Python P95 |
| --- | ---: | ---: | ---: | ---: | ---: |
| Public settings | 26105.8 | 1913.7 | 13.6x | 9.1ms | 200.3ms |
| Public carousel | 30420.8 | 2138.0 | 14.2x | 8.2ms | 113.4ms |
| Public skin library search | 16894.7 | 777.9 | 21.7x | 17.0ms | 552.6ms |
| Site login | 305.6 | 42.1 | 7.3x | 695.7ms | 4.58s |
| Yggdrasil metadata | 32938.5 | 2694.4 | 12.2x | 7.5ms | 110.9ms |
| Yggdrasil authenticate | 292.1 | 42.6 | 6.9x | 1.04s | 4.54s |
| Yggdrasil validate | 31803.1 | 1126.3 | 28.2x | 7.8ms | 422.1ms |
| Yggdrasil profile | 61355.0 | 1782.7 | 34.4x | 5.2ms | 151.1ms |
| Yggdrasil lookup name | 64973.6 | 1827.5 | 35.6x | 4.8ms | 164.2ms |
| Yggdrasil hasJoined | 2072.2 | 250.8 | 8.3x | 127.6ms | 1.36s |
| Me | 20258.1 | 984.3 | 20.6x | 13.6ms | 384.1ms |
| My profiles | 28928.8 | 891.2 | 32.5x | 8.9ms | 469.3ms |
| My textures | 29838.0 | 1125.8 | 26.5x | 8.5ms | 361.6ms |
| Texture detail | 29216.8 | 1101.1 | 26.5x | 8.6ms | 360.5ms |
| Admin users | 18290.2 | 672.9 | 27.2x | 16.7ms | 780.4ms |
| Admin user detail | 28837.8 | 822.2 | 35.1x | 8.9ms | 510.3ms |
| Admin user profiles | 28739.6 | 1032.5 | 27.8x | 9.1ms | 689.5ms |
| Admin profiles | 22630.1 | 809.2 | 28.0x | 13.2ms | 822.5ms |
| Admin textures | 22827.7 | 793.0 | 28.8x | 13.6ms | 659.7ms |
| Admin invites | 24581.6 | 915.9 | 26.8x | 12.1ms | 371.8ms |
| Admin settings/site | 2415.1 | 1318.3 | 1.8x | 90.0ms | 890.1ms |

完整报告见 [`reports/concurrency-load-test.md`](reports/concurrency-load-test.md)，Python 对照基线为 `dev:reports/python-concurrency-load-test.md`。

## 📄 许可证

[MIT License](LICENSE)
