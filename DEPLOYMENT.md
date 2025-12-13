# Element-Skin Docker 部署完整指南

本文档提供 Element-Skin 的完整 Docker 部署步骤，适用于生产环境。

---

## 📋 部署前准备

### 系统要求

- **操作系统**: Linux (推荐 Ubuntu 20.04+/Debian 11+/CentOS 8+) 或 Windows Server 2019+
- **Docker**: >= 20.10
- **Docker Compose**: >= 2.0
- **磁盘空间**: 至少 10GB 可用空间
- **内存**: 至少 2GB RAM（推荐 4GB）
- **CPU**: 至少 2 核心

### 安装 Docker 和 Docker Compose

#### Ubuntu/Debian

```bash
# 更新软件包索引
sudo apt-get update

# 安装依赖
sudo apt-get install -y ca-certificates curl gnupg lsb-release

# 添加 Docker 官方 GPG 密钥
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | \
  sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg

# 设置 Docker 仓库
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
  https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# 安装 Docker Engine
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

# 启动 Docker 服务
sudo systemctl start docker
sudo systemctl enable docker

# 验证安装
docker --version
docker compose version
```

#### CentOS/RHEL

```bash
# 安装依赖
sudo yum install -y yum-utils

# 添加 Docker 仓库
sudo yum-config-manager --add-repo \
  https://download.docker.com/linux/centos/docker-ce.repo

# 安装 Docker Engine
sudo yum install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

# 启动 Docker 服务
sudo systemctl start docker
sudo systemctl enable docker

# 验证安装
docker --version
docker compose version
```

---

## 🚀 快速部署（推荐）

### 步骤 1: 克隆项目

```bash
# 克隆仓库
git clone https://github.com/your-repo/element-skin.git
cd element-skin

# 或下载发布包
wget https://github.com/your-repo/element-skin/archive/refs/tags/v1.0.0.tar.gz
tar -xzf v1.0.0.tar.gz
cd element-skin-1.0.0
```

### 步骤 2: 创建配置目录结构

```bash
# 创建必要的目录
mkdir -p config/keys data logs

# 设置权限
chmod 755 config data logs
```

### 步骤 3: 配置后端

#### 生成 RSA 密钥对

```bash
cd skin-backend

# 方法1：使用 Python 脚本生成
python gen_key.py
mv private.pem public.pem ../config/keys/

# 方法2：使用 OpenSSL 生成
openssl genrsa -out ../config/keys/private.pem 4096
openssl rsa -in ../config/keys/private.pem -pubout -out ../config/keys/public.pem

cd ..
```

#### 创建配置文件

创建 `config/config.yaml`：

```yaml
# Element-Skin 后端配置
jwt:
  secret: "CHANGE-THIS-TO-A-RANDOM-SECRET-KEY"  # ⚠️ 务必修改！

database:
  path: "/data/yggdrasil.db"

textures:
  directory: "/data/textures"

server:
  host: "0.0.0.0"
  port: 8000
```

**重要**: 修改 `jwt.secret` 为随机强密钥：

```bash
# 生成随机密钥
openssl rand -base64 32
# 或
python -c "import secrets; print(secrets.token_urlsafe(32))"
```

### 步骤 4: 配置环境变量

```bash
# 复制环境变量模板
cp .env.example .env

# 编辑 .env 文件
nano .env  # 或使用 vim/vi
```

修改 `.env` 中的关键配置：

```bash
JWT_SECRET=your-generated-random-secret-key
TZ=Asia/Shanghai
LOG_LEVEL=INFO
```

### 步骤 5: 构建并启动服务

#### 方式一：使用 Docker Compose（推荐）

```bash
# 构建镜像
docker compose build

# 启动服务（后台运行）
docker compose up -d

# 查看日志
docker compose logs -f

# 查看服务状态
docker compose ps

# 停止服务
docker compose down

# 重启服务
docker compose restart
```

#### 方式二：分步构建

```bash
# 构建后端镜像
cd skin-backend
docker build -t element-skin-backend:latest .
cd ..

# 构建前端镜像
cd element-skin
docker build -t element-skin-frontend:latest .
cd ..

# 创建网络
docker network create element-skin-network

# 启动后端
docker run -d \
  --name element-skin-backend \
  --network element-skin-network \
  -p 8000:8000 \
  -v $(pwd)/config/config.yaml:/app/config.yaml:ro \
  -v $(pwd)/config/keys:/app/keys:ro \
  -v $(pwd)/data:/data \
  --env-file .env \
  element-skin-backend:latest

# 启动前端
docker run -d \
  --name element-skin-frontend \
  --network element-skin-network \
  -p 80:80 \
  element-skin-frontend:latest
```

### 步骤 6: 验证部署

```bash
# 检查容器状态
docker compose ps

# 应显示类似输出：
# NAME                       STATUS        PORTS
# element-skin-backend       Up (healthy)  0.0.0.0:8000->8000/tcp
# element-skin-frontend      Up (healthy)  0.0.0.0:80->80/tcp

# 测试后端 API
curl http://localhost:8000/

# 应返回 Yggdrasil 元数据 JSON

# 测试前端
curl http://localhost/

# 应返回 HTML 页面
```

### 步骤 7: 首次配置

1. 访问 `http://your-server-ip/`
2. 点击右上角「注册」
3. 注册第一个账号（自动成为管理员）
4. 登录后进入「管理面板」→「设置」
5. 配置以下关键项：
   - **站点名称**: 你的皮肤站名称
   - **站点 URL**: `http://your-domain.com`（必须与实际访问地址一致！）
   - **材质大小限制**: 根据需求调整
   - **速率限制**: 建议开启，防止滥用
6. 保存配置

---

## 🎯 子目录部署指南

如果您需要将 Element-Skin 部署在网站的子目录下（例如 `http://yourdomain.com/skin/`），请遵循以下步骤。

### 为什么需要子目录部署？

常见场景：
- 在同一域名下运行多个应用
- 与现有网站集成
- 使用统一的 Nginx 入口管理多个服务

### 配置步骤

#### 方案一：Docker Compose 子目录部署

##### 1. 修改环境变量

编辑 `.env` 文件，设置基础路径：

```bash
# 子目录路径（必须以 / 开头和结尾）
VITE_BASE_PATH=/skin/

# API 基础 URL（如果后端也在子目录，则需要配置）
VITE_API_BASE=/skin

# 其他配置...
JWT_SECRET=your-secret-key
```

**重要**: 
- `VITE_BASE_PATH` 必须以 `/` 开头和结尾，如 `/skin/`
- 如果后端也需要部署在子目录，需要同时配置 Nginx 代理

##### 2. 修改 docker-compose.yml

```yaml
services:
  frontend:
    build:
      context: ./element-skin
      dockerfile: Dockerfile
      args:
        # 传递基础路径到构建阶段
        - VITE_BASE_PATH=/skin/
        - VITE_API_BASE=/skin
    environment:
      # 也可以通过环境变量传递
      - VITE_BASE_PATH=/skin/
    # ... 其他配置
```

##### 3. 配置 Nginx 反向代理

创建或修改 `config/nginx.conf`：

```nginx
server {
    listen 80;
    server_name yourdomain.com;

    # 根路径（可以是其他应用）
    location / {
        root /var/www/html;
        index index.html;
    }

    # 皮肤站前端（子目录）
    location /skin/ {
        proxy_pass http://frontend:80/;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # 重要：处理子目录路由
        proxy_redirect off;
    }

    # 后端 API（保持在根路径或子路径）
    # 方式1：后端在根路径（推荐）
    location ~ ^/(authserver|sessionserver|admin|register|textures|static|api|me|public) {
        proxy_pass http://backend:8000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # 方式2：后端也在子路径（如需要）
    # location /skin/api/ {
    #     rewrite ^/skin/api/(.*) /$1 break;
    #     proxy_pass http://backend:8000;
    #     # ... 其他代理设置
    # }
}
```

**关键配置说明**：

1. **前端子路径代理**：
   - `location /skin/` 匹配前端请求
   - `proxy_pass http://frontend:80/` 注意末尾的 `/`
   - 这样会将 `/skin/` 映射到容器内的 `/`

2. **后端 API 路径**：
   - **推荐**：后端 API 保持在根路径（如 `/authserver`）
   - 前端通过 `VITE_API_BASE` 配置 API 前缀
   - Minecraft 客户端直接访问根路径 API

##### 4. 修改 docker-compose.yml 端口配置

如果使用 Nginx 统一入口：

```yaml
services:
  backend:
    ports:
      - "8000:8000"  # 保持不变，供 Nginx 内部访问

  frontend:
    # 不直接暴露端口，仅供 Nginx 访问
    # ports:
    #   - "80:80"
    expose:
      - "80"

  nginx:
    image: nginx:1.25-alpine
    container_name: element-skin-nginx
    ports:
      - "80:80"      # 统一入口
      - "443:443"    # HTTPS
    volumes:
      - ./config/nginx.conf:/etc/nginx/conf.d/default.conf:ro
    depends_on:
      - frontend
      - backend
    networks:
      - element-skin-network
```

##### 5. 重新构建和启动

```bash
# 停止现有服务
docker compose down

# 重新构建（必须，因为 base path 是构建时设置的）
docker compose build --no-cache

# 启动服务
docker compose up -d

# 检查状态
docker compose ps
docker compose logs -f
```

##### 6. 配置站点 URL

访问 `http://yourdomain.com/skin/`，登录管理员账号：

1. 进入「管理面板」→「设置」
2. **站点 URL** 设置为：`http://yourdomain.com/skin`（注意：不带末尾斜杠）
3. 保存配置

**这一步非常重要！** 站点 URL 必须与实际访问路径一致，否则：
- 材质 URL 会错误
- Yggdrasil API 元数据会不正确
- Minecraft 客户端无法正常工作

#### 方案二：传统部署 + Nginx 子目录

##### 1. 构建前端（带基础路径）

```bash
cd element-skin

# 设置环境变量
export VITE_BASE_PATH=/skin/
export VITE_API_BASE=

# 构建
npm run build

# 构建产物在 dist/ 目录
```

##### 2. 配置 Nginx

```nginx
server {
    listen 80;
    server_name yourdomain.com;
    root /var/www;

    # 前端静态文件（子目录）
    location /skin/ {
        alias /var/www/element-skin/dist/;
        try_files $uri $uri/ /skin/index.html;
        
        # 静态资源缓存
        location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
            expires 1y;
            add_header Cache-Control "public, immutable";
        }
    }

    # 后端 API（根路径）
    location ~ ^/(authserver|sessionserver|admin|register|textures|static|api|me|public) {
        proxy_pass http://127.0.0.1:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

**注意**：使用 `alias` 而不是 `root`：
- `alias /path/to/dist/` - 将 `/skin/` 映射到 `dist/` 目录
- `try_files` 中的路径要包含 `/skin/` 前缀

##### 3. 重启 Nginx

```bash
# 测试配置
sudo nginx -t

# 重启
sudo systemctl reload nginx
```

### 常见问题

#### Q1: 子目录部署后，页面样式丢失

**原因**: `VITE_BASE_PATH` 设置错误或未生效。

**解决**:
```bash
# 检查构建产物中的路径
cat dist/index.html | grep -E 'src=|href='
# 应该看到类似 /skin/assets/... 的路径

# 如果路径不对，重新构建
export VITE_BASE_PATH=/skin/
npm run build
```

#### Q2: API 请求 404

**原因**: 后端 API 路径配置不匹配。

**解决**:
1. 检查浏览器开发者工具 Network 面板，查看请求的完整 URL
2. 确认 Nginx 配置中后端代理路径正确
3. 如果前端配置了 `VITE_API_BASE=/skin`，确保后端也能在该路径访问

**推荐配置**：
- 前端：`/skin/` 子目录
- 后端：`/` 根路径
- 前端不设置 `VITE_API_BASE`，让 API 请求直接发送到根路径

#### Q3: 路由跳转后页面 404

**原因**: Nginx 未正确配置 SPA 路由回退。

**解决**:
```nginx
location /skin/ {
    alias /path/to/dist/;
    # 关键：所有未匹配的请求都返回 index.html
    try_files $uri $uri/ /skin/index.html;
}
```

#### Q4: Minecraft 客户端连接失败

**原因**: Yggdrasil API 必须在根路径，不能在子目录。

**解决**:
- 后端 API 始终保持在根路径（如 `/authserver`）
- 在 Nginx 中不要将后端 API 也放到子目录
- 站点 URL 设置为根域名，而不是子目录

```yaml
# authlib-injector 配置示例
Yggdrasil 服务器: http://yourdomain.com
# 不是: http://yourdomain.com/skin
```

#### Q5: 材质显示不正常

**检查清单**:
1. 管理面板中的「站点 URL」是否正确
2. 检查 `GET /` API 返回的 `skinDomains`
3. 确认材质 URL 格式：`http://yourdomain.com/static/textures/xxx.png`
4. 材质路径不应包含 `/skin/` 前缀

### 推荐架构

**最佳实践**：

```
域名结构：
├── http://yourdomain.com/           → 其他应用或主站
├── http://yourdomain.com/skin/      → Element-Skin 前端
├── http://yourdomain.com/authserver → Yggdrasil API（后端）
├── http://yourdomain.com/admin      → 管理 API（后端）
└── http://yourdomain.com/static     → 材质文件（后端）

Nginx 配置：
├── location /skin/          → 前端容器
├── location /authserver     → 后端容器
├── location /sessionserver  → 后端容器
├── location /admin          → 后端容器
├── location /static         → 后端容器
└── location /               → 其他应用
```

**优势**：
- 前端和后端路径清晰分离
- Minecraft 客户端配置简单
- 材质 URL 无歧义
- 易于维护和调试

### 完整示例

#### docker-compose.yml（子目录部署）

```yaml
version: '3.8'

networks:
  element-skin-network:
    driver: bridge

services:
  backend:
    build: ./skin-backend
    environment:
      - JWT__SECRET=${JWT_SECRET}
      - DATABASE__PATH=/data/yggdrasil.db
    volumes:
      - ./config/config.yaml:/app/config.yaml:ro
      - ./data:/data
    networks:
      - element-skin-network
    expose:
      - "8000"

  frontend:
    build:
      context: ./element-skin
      args:
        - VITE_BASE_PATH=/skin/
        - VITE_API_BASE=
    networks:
      - element-skin-network
    expose:
      - "80"
    depends_on:
      - backend

  nginx:
    image: nginx:1.25-alpine
    ports:
      - "80:80"
    volumes:
      - ./config/nginx-subdir.conf:/etc/nginx/conf.d/default.conf:ro
    networks:
      - element-skin-network
    depends_on:
      - frontend
      - backend
```

#### nginx-subdir.conf

```nginx
server {
    listen 80;
    server_name _;

    # 前端（子目录）
    location /skin/ {
        proxy_pass http://frontend:80/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # 后端 API（根路径）
    location ~ ^/(authserver|sessionserver|admin|register|textures|static|api|me|public) {
        proxy_pass http://backend:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # 根路径（可选，其他应用）
    location / {
        return 200 "Welcome to Element-Skin Server\nFrontend: /skin/\nAPI: /authserver";
        add_header Content-Type text/plain;
    }
}
```

#### 部署命令

```bash
# 1. 设置环境变量
echo "VITE_BASE_PATH=/skin/" >> .env
echo "JWT_SECRET=$(openssl rand -base64 32)" >> .env

# 2. 创建配置
mkdir -p config data
cp config/nginx.conf config/nginx-subdir.conf
# 编辑 nginx-subdir.conf

# 3. 构建和启动
docker compose build --no-cache
docker compose up -d

# 4. 验证
curl http://localhost/           # 根路径
curl http://localhost/skin/      # 前端
curl http://localhost/authserver # 后端 API

# 5. 配置站点
# 访问 http://localhost/skin/
# 在管理面板设置站点 URL 为: http://your-domain.com/skin
```

---

## 🔧 进阶配置

### 使用 Nginx 反向代理（统一入口）

如需统一入口和 HTTPS 支持，可使用 Nginx 反向代理：

#### 1. 修改 docker-compose.yml

取消注释 `nginx` 服务部分，并修改端口配置：

```yaml
services:
  frontend:
    ports:
      - "3000:80"  # 改为非 80 端口

  nginx:
    # 取消注释此服务
    ports:
      - "80:80"
      - "443:443"
```

#### 2. 配置 SSL 证书（HTTPS）

```bash
# 使用 Let's Encrypt 获取证书
sudo apt-get install certbot

# 获取证书（替换为你的域名）
sudo certbot certonly --standalone -d yourdomain.com

# 复制证书到项目目录
sudo mkdir -p config/ssl
sudo cp /etc/letsencrypt/live/yourdomain.com/fullchain.pem config/ssl/cert.pem
sudo cp /etc/letsencrypt/live/yourdomain.com/privkey.pem config/ssl/key.pem
sudo chown -R $USER:$USER config/ssl
```

#### 3. 修改 Nginx 配置

编辑 `config/nginx.conf`，取消注释 SSL 相关配置：

```nginx
server {
    listen 443 ssl http2;
    server_name yourdomain.com;
    
    ssl_certificate /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    
    # ... 其他配置
}

# HTTP 重定向到 HTTPS
server {
    listen 80;
    server_name yourdomain.com;
    return 301 https://$server_name$request_uri;
}
```

#### 4. 重启服务

```bash
docker compose restart nginx
```

### 数据备份策略

#### 自动备份脚本

创建 `backup.sh`：

```bash
#!/bin/bash

# 配置
BACKUP_DIR="/backup/element-skin"
DATE=$(date +%Y%m%d_%H%M%S)
PROJECT_DIR="/path/to/element-skin"

# 创建备份目录
mkdir -p "$BACKUP_DIR"

# 备份数据库
cp "$PROJECT_DIR/data/yggdrasil.db" "$BACKUP_DIR/yggdrasil_$DATE.db"

# 备份材质文件（可选，材质较大可跳过）
tar -czf "$BACKUP_DIR/textures_$DATE.tar.gz" -C "$PROJECT_DIR/data" textures

# 删除 30 天前的备份
find "$BACKUP_DIR" -type f -mtime +30 -delete

echo "Backup completed: $DATE"
```

#### 设置定时任务

```bash
# 编辑 crontab
crontab -e

# 添加每日凌晨 2 点备份
0 2 * * * /path/to/backup.sh >> /var/log/element-skin-backup.log 2>&1
```

### 日志管理

#### 查看日志

```bash
# 实时查看所有服务日志
docker compose logs -f

# 查看特定服务日志
docker compose logs -f backend
docker compose logs -f frontend

# 查看最近 100 行日志
docker compose logs --tail=100 backend
```

#### 配置日志轮转

创建 `/etc/logrotate.d/element-skin`：

```
/path/to/element-skin/logs/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 0644 root root
    postrotate
        docker compose restart backend frontend
    endscript
}
```

### 性能优化

#### 调整资源限制

编辑 `docker-compose.yml` 中的 `deploy.resources` 配置：

```yaml
services:
  backend:
    deploy:
      resources:
        limits:
          cpus: '2.0'      # 根据服务器配置调整
          memory: 1G       # 根据用户量调整
        reservations:
          cpus: '1.0'
          memory: 512M
```

#### 启用数据库优化

SQLite 优化（在后端代码中配置）：

```python
# database.py
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA cache_size=-64000;  # 64MB
```

---

## 🛡️ 安全加固

### 1. 防火墙配置

```bash
# UFW (Ubuntu)
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 22/tcp  # SSH
sudo ufw enable

# Firewalld (CentOS)
sudo firewall-cmd --permanent --add-service=http
sudo firewall-cmd --permanent --add-service=https
sudo firewall-cmd --reload
```

### 2. 文件权限

```bash
# 限制敏感文件权限
chmod 600 config/config.yaml
chmod 600 config/keys/private.pem
chmod 644 config/keys/public.pem
chmod 700 data
```

### 3. 容器安全

```bash
# 定期更新基础镜像
docker compose pull
docker compose up -d

# 扫描镜像漏洞（安装 Trivy）
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
  aquasec/trivy image element-skin-backend:latest
```

### 4. 启用速率限制

在管理面板中：
- 登录尝试: 5 次/小时
- API 请求: 100 次/分钟
- 上传材质: 10 次/小时

---

## 📊 监控与维护

### 健康检查

```bash
# 检查服务健康状态
docker inspect --format='{{.State.Health.Status}}' element-skin-backend
docker inspect --format='{{.State.Health.Status}}' element-skin-frontend

# 查看资源使用
docker stats element-skin-backend element-skin-frontend
```

### 常用维护命令

```bash
# 重启服务
docker compose restart

# 查看容器详情
docker inspect element-skin-backend

# 进入容器内部（调试用）
docker exec -it element-skin-backend sh

# 清理未使用的镜像和容器
docker system prune -a

# 查看磁盘使用
du -sh data/
```

---

## 🔄 更新升级

### 更新到新版本

```bash
# 1. 备份数据
./backup.sh

# 2. 拉取最新代码
git pull origin main

# 3. 重新构建镜像
docker compose build --no-cache

# 4. 停止旧容器
docker compose down

# 5. 启动新容器
docker compose up -d

# 6. 验证更新
docker compose ps
docker compose logs -f
```

### 回滚到旧版本

```bash
# 1. 停止服务
docker compose down

# 2. 切换到旧版本
git checkout v1.0.0

# 3. 恢复备份（如有必要）
cp /backup/element-skin/yggdrasil_20250114.db data/yggdrasil.db

# 4. 启动服务
docker compose up -d
```

---

## 🐛 故障排查

### 容器无法启动

```bash
# 查看详细日志
docker compose logs backend

# 常见原因：
# 1. 端口被占用 → 修改 docker-compose.yml 端口映射
# 2. 权限问题 → 检查挂载目录权限
# 3. 配置错误 → 检查 config.yaml 语法
```

### 前端无法访问后端

```bash
# 检查网络连接
docker compose exec frontend ping backend

# 检查后端健康状态
curl http://localhost:8000/

# 检查防火墙规则
sudo ufw status
```

### 材质上传失败

```bash
# 检查材质目录权限
ls -la data/textures/

# 检查磁盘空间
df -h

# 检查日志
docker compose logs backend | grep texture
```

---

## 📞 获取帮助

如遇到问题，请按以下顺序排查：

1. 查看 [常见问题文档](README.md#常见问题)
2. 搜索 [GitHub Issues](https://github.com/your-repo/element-skin/issues)
3. 查看容器日志：`docker compose logs -f`
4. 提交新的 Issue 并附上：
   - 操作系统和 Docker 版本
   - 完整的错误日志
   - 复现步骤

---

## ✅ 部署检查清单

部署完成后，请确认以下各项：

- [ ] 后端容器运行正常（`docker compose ps`）
- [ ] 前端容器运行正常
- [ ] 可以访问前端页面 `http://your-domain/`
- [ ] 可以访问后端 API `http://your-domain:8000/docs`
- [ ] 已修改 `jwt.secret` 为随机密钥
- [ ] 已在管理面板配置站点 URL
- [ ] 已启用速率限制
- [ ] 已配置防火墙规则
- [ ] 已设置定期备份
- [ ] 已启用 HTTPS（推荐）
- [ ] 已测试材质上传和显示
- [ ] 已测试 Minecraft 客户端连接

---

**祝您部署顺利！** 🎉
