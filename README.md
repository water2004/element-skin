# Element-Skin — Minecraft Yggdrasil 皮肤站

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/water2004/element-skin)

基于 Vue 3 + FastAPI 的现代化 Minecraft 外置登录系统。采用 **Python 3.14 Free Threading** 构建，开启 GIL-free 时代，具备极致的并发处理潜能。

![](./img/root.png)

## ✨ 功能特性

- **🚀 极致性能**: 后端基于 Python 3.14 并开启 **Free Threading (GIL-free)**，结合 `uvloop` 充分发挥多核并发优势。
- **🗄️ 现代化数据库**: 使用 **PostgreSQL 18** 作为主存储，支持高性能异步驱动 (`asyncpg`)。
- **✅ 完整协议支持**: 完美实现 Yggdrasil API，无缝对接 Authlib-Injector 等主流加载器。
- **✅ 皮肤管理**: 支持皮肤/披风上传，集成 SkinView3D 提供丝滑的 3D 实时预览。
- **✅ 完善的用户系统**: 包含邮箱验证、注册验证码、密码找回流程（支持 SMTP）。
- **✅ 强大的管理后台**: 响应式设计，支持用户管理、邀请码机制、轮播图配置及邮件服务测试。
- **✅ 安全与防护**: 内置 API 速率限制 (Rate Limiting) 及多种安全防护机制。
- **✅ 灵活部署**: 既支持 Docker 一键部署，也支持复杂的子目录 (Sub-path) 架构。

---

## 🚀 Docker 部署指南 (推荐)

项目现在默认使用 **PostgreSQL 18** 并支持自动化初始化。

### 1. 准备配置文件

在宿主机创建 `config.yaml` 文件。这是系统运行的核心配置。

```yaml
# Element-Skin 配置文件

jwt:
  secret: "dev-secret-please-change-to-a-very-long-string-in-production"  # ⚠️ 生产环境必须修改为随机长字符串

# RSA 密钥配置 (系统会自动生成)
keys:
  private_key: "/app/private.pem"
  public_key: "/app/public.pem"

database:
  # 格式: postgresql://用户名:密码@db:5432/数据库名?sslmode=disable
  dsn: "postgresql://elementskin:password123@db:5432/elementskin?sslmode=disable"
  max_connections: 20

textures:
  directory: "/app/textures"

carousel:
  directory: "/app/carousel"

server:
  host: "0.0.0.0"
  port: 8000
  # ⚠️ 站点的外部访问地址
  site_url: "http://yourdomain.com" 
  # ⚠️ 后端 API 外部访问地址
  api_url: "http://yourdomain.com/skinapi" 

# CORS 跨域配置
cors:
  allow_origins: ["*"]
  allow_credentials: true

mojang:
  session_url: "https://sessionserver.mojang.com"
  account_url: "https://api.mojang.com"
  services_url: "https://api.minecraftservices.com"
  skin_domains: ["textures.minecraft.net"]
  cache_ttl: 3600
```

### 2. 选择部署方案

#### 方案 A：根目录部署 —— ✅ 推荐
请根据你的需求选择一种方案，配置 `docker-compose.yml` 和 `Nginx`。

#### 方案 A：根目录部署 (GHCR 镜像) —— ✅ 推荐
*无需本地构建，开箱即用。*

**docker-compose.yml**
```yaml
version: '3.8'
services:
  backend:
    image: ghcr.io/water2004/element-skin-backend:main
    container_name: element-skin-backend
    restart: unless-stopped
    ports:
      - "8000:8000"
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./data:/data
  frontend:
    image: ghcr.io/water2004/element-skin-frontend:main
    container_name: element-skin-frontend
    restart: unless-stopped
    ports:
      - "3000:80"
    volumes:
      - ./data/textures:/usr/share/nginx/html/static/textures:ro
      - ./data/carousel:/usr/share/nginx/html/static/carousel:ro
```

在项目的根目录下, 有一份完整的`docker-compose.yml`配置模板, 但若是使用ghcr镜像, 上面的配置已经足够

**Nginx 主机配置**
```nginx
server {
    listen 80;
    server_name yourdomain.com;

    location / {
        proxy_pass http://localhost:3000/; # 注意末尾的 /
    }

    # 后端 API 转发
    # 注意：使用 GHCR 镜像时，后端必须匹配 /skinapi 路径
    location /skinapi/ {
        proxy_pass http://localhost:8000; # 注意末尾没有 /
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
    
    # 处理不带斜杠的请求
    location = /skinapi {
        proxy_pass http://localhost:8000/skinapi/; # 注意末尾的 /
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

---

#### 方案 B：子目录部署 (本地构建)
*适用于将皮肤站部署在 `https://example.com/skin/` 这样的子路径下。此方案需要本地编译前端。*

**启动命令**
根据你的路径需求，修改项目根目录下的`docker-compose.yml`, 并使用对应的环境变量启动：

| 场景 | 前端路径 | 后端路径 | 启动命令 |
|-----|---------|---------|---------|
| **场景 1** | `/skin/` | `/skinapi` | `VITE_BASE_PATH=/skin/ docker compose up -d --build` |
| **场景 2** | `/skin/` | `/skin/api/` | `VITE_BASE_PATH=/skin/ VITE_API_BASE=/skin/api docker compose up -d --build` |

> 💡 **低内存模式**: 如果构建时内存不足，可添加 `BUILD_MODE=low-memory` 环境变量跳过类型检查。

**Nginx 主机配置 (对应场景 1)**
```nginx
location /skin/ {
    proxy_pass http://localhost:3000/; # 末尾有 /，去除 /skin/ 前缀
}
location /skinapi/ {
    proxy_pass http://localhost:8000;  # 末尾无 /，保留完整路径
    proxy_set_header Host $host;
}
# 处理不带斜杠的请求
location /skinapi {
    proxy_pass http://localhost:8000/skinapi/;  # 末尾有 /
    proxy_set_header Host $host;
}
```

**Nginx 主机配置 (对应场景 2)**
```nginx
location /skin/ {
    proxy_pass http://localhost:3000/;
}
location /skin/api/ {
    proxy_pass http://localhost:8000;
    proxy_set_header Host $host;
}
# 处理不带斜杠的请求
location /skin/api {
    proxy_pass http://localhost:8000/skin/api/;
    proxy_set_header Host $host;
}
```

---

### 3. 初始化设置 (重要)

容器启动成功后，PostgreSQL 会通过 `init.sql` 自动完成建表。

1.  **注册管理员**: 访问站点，注册的**第一个账号**将自动获得管理员权限。
2.  **配置站点设置**: 登录后进入 `管理面板` -> `站点设置` 修改名称和开关。
3.  **配置邮件服务**: 进入 `管理面板` -> `邮件服务` 配置 SMTP，即可启用验证码和密码找回。

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

#### 2. 后端 (Python 3.14+)
```bash
cd skin-backend
python -m venv .venv
# Windows: .venv\Scripts\activate | Linux: source .venv/bin/activate
pip install -r requirements.txt
python gen_key.py                # 生成密钥
# 运行测试 (需本地开启 PG)
$env:PYTHONPATH='.'; ..\.venv\Scripts\python.exe -m pytest tests/
```

#### 3. 前端 (Node.js)
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
├── skin-backend/       # 后端源码 (FastAPI)
│   ├── database_module/# PostgreSQL 异步适配
│   ├── init.sql        # 自动初始化脚本
│   └── ...
├── config.yaml         # 配置文件
├── pgdata/             # 数据库物理存储 (自动生成)
├── docker-compose.yml  
└── README.md
```

## 📋 TODO 

### 核心功能
- [x] 完整的 Yggdrasil 协议支持
- [x] Python 3.14 Free Threading + uvloop 优化
- [x] PostgreSQL 18 数据库适配
- [x] 基于 Docker InitDB 的自动化建表
- [x] 邀请码注册机制
- [x] 公共皮肤库
- [ ] 更好的用户材质管理
  - [x] 允许用户配置已有的材质信息
  - [x] 公共皮肤库添加材质名称
  - [ ] 公共皮肤库按热度/时间排序
- [ ] 导入第三方皮肤站数据

### 安全与性能
- [x] JWT 认证 (HS256, 32字节以上密钥)
- [x] API 速率限制
- [x] 数据库连接池适配 (asyncpg)
- [ ] Redis 缓存支持
- [ ] 材质存储优化 (S3/OSS 支持)

---

## 🧪 自动化测试

项目采用了分层测试架构，确保从底层数据库到顶层 API 的稳定性。

1.  **数据库层 (tests/database/)**: 验证 PG 逻辑、Schema 自动重置及缓存一致性。
2.  **业务逻辑层 (tests/backends/)**: 验证注册权限、材质级联更新。
3.  **API 接口层 (tests/api/)**: 模拟真实 HTTP 请求，验证路由。

## 📄 许可证

[MIT License](LICENSE)
