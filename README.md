<h1 align="center" style="margin-bottom: 8px;">
  <img src="./logo.svg" width="92" alt="Element-Skin Logo" style="vertical-align: -33px;"> <span style="color: #3b91e6;">Element-Skin</span>
</h1>

<p align="center" style="margin-top: 0; margin-bottom: 14px; color: #374151;">
  面向高并发场景的现代化外置登录与材质平台
</p>

<p align="center" style="margin-top: 0; margin-bottom: 16px; color: #4B5563;">
  基于 Vue 3 + FastAPI，采用 <strong>Python 3.14 Free Threading</strong> 构建，释放多核并发潜能。
</p>

<p align="center">
  <a href="https://deepwiki.com/water2004/element-skin">
    <img src="https://deepwiki.com/badge.svg">
  </a>
  <a href="LICENSE">
    <img src="https://img.shields.io/github/license/water2004/element-skin">
  </a>
  <img src="https://img.shields.io/badge/Vue-3-4FC08D?logo=vue.js&logoColor=white">
  <img src="https://img.shields.io/badge/Python-3.14t-3776AB?logo=python&logoColor=white">
  <img src="https://img.shields.io/badge/PostgreSQL-4169E1?logo=postgresql&logoColor=white">
</p>

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
      - ./data/db:/var/lib/postgresql/data
  backend:
    image: ghcr.io/water2004/element-skin:latest
    container_name: element-skin
    restart: unless-stopped
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./frontend:/app/frontend           # 前端、皮肤、轮播图全部在这里
    ports:
      - "8000:8000"
```

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
  dsn: "postgresql://elementskin:password123@db:5432/elementskin?sslmode=disable" #⚠️ 用户名和密码请确保与 PostgreSQL 环境变量一致
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
---

## Docker本地构建
对于希望前端或后端地址部署在子目录的用户，可以通过构建参数灵活配置路径：
- **前端路径**: 通过 `VITE_BASE_PATH` 定义前端资源的基础路径
- **后端路径**: 通过 `VITE_API_BASE` 定义后端 API 的基础路径

根据你的路径需求，在启动时传入环境变量。前端会根据这些参数编译，并自动释放到宿主机的 `./frontend` 目录：

| 场景 | 前端路径 | 后端路径 | 启动命令 |
|-----|---------|---------|---------|
| **场景 1** | `/skin/` | `/skinapi` | `VITE_BASE_PATH=/skin/ docker compose up -d --build` |
| **场景 2** | `/skin/` | `/skin/api/` | `VITE_BASE_PATH=/skin/ VITE_API_BASE=/skin/api docker compose up -d --build` |

**Nginx 主机配置 (对应场景 1)**
```nginx
# 1. 前端静态文件
location /skin/ {
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

> 💡 **低内存模式**: 如果构建时内存不足，可添加 `BUILD_MODE=low-memory` 环境变量跳过类型检查。

---

## 从1.3.1升级到2.0.0
2.x版本最大的更新是数据库从 SQLite 切换到 PostgreSQL，因此需要进行数据迁移。在迁移之前，请**确保皮肤站版本已经升级到 v1.3.1**，并且已经备份了数据库文件。迁移步骤如下：

1. 按照上面的 Docker 部署指南，启动皮肤站服务
2. 按照你的数据库配置，编辑`sqlite_to_postgres.py`文件中的数据库连接字符串
3. 按照你原先的数据库文件路径，编辑`sqlite_to_postgres.py`文件中的SQLite数据库路径
4. 运行迁移脚本：
```bash
python sqlite_to_postgres.py
```
5. 迁移完成后，重启皮肤站服务，确保一切正常运行

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
uvicorn routes_reference:app --reload --host 0.0.0.0
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
- [x] 完整的yggdrasil协议支持
- [x] 用户注册与登录
- [x] 用户材质上传
- [x] 游戏角色管理
- [x] 邮箱验证码与密码找回
- [x] 邀请码注册机制
- [x] Mojang服务fallback机制
- [x] 用户封禁与解封
- [x] 公共皮肤库
- [ ] 更好的用户材质管理
  - [x] 允许用户删除自己上传到公共库的材质
  - [x] 允许用户配置已有的材质信息, 如模型类型等
  - [x] 公共皮肤库添加材质名称
  - [ ] 公共皮肤库按名称搜索
  - [ ] 公共皮肤库按上传时间排序,热度排序
- [x] 多个fallback服务支持
- [ ] 导入第三方皮肤站的角色和材质数据

### 安全与性能
- [x] sqlite数据库模块
- [x] JWT认证机制
- [x] API速率限制
- [x] 数据库内存缓存与连接池
- [x] 管理员设置细粒度API
- [ ] 数据库性能优化
- [ ] 多数据库支持（PostgreSQL、MySQL等）
- [ ] Redis缓存支持
- [ ] 材质存储优化（如使用云存储或CDN）

### 前端优化
- [x] 响应式设计
- [x] 深色模式支持
- [x] 页脚信息（如站点名称、版权信息等）
- [ ] 国际化 (i18n) 支持
- [ ] 移动端适配优化
- [ ] 前端性能优化（如图片懒加载、代码分割等）

### 端点与集成
- [ ] 移动端 App 认证接口
- [ ] 第三方登录（GitHub、微博等）
- [ ] 批量材质导入工具

### 测试
- [x] 分层自动化测试框架 (Pytest + Asyncio)
- [x] 数据库层 (Database Layer) 全接口覆盖
- [ ] 业务逻辑层 (Backend Logic Layer) 完整覆盖
- [ ] API 接口层 (Integration Layer) 核心流程覆盖

---

## 🧪 自动化测试

项目采用了分层测试架构，确保从底层数据库到顶层 API 的稳定性。

### 测试架构
1.  **数据库层 (tests/database/)**: 验证 SQL 逻辑、数据迁移及缓存一致性。
2.  **业务逻辑层 (tests/backends/)**: 验证核心业务规则（如注册权限、材质级联更新）。
3.  **API 接口层 (tests/api/)**: 模拟真实 HTTP 请求，验证路由、中间件及响应格式。

### 运行测试
测试会自动创建临时数据库和文件目录，不会影响本地开发数据。

```bash
cd skin-backend
# 安装测试依赖
pip install -r requirements.txt

# 运行所有测试
pytest tests/

# 查看详细输出
pytest -v
```

### 编写新测试
利用 `tests/conftest.py` 中预定义的 Fixtures 可以极速编写测试：
- `db_session`: 获取一个干净的临时数据库实例。
- `user_factory`: 快速创建测试用户。
- `auth_headers` / `admin_headers`: 自动生成带 JWT 的请求头。
- `client`: 异步 API 客户端。

## 📄 许可证

[MIT License](LICENSE)