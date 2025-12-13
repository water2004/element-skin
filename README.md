# Element-Skin — Minecraft Yggdrasil 认证服务器 & 皮肤站

<div align="center">

**一个现代化的 Minecraft 外置登录系统，基于 Vue 3 + FastAPI 构建**

[![Vue 3](https://img.shields.io/badge/Vue-3.5-4FC08D?logo=vue.js)](https://vuejs.org/)
[![FastAPI](https://img.shields.io/badge/FastAPI-0.95+-009688?logo=fastapi)](https://fastapi.tiangolo.com/)
[![Element Plus](https://img.shields.io/badge/Element_Plus-2.3-409EFF?logo=element)](https://element-plus.org/)
[![Python](https://img.shields.io/badge/Python-3.10+-3776AB?logo=python)](https://www.python.org/)

</div>

---

## 📖 项目简介

Element-Skin 是一个完整的 Minecraft 外置登录解决方案，实现了 Yggdrasil 认证协议，提供皮肤/披风管理、用户管理、权限系统等功能。

### 🌟 核心特性

#### 认证系统
- ✅ 完整的 Yggdrasil API 支持（登录、刷新、验证、登出）
- ✅ JWT Token 认证，可配置过期时间
- ✅ 多客户端会话管理
- ✅ RSA 数字签名支持

#### 皮肤系统
- ✅ 支持皮肤与披风上传（Steve/Alex 模型）
- ✅ 实时 3D 预览（基于 SkinView3D）
- ✅ 材质哈希去重存储
- ✅ 可配置文件大小限制

#### 安全特性
- ✅ Bcrypt 密码加密，自动迁移旧密码
- ✅ 可配置速率限制，防暴力破解
- ✅ 邀请码系统（可选）
- ✅ 角色权限管理（用户/管理员）

#### 管理功能
- ✅ 用户管理（封禁、删除、重置密码）
- ✅ 实时站点设置（无需重启）
- ✅ 材质管理（删除、替换）
- ✅ 邀请码管理

#### 用户体验
- ✅ 现代化 UI 设计（Element Plus）
- ✅ 响应式布局，支持移动端
- ✅ 流畅动画与交互反馈
- ✅ 暗色主题支持（TODO）

---

## 🏗️ 技术架构

### 前端技术栈
```
Vue 3 (Composition API) + TypeScript
├── Element Plus      # UI 组件库
├── Pinia            # 状态管理
├── Vue Router       # 路由管理
├── Axios            # HTTP 客户端
├── SkinView3D       # 3D 皮肤预览
└── Vite             # 构建工具
```

### 后端技术栈
```
FastAPI + Python 3.10+
├── aiosqlite        # 异步 SQLite 数据库
├── PyJWT            # JWT 令牌处理
├── cryptography     # RSA 签名
├── bcrypt           # 密码哈希
├── Pillow           # 图像处理
├── SlowAPI          # 速率限制
└── Uvicorn          # ASGI 服务器
```

### 目录结构
```
element-skin/
├── element-skin/          # 前端项目
│   ├── src/
│   │   ├── components/   # 通用组件
│   │   ├── views/        # 页面组件
│   │   ├── router/       # 路由配置
│   │   ├── stores/       # Pinia 状态
│   │   └── assets/       # 静态资源
│   ├── public/           # 公共资源
│   ├── package.json
│   └── vite.config.ts
│
├── skin-backend/          # 后端项目
│   ├── routes_reference.py  # API 路由
│   ├── backend.py           # 业务逻辑
│   ├── database.py          # 数据库操作
│   ├── models.py            # 数据模型
│   ├── config_loader.py     # 配置加载
│   ├── rate_limiter.py      # 速率限制
│   ├── gen_key.py           # RSA 密钥生成
│   ├── config.yaml          # 配置文件
│   ├── requirements.txt     # Python 依赖
│   └── textures/            # 材质存储目录（运行时创建）
│
└── README.md              # 本文档
```

---

## 🚀 快速开始

### 前置要求

- **Node.js** >= 20.19.0 或 >= 22.12.0
- **Python** >= 3.10
- **npm** 或 **yarn**（推荐）

### 开发环境搭建

#### 1. 克隆项目
```bash
git clone https://github.com/your-repo/element-skin.git
cd element-skin
```

#### 2. 后端安装与启动

```bash
cd skin-backend

# 创建虚拟环境
python -m venv .venv

# 激活虚拟环境
# Windows:
.\.venv\Scripts\activate
# macOS/Linux:
source .venv/bin/activate

# 安装依赖
pip install -r requirements.txt

# 生成 RSA 密钥对（首次运行必须）
python gen_key.py

# 启动开发服务器
uvicorn routes_reference:app --reload --host 0.0.0.0 --port 8000
```

后端将在 `http://localhost:8000` 启动，访问 `http://localhost:8000/docs` 查看 API 文档。

#### 3. 前端安装与启动

```bash
cd element-skin

# 安装依赖
npm install

# 启动开发服务器
npm run dev
```

前端将在 `http://localhost:5173` 启动，Vite 已配置代理，自动转发 API 请求到后端。

#### 4. 访问应用

- 前端地址：http://localhost:5173
- 后端 API：http://localhost:8000
- API 文档：http://localhost:8000/docs

**默认管理员账户：**  
首次启动后，请直接注册账号，第一个注册的用户将自动成为管理员。

---

## ⚙️ 配置说明

### 配置系统架构

Element-Skin 采用双层配置系统：

#### 1. 基础配置（`config.yaml`）— 需重启生效

```yaml
# JWT 认证配置
jwt:
  secret: "your-secret-key-here"  # ⚠️ 生产环境务必修改！

# 数据库配置
database:
  path: "yggdrasil.db"

# 材质存储配置
textures:
  directory: "textures"

# 服务器配置
server:
  host: "0.0.0.0"
  port: 8000
```

**环境变量覆盖**（优先级更高）：
```bash
# 双下划线表示层级结构
export JWT__SECRET="production-secret-key"
export DATABASE__PATH="/data/yggdrasil.db"
export TEXTURES__DIRECTORY="/data/textures"

# 前端配置（部署到子目录时使用）
export VITE_BASE_PATH="/skin/"  # 必须以 / 开头和结尾
export VITE_API_BASE=""         # API 基础路径（通常留空）
```

#### 2. 运营配置（管理面板 → 设置）— 实时生效

在管理面板中可配置：
- 站点名称、URL
- JWT 过期时间
- 速率限制（开关、尝试次数、时间窗口）
- 材质大小限制
- 注册开关、邀请码要求

**建议实践：**  
基础配置用于部署初始化，日常运营配置通过管理面板修改，无需重启服务。

---

## 🐳 Docker 部署（推荐）

> **📘 完整部署指南**: 请参阅 [DEPLOYMENT.md](./DEPLOYMENT.md) 获取详细的生产环境部署步骤。

### 快速开始（一键部署）

#### Linux/macOS

```bash
# 1. 克隆项目
git clone https://github.com/your-repo/element-skin.git
cd element-skin

# 2. 运行快速部署脚本
chmod +x quick-deploy.sh
./quick-deploy.sh

# 3. 访问应用
# 前端: http://localhost/
# 后端: http://localhost:8000/
```

#### Windows

```powershell
# 1. 克隆项目
git clone https://github.com/your-repo/element-skin.git
cd element-skin

# 2. 运行快速部署脚本
.\quick-deploy.bat

# 3. 访问应用
# 前端: http://localhost/
# 后端: http://localhost:8000/
```

**快速部署脚本会自动完成：**
- ✅ 创建目录结构
- ✅ 生成 JWT 密钥和 RSA 密钥对
- ✅ 创建配置文件
- ✅ 构建 Docker 镜像
- ✅ 启动服务

> **📘 子目录部署**: 如需将前端部署到子目录（如 `/skin/`），请使用：
> ```bash
> # Linux/macOS
> ./quick-deploy-subdir.sh /skin/
> ```
> 详细说明请参阅 [SUBDIRECTORY_DEPLOYMENT.md](./SUBDIRECTORY_DEPLOYMENT.md) 或 [DEPLOYMENT.md](./DEPLOYMENT.md) 的"子目录部署指南"章节。

### 方案一：Docker Compose（手动部署）

#### 1. 准备配置文件

创建部署目录：
```bash
mkdir -p element-skin-deploy
cd element-skin-deploy
```

创建 `config.yaml`：
```yaml
jwt:
  secret: "CHANGE-THIS-TO-A-RANDOM-SECRET-KEY"
database:
  path: "/data/yggdrasil.db"
textures:
  directory: "/data/textures"
server:
  host: "0.0.0.0"
  port: 8000
```

#### 2. 创建 docker-compose.yml

参见下方完整配置示例。

#### 3. 启动服务

```bash
# 构建并启动
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down

# 重启服务
docker-compose restart
```

#### 4. 访问应用

- 前端：http://your-domain.com
- 后端 API：http://your-domain.com:8000
- API 文档：http://your-domain.com:8000/docs

### 方案二：手动构建镜像

#### 1. 构建后端镜像

```bash
cd skin-backend
docker build -t element-skin-backend:latest .
```

#### 2. 构建前端镜像

```bash
cd element-skin
docker build -t element-skin-frontend:latest .
```

#### 3. 运行容器

```bash
# 后端容器
docker run -d \
  --name element-skin-backend \
  -p 8000:8000 \
  -v /path/to/config.yaml:/app/config.yaml:ro \
  -v /path/to/data:/data \
  -e JWT__SECRET="your-secret" \
  element-skin-backend:latest

# 前端容器
docker run -d \
  --name element-skin-frontend \
  -p 80:80 \
  -e API_BASE_URL="http://your-backend:8000" \
  element-skin-frontend:latest
```

---

## 📦 生产部署指南

### 传统部署（无 Docker）

#### 后端部署

```bash
cd skin-backend

# 安装依赖
pip install -r requirements.txt

# 生成密钥对
python gen_key.py

# 使用 gunicorn + uvicorn worker（推荐）
pip install gunicorn
gunicorn routes_reference:app \
  -w 4 \
  -k uvicorn.workers.UvicornWorker \
  --bind 0.0.0.0:8000 \
  --access-logfile - \
  --error-logfile -

# 或使用 uvicorn（简单场景）
uvicorn routes_reference:app \
  --host 0.0.0.0 \
  --port 8000 \
  --workers 4
```

#### 前端部署

```bash
cd element-skin

# 设置环境变量（可选）
export VITE_BASE_PATH=/
export VITE_API_BASE=https://api.yourdomain.com

# 构建生产版本
npm run build

# 将 dist/ 目录部署到 Nginx/Apache
```

**子目录部署**：如需将前端部署到子目录（如 `/skin/`），请参阅 [DEPLOYMENT.md](./DEPLOYMENT.md) 的「子目录部署指南」章节。

**Nginx 配置示例（根目录部署）：**
```nginx
server {
    listen 80;
    server_name yourdomain.com;

    # 前端静态文件
    location / {
        root /path/to/element-skin/dist;
        try_files $uri $uri/ /index.html;
    }

    # 后端 API 代理
    location /authserver {
        proxy_pass http://127.0.0.1:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /sessionserver {
        proxy_pass http://127.0.0.1:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location ~ ^/(admin|register|textures|static|api|me|public) {
        proxy_pass http://127.0.0.1:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### 生产环境检查清单

- [ ] 修改 `config.yaml` 中的 `jwt.secret` 为随机强密钥
- [ ] 配置管理面板中的站点 URL 为公网域名
- [ ] 启用 HTTPS（Let's Encrypt 推荐）
- [ ] 配置防火墙规则，仅开放 80/443 端口
- [ ] 设置定期备份数据库和材质文件
- [ ] 配置日志轮转（logrotate）
- [ ] 启用速率限制，防止 API 滥用
- [ ] 为 `private.pem` 设置严格权限（chmod 600）
- [ ] 配置监控与告警（可选）

---

## 🔧 开发指南

### 前端开发

```bash
cd element-skin

# 开发模式（热重载）
npm run dev

# 类型检查
npm run type-check

# 代码检查与修复
npm run lint

# 代码格式化
npm run format

# 构建生产版本
npm run build

# 预览生产构建
npm run preview
```

### 后端开发

```bash
cd skin-backend

# 开发模式（自动重载）
uvicorn routes_reference:app --reload

# 生成新的邀请码
python -c "import uuid; print(uuid.uuid4())"

# 重新生成密钥对
python gen_key.py

# 数据库迁移（手动）
# 修改 database.py 的 init() 方法，然后重启后端
```

### API 测试

访问 `http://localhost:8000/docs` 使用 Swagger UI 测试 API。

常用 API 端点：
- `GET /` — Yggdrasil 元数据
- `POST /authserver/authenticate` — 登录
- `POST /authserver/refresh` — 刷新令牌
- `POST /authserver/validate` — 验证令牌
- `POST /authserver/signout` — 登出
- `GET /sessionserver/session/minecraft/profile/:uuid` — 获取角色信息
- `POST /sessionserver/session/minecraft/join` — 加入服务器
- `GET /textures/:uuid` — 获取材质
- `POST /textures/upload` — 上传材质

---

## 🐛 常见问题

### 1. 材质在客户端不显示

**检查清单：**
- 确认管理面板中的 `site_url` 设置正确（包含协议和端口）
- 检查 `GET /` 返回的 `skinDomains` 是否与客户端请求的域名匹配
- 确认材质 URL 格式正确：`/static/textures/{hash}.png`
- 检查防火墙是否开放了材质文件访问

### 2. 登录后提示 "Invalid token"

**可能原因：**
- `jwt.secret` 被修改导致旧令牌失效
- JWT 过期时间设置过短
- 系统时间不同步

**解决方案：**
```bash
# 清空浏览器 localStorage
# 或在控制台执行：
localStorage.clear()

# 重新登录
```

### 3. 前端显示站点名为默认值

**解决方案：**
1. 登录管理面板
2. 进入 设置 页面
3. 填写站点名称和 URL
4. 点击保存
5. 刷新前端页面（Ctrl+F5 强制刷新）

### 4. Docker 容器无法启动

**检查日志：**
```bash
docker-compose logs backend
docker-compose logs frontend
```

**常见问题：**
- 端口冲突：修改 docker-compose.yml 中的端口映射
- 权限问题：确保挂载目录有正确的读写权限
- 配置错误：检查 config.yaml 语法是否正确

### 5. 上传材质失败

**可能原因：**
- 文件大小超过限制（默认 1MB）
- 图片尺寸不符合要求（64x32 或 64x64）
- 文件格式不正确（必须是 PNG）

**解决方案：**
在管理面板中调整 `skin_max_size` 和 `cape_max_size` 设置。

---

## 📝 更新日志

### v1.0.0 (2025-12-14)
- ✅ 初始版本发布
- ✅ 完整的 Yggdrasil API 实现
- ✅ 现代化 UI 设计
- ✅ Docker 部署支持
- ✅ 速率限制与安全特性

---

## 📄 许可证

本项目采用 MIT 许可证，详见 LICENSE 文件。

---

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

**贡献步骤：**
1. Fork 本仓库
2. 创建特性分支：`git checkout -b feature/amazing-feature`
3. 提交更改：`git commit -m 'Add amazing feature'`
4. 推送分支：`git push origin feature/amazing-feature`
5. 提交 Pull Request

---

## 📮 联系方式

- Issue Tracker: https://github.com/your-repo/element-skin/issues
- Email: your-email@example.com

---

## 🙏 致谢

- [Yggdrasil API 规范](https://github.com/yushijinhun/authlib-injector/wiki)
- [Vue.js](https://vuejs.org/)
- [FastAPI](https://fastapi.tiangolo.com/)
- [Element Plus](https://element-plus.org/)
- [SkinView3D](https://github.com/bs-community/skinview3d)

---

<div align="center">
Made with ❤️ by Element-Skin Team
</div>

