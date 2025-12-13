# Element-Skin — Minecraft Yggdrasil 皮肤站

基于 Vue 3 + FastAPI 的 Minecraft 外置登录系统。

![](./img/root.png)

## 功能特性

- ✅ 完整的 Yggdrasil API 支持
- ✅ 皮肤/披风管理（3D预览）
- ✅ 用户管理和权限系统
- ✅ 速率限制和安全防护
- ✅ 支持子目录部署

---

## 快速开始

### RSA 密钥生成

> **重要提示**: RSA 密钥会在容器首次启动时自动生成到 `data` 目录，实现数据持久化

如果需要提前生成：
```bash
# 在项目根目录，确保 data 目录存在
mkdir -p data

# 生成密钥到 data 目录
cd skin-backend
python gen_key.py
mv private.pem public.pem ../data/
cd ..
```

> 容器启动时会检查 `/data/` 目录，如果密钥不存在才会生成，避免覆盖现有密钥。

### 开发环境

#### 后端
```bash
cd skin-backend
python -m venv .venv
.venv\Scripts\activate  # Windows
# source .venv/bin/activate  # Linux/macOS
pip install -r requirements.txt
python gen_key.py
uvicorn routes_reference:app --reload
```

#### 前端
```bash
cd element-skin
npm install
npm run dev
```

访问 http://localhost:5173

---

## Docker 部署

### 1. 准备配置

创建 `config.yaml`：
```yaml
jwt:
  secret: "CHANGE-ME-TO-RANDOM-SECRET"
database:
  path: "/data/yggdrasil.db"
textures:
  directory: "/data/textures"
server:
  host: "0.0.0.0"
  port: 8000
```

创建数据目录：
```bash
mkdir data
```

### 2. 启动容器

**根目录部署**（前端在 `/`，后端在 `/authserver` 等）：
```bash
docker compose up -d --build
```

**前端子目录部署**（前端在 `/skin/`，后端在根路径）：
```bash
VITE_BASE_PATH=/skin/ docker compose up -d --build
```

**前端+后端都在子目录**（前端在 `/skin/`，后端在 `/skin/api/`）：

需要在 `config.yaml` 中配置 `server.root_path: "/skin/api"`，然后：
```bash
VITE_BASE_PATH=/skin/ VITE_API_BASE=/skin/api docker compose up -d --build
```

> **注意**: 修改环境变量后必须使用 `--build` 重新构建镜像

### 3. 更新镜像

代码修改后，需要重新构建镜像：

```bash
# 重新构建并启动（所有服务）
docker compose up -d --build

# 或者单独重建某个服务
docker compose up -d --build backend   # 仅重建后端
docker compose up -d --build frontend  # 仅重建前端
```

> **提示**: `--build` 参数会强制重新构建镜像，确保代码变更生效

### 4. 配置主机 Nginx

参考 `nginx-host.conf`，配置主机的 Nginx 反向代理。

**方案1: 根目录部署**（推荐）
```nginx
# 前端: /
location / {
    proxy_pass http://localhost:3000;
}

# 后端: /authserver, /api 等
location ~ ^/(authserver|sessionserver|admin|register|textures|static|api|me|public|docs) {
    proxy_pass http://localhost:8000;
}
```

**方案2: 前端子目录**（前端在 `/skin/`，后端在根路径）
```nginx
location /skin/ {
    proxy_pass http://localhost:3000/;
}
location ~ ^/(authserver|...) {
    proxy_pass http://localhost:8000;
}
```
部署方案对比

| 方案 | 前端路径 | 后端路径 | 前端配置 | 后端配置 | 适用场景 |
|-----|---------|---------|---------|---------|---------|
| 方案1 | `/` | `/authserver` 等 | 默认 | 默认 | 推荐，配置最简单 |
| 方案2 | `/skin/` | `/authserver` 等 | `VITE_BASE_PATH=/skin/` | 默认 | 前端与其他应用共存 |
| 方案3 | `/skin/` | `/skin/api/` | `VITE_BASE_PATH=/skin/`<br>`VITE_API_BASE=/skin/api` | `root_path="/skin/api"` | 完全隔离的子目录 |

**方案3: 前端+后端都在子目录**（前端 `/skin/`，后端 `/skin/api/`）

在 `config.yaml` 中配置后端路径前缀：
```yaml
server:
  root_path: "/skin/api"
```

然后启动：
```bash
VITE_BASE_PATH=/skin/ VITE_API_BASE=/skin/api docker compose up -d --build
```

配置Nginx：
```nginx
location /skin/ {
    proxy_pass http://localhost:3000/;
}
location /skin/api/ {
    proxy_pass http://localhost:8000/skin/api/;
}
```

> **重要**: 
> - 后端在子目录时，需要在 FastAPI 中配置 `root_path`，无需 Nginx rewrite
> - 前端构建时设置 `VITE_API_BASE=/skin/api`
> - Minecraft 客户端的 authlib-injector 地址为 `http://yourdomain.com/skin/api`
> - 完整配置见 `nginx-host.conf` 文件

### 4. 首次配置

**安全配置（生产环境必须修改）**：

1. **修改 JWT 密钥**：
   ```bash
   # 生成随机密钥
   openssl rand -base64 32
   # 写入 config.yaml 的 jwt.secret
   ```

2. **配置 CORS 允许的域名**：
   编辑 `config.yaml`，将 `cors.allow_origins` 改为你的前端域名：
   ```yaml
   cors:
     allow_origins: ["https://yourdomain.com"]  # 不要用 ["*"]！
     allow_credentials: true
   ```

**初始化设置**：

1. 访问站点并注册账号（第一个用户自动成为管理员）
2. 登录后进入「管理面板」→「设置」
3. 配置站点 URL（必须与实际访问地址一致）

---必须以 `/` 开头和结尾 |
| `VITE_API_BASE` | 空 | 后端API前缀，仅当后端也在子目录时设置 |

**示例**:
```bash
# 根目录部署
docker compose up -d

# 前端子目录，后端根路径
VITE_BASE_PATH=/skin/ docker compose up -d --build

# 前端+后端都在子目录
VITE_BASE_PATH=/skin/ VITE_API_BASE=/skin/api docker compose up -d --build
```
## 配置说明

### Docker Compose 参数

| 参数 | 默认值 | 说明 |
|-----|--------|------|
| `VITE_BASE_PATH` | `/` | 前端部署路径，子目录需以 `/` 开头和结尾 |
| `VITE_API_BASE` | 空 | API 地址，通常留空 |

### 端口映射

- 前端：`3000:80`（容器内80端口映射到主机3000）
- 后端：`8000:8000`

### 目录映射

- `./config.yaml` → `/app/config.yaml`（配置文件）
- `./data` → `/data`（数据库和材质文件）

---

## 生产部署建议

1. **修改 JWT 密钥**
   ```bash
   # 生成随机密钥
   openssl rand -base64 32
   # 写入 config.yaml
   ```

2. **配置 HTTPS**
   ```nginx
   server {
       listen 443 ssl;
       ssl_certificate /path/to/cert.pem;
       ssl_certificate_key /path/to/key.pem;
       # ... 其他配置
   }
   ```

3. **备份数据**
   ```bash
   # 定期备份 data 目录
   tar -czf backup-$(date +%Y%m%d).tar.gz data/
  如后端在子目录，确认设置了 `VITE_API_BASE`
- 如后端在子目录，确认 Nginx 配置了 rewrite 规则
- 确认后端容器在 8000 端口运行

### Minecraft 客户端连接失败
- **方案1和2**: 后端在根路径，authlib-injector 地址为 `http://yourdomain.com`
- **方案3**: 后端在子目录，authlib-injector 地址为 `http://yourdomain.com/skin/api`
- 管理面板的站点 URL 必须与 authlib-injector 地址一致

### 样式丢失（子目录部署）
- 检查 `VITE_BASE_PATH` 是否正确（必须以 `/` 开头和结尾）
- 确认已重新构建：`docker compose up -d --build`

### 前端构建失败
- 如果没有 `package-lock.json`，Dockerfile 会自动使用 `npm install`
- 检查 Node.js 版本是否为 20+

### 后端密钥文件缺失
- 容器首次启动时会自动生成到 `data` 目录
- 检查容器日志：`docker compose logs backend`
- 如果密钥丢失，删除 `data/private.pem` 和 `data/public.pem`，然后重启容器

### API 请求 404
- 检查主机 Nginx 配置中的后端代理路径
- **方案3**: 后端在子目录时，确认 `config.yaml` 中设置了 `server.root_path: "/skin/api"`
- 确认前端设置了 `VITE_API_BASE=/skin/api`
- 确认后端容器在 8000 端口运行
- 检查是否已重新构建：`docker compose up -d --build backend`

### Minecraft 客户端连接失败
- 后端 API 必须在根路径（`/authserver`）
- 管理面板的站点 URL 设置为根域名

### 材质显示异常
- 检查管理面板的「站点 URL」设置
- 站点 URL 必须与实际访问地址一致

---

## 项目结构

```
element-skin/
├── element-skin/       # 前端（Vue 3）
├── skin-backend/       # 后端（FastAPI）
├── config.yaml         # 配置文件
├── data/               # 数据目录（数据库+材质）
├── docker-compose.yml  # Docker 编排
└── nginx-host.conf     # 主机 Nginx 配置示例
```

---

## 技术栈

**前端**: Vue 3 + Element Plus + SkinView3D  
**后端**: FastAPI + SQLite + bcrypt  
**部署**: Docker + Nginx

---

## 许可证

MIT License
