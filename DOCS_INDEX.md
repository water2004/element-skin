# 📚 Element-Skin 文档索引

快速导航到您需要的文档。

---

## 🎯 快速开始

| 文档 | 内容 | 适用对象 |
|-----|------|---------|
| [README.md](./README.md) | 项目介绍、快速开始、配置说明 | 所有用户 |
| [quick-deploy.sh](./quick-deploy.sh) / [quick-deploy.bat](./quick-deploy.bat) | 一键部署脚本 | 快速体验 |

---

## 🚀 部署相关

| 文档 | 内容 | 适用场景 |
|-----|------|---------|
| [DEPLOYMENT.md](./DEPLOYMENT.md) | **完整部署指南**（生产环境） | 正式部署 |
| [SUBDIRECTORY_DEPLOYMENT.md](./SUBDIRECTORY_DEPLOYMENT.md) | **子目录部署快速参考** | 子目录部署 |
| [DOCKER_BUILD.md](./DOCKER_BUILD.md) | Docker 镜像构建与发布 | 自定义镜像 |
| [docker-compose.yml](./docker-compose.yml) | 完整 Docker Compose 配置 | 标准部署 |
| [docker-compose.simple.yml](./docker-compose.simple.yml) | 简化版配置 | 快速测试 |

### 部署流程

```
选择部署方式
    ├─ 快速体验 → 运行 quick-deploy.sh/bat
    ├─ 开发环境 → README.md "开发环境搭建"
    ├─ 生产环境（根目录） → DEPLOYMENT.md "Docker 部署"
    ├─ 生产环境（子目录） → SUBDIRECTORY_DEPLOYMENT.md 或 quick-deploy-subdir.sh
    └─ 自定义镜像 → DOCKER_BUILD.md "镜像构建"
```

---

## ⚙️ 配置相关

| 文件 | 说明 | 优先级 |
|-----|------|-------|
| [config/config.yaml](./config/config.yaml) | 基础配置（需重启生效） | 低 |
| [.env](./.env.example) | 环境变量（覆盖 config.yaml） | 中 |
| 管理面板 → 设置 | 运营配置（实时生效） | 高 |

### 配置优先级

```
管理面板设置 > 环境变量 > config.yaml > 默认值
```

---

## 🐳 Docker 相关

### Dockerfile

| 文件 | 说明 |
|-----|------|
| [skin-backend/Dockerfile](./skin-backend/Dockerfile) | 后端镜像构建配置 |
| [element-skin/Dockerfile](./element-skin/Dockerfile) | 前端镜像构建配置 |

### Docker Compose 配置

| 文件 | 说明 | 使用场景 |
|-----|------|---------|
| [docker-compose.yml](./docker-compose.yml) | 完整配置（含网络、资源限制） | 生产环境 |
| [docker-compose.simple.yml](./docker-compose.simple.yml) | 简化配置 | 快速测试 |

### Docker 忽略文件

| 文件 | 说明 |
|-----|------|
| [skin-backend/.dockerignore](./skin-backend/.dockerignore) | 后端镜像构建排除文件 |
| [element-skin/.dockerignore](./element-skin/.dockerignore) | 前端镜像构建排除文件 |

---

## 🔧 Nginx 配置

| 文件 | 说明 |
|-----|------|
| [config/nginx.conf](./config/nginx.conf) | Nginx 反向代理配置示例 |

适用场景：
- 统一入口（前端+后端）
- HTTPS 配置
- 负载均衡
- 静态资源缓存

---

## 📖 其他文档

| 文档 | 内容 |
|-----|------|
| [SUBDIRECTORY_DEPLOYMENT.md](./SUBDIRECTORY_DEPLOYMENT.md) | 子目录部署快速参考 |
| [doc/Yggdrasil-服务端技术规范.md](./doc/Yggdrasil-服务端技术规范.md) | Yggdrasil API 规范 |
| DEPLOYMENT_INFO.txt | 部署信息（自动生成） |

---

## 🎯 按场景查找

### 场景 1: 我是新手，想快速体验

1. ✅ 运行 [quick-deploy.sh](./quick-deploy.sh) 或 [quick-deploy.bat](./quick-deploy.bat)
2. ✅ 访问 http://localhost
3. ✅ 参考 [README.md](./README.md) 的"首次配置"章节

### 场景 2: 我要部署到生产环境

1. ✅ 阅读 [DEPLOYMENT.md](./DEPLOYMENT.md) 完整指南
2. ✅ 准备配置文件：[config/config.yaml](./config/config.yaml) 和 [.env](./.env.example)
3. ✅ 使用 [docker-compose.yml](./docker-compose.yml) 部署
4. ✅ 配置 HTTPS（参考 DEPLOYMENT.md）
5. ✅ 完成"生产环境检查清单"

**特殊场景：子目录部署**
- 如需将前端部署到子目录（如 `/skin/`），参考 [DEPLOYMENT.md](./DEPLOYMENT.md) 的"子目录部署指南"章节
- 或使用快速脚本：`./quick-deploy-subdir.sh /skin/`
- 查看配置示例：[config/nginx-subdir.conf](./config/nginx-subdir.conf)

### 场景 3: 我要自定义镜像

1. ✅ 阅读 [DOCKER_BUILD.md](./DOCKER_BUILD.md)
2. ✅ 修改 [Dockerfile](./skin-backend/Dockerfile)
3. ✅ 构建并测试镜像
4. ✅ 推送到镜像仓库

### 场景 4: 我要本地开发

1. ✅ 参考 [README.md](./README.md) "开发环境搭建"
2. ✅ 安装依赖（npm install + pip install）
3. ✅ 启动开发服务器
4. ✅ 参考"开发指南"章节

### 场景 5: 我要配置 Nginx 反向代理

1. ✅ 复制 [config/nginx.conf](./config/nginx.conf) 到 Nginx 配置目录（根目录部署）
2. ✅ 或使用 [config/nginx-subdir.conf](./config/nginx-subdir.conf)（子目录部署）
3. ✅ 修改域名和上游服务器地址
4. ✅ 配置 SSL 证书（参考 DEPLOYMENT.md）
5. ✅ 重载 Nginx 配置

### 场景 6: 我遇到问题了

1. ✅ 查看 [README.md](./README.md) "常见问题"章节
2. ✅ 查看 [DEPLOYMENT.md](./DEPLOYMENT.md) "故障排查"章节
3. ✅ 检查容器日志：`docker compose logs -f`
4. ✅ 提交 Issue 到 GitHub

---

## 📋 命令速查表

### Docker Compose 常用命令

```bash
# 启动服务
docker compose up -d

# 停止服务
docker compose down

# 重启服务
docker compose restart

# 查看日志
docker compose logs -f

# 查看状态
docker compose ps

# 重新构建
docker compose build --no-cache

# 更新镜像
docker compose pull
```

### 开发命令

```bash
# 后端
cd skin-backend
python -m venv .venv
source .venv/bin/activate  # Windows: .venv\Scripts\activate
pip install -r requirements.txt
uvicorn routes_reference:app --reload

# 前端
cd element-skin
npm install
npm run dev
npm run build
```

---

## 🔗 外部资源

- [Vue.js 官方文档](https://vuejs.org/)
- [FastAPI 官方文档](https://fastapi.tiangolo.com/)
- [Element Plus 文档](https://element-plus.org/)
- [Docker 官方文档](https://docs.docker.com/)
- [Yggdrasil API 规范](https://github.com/yushijinhun/authlib-injector/wiki)

---

## 📞 获取帮助

- 📖 查看文档：从上方索引找到对应文档
- 🐛 报告问题：[GitHub Issues](https://github.com/your-repo/element-skin/issues)
- 💬 讨论交流：[GitHub Discussions](https://github.com/your-repo/element-skin/discussions)

---

**祝您使用愉快！** 🎉
