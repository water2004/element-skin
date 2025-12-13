# EDTP 皮肤站 — 测试与部署指南

快速指南，帮助在本地开发与简单部署本项目（前端 + 后端）。

**目录结构（相关）**
- `element-skin/` — 前端 (Vue 3 + Vite + Element Plus)
- `skin-backend/` — 后端 (FastAPI)

**前提**
- 安装 Node.js (>=18) 与 npm/yarn
- 安装 Python 3.10+ 和虚拟环境工具

**一键安装（后端）**
```bash
cd skin-backend
python -m venv .venv
.\.venv\Scripts\activate    # Windows
source .venv/bin/activate    # macOS / Linux
pip install -r requirements.txt
```

**运行后端（开发）**
```bash
cd skin-backend
uvicorn routes_reference:app --reload --host 0.0.0.0 --port 8000
```

说明：
- 静态材质目录 `skin-backend/textures/` 会在运行时自动创建。
- 若需要签名 (Yggdrasil)，请使用 `gen_key.py` 生成 `private.pem`/`public.pem`。
- **配置文件**：`skin-backend/config.yaml` 包含所有服务器配置（JWT、速率限制、数据库等）。环境变量可覆盖配置（格式：`JWT__SECRET`）。

**安全特性**
- ✅ **密码加密**：使用 bcrypt 哈希存储，自动升级旧明文密码
- ✅ **速率限制**：防止暴力破解，**可在管理面板实时配置**
- ✅ **文件大小检查**：上传材质自动验证大小限制
- ✅ **配置分离**：基础配置用文件，运营配置在数据库（无需重启）

**配置系统说明**

配置分为两类：

1. **基础配置（config.yaml）** - 需要重启生效
   - `jwt.secret` — JWT 签名密钥（生产必改！）
   - `database.path` — 数据库文件路径
   - `textures.directory` — 材质存储目录
   - `server.host` / `server.port` — 服务器监听地址

2. **运营配置（管理面板 → 设置）** - 实时生效
   - 站点名称、URL
   - JWT 过期时间
   - 速率限制开关、尝试次数、时间窗口
   - 材质大小限制
   - 注册开关、邀请码要求

环境变量可覆盖配置文件（格式：`JWT__SECRET`），但运营配置建议在管理面板修改。

**一键安装（前端）**
```bash
cd element-skin
npm install
npm run dev
```

开发模式下，Vite 配置已把常用后端路由代理到 `http://127.0.0.1:8000`，无需额外跨域设置。

**环境变量（可选）**
- `JWT_SECRET` — JWT 签名密钥（后端），默认开发值已内置，生产请设置强密钥。
- `JWT_EXPIRE_DAYS` — JWT 过期天数。
- `VITE_BASE_PATH` — 前端部署的 base 路径（Vite 的 `base`），若将前端部署到子目录请设置。
- `VITE_API_BASE` — 前端 `axios` 的 baseURL（开发时可留空，Vite proxy 会处理）。

**生产部署建议**
- 使用专用 ASGI 服务器（`uvicorn` 或 `gunicorn -k uvicorn.workers.UvicornWorker`）并在前端或反向代理（Nginx）下提供静态文件。
- 确保 `site_url` 在后台设置为你的公开地址（包含子目录），这样生成的材质 URL 与 `skinDomains` 将匹配客户端。
- 为 `public.pem` / `private.pem` 使用受信任的存储与访问权限。

**常见命令**
- 初始化数据库（首次运行后会自动创建表）: 启动后端即可
- 清理静态材质（手动）: `rm -rf skin-backend/textures/*`

**调试小贴士**
- 若前端左上角站点名未更新：确认在 管理面板 → 设置 中保存了 `site_name`，然后刷新浏览器（硬刷新）。
- 若材质在客户端不显示：检查 `GET /` 返回的 `skinDomains` 与客户端请求的 host 是否匹配；并确认材质 URL 正确（`/static/textures/{hash}.png`）。

