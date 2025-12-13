# Docker 镜像构建与发布指南

本文档说明如何构建、标记和发布 Element-Skin Docker 镜像到容器仓库。

---

## 📦 本地构建

### 构建后端镜像

```bash
cd skin-backend

# 基础构建
docker build -t element-skin-backend:latest .

# 指定标签
docker build -t element-skin-backend:v1.0.0 .

# 多标签构建
docker build -t element-skin-backend:latest -t element-skin-backend:v1.0.0 .
```

### 构建前端镜像

```bash
cd element-skin

# 基础构建
docker build -t element-skin-frontend:latest .

# 带构建参数
docker build \
  --build-arg VITE_BASE_PATH=/ \
  --build-arg VITE_API_BASE=https://api.yourdomain.com \
  -t element-skin-frontend:latest .

# 指定版本标签
docker build -t element-skin-frontend:v1.0.0 .
```

### 测试构建的镜像

```bash
# 测试后端镜像
docker run --rm -p 8000:8000 element-skin-backend:latest

# 测试前端镜像
docker run --rm -p 80:80 element-skin-frontend:latest

# 进入容器调试
docker run --rm -it element-skin-backend:latest sh
```

---

## 🏷️ 镜像标记

### 版本标记策略

```bash
# 主版本标记
docker tag element-skin-backend:latest element-skin-backend:v1
docker tag element-skin-backend:latest element-skin-backend:v1.0
docker tag element-skin-backend:latest element-skin-backend:v1.0.0

# 日期标记
docker tag element-skin-backend:latest element-skin-backend:20250114

# 特性标记
docker tag element-skin-backend:latest element-skin-backend:dev
docker tag element-skin-backend:latest element-skin-backend:staging
docker tag element-skin-backend:latest element-skin-backend:prod
```

---

## 🚀 发布到 Docker Hub

### 1. 登录 Docker Hub

```bash
docker login

# 或指定用户名
docker login -u your-username
```

### 2. 标记镜像

```bash
# 后端镜像
docker tag element-skin-backend:latest your-username/element-skin-backend:latest
docker tag element-skin-backend:latest your-username/element-skin-backend:v1.0.0

# 前端镜像
docker tag element-skin-frontend:latest your-username/element-skin-frontend:latest
docker tag element-skin-frontend:latest your-username/element-skin-frontend:v1.0.0
```

### 3. 推送镜像

```bash
# 推送单个标签
docker push your-username/element-skin-backend:latest

# 推送所有标签
docker push your-username/element-skin-backend --all-tags
docker push your-username/element-skin-frontend --all-tags
```

### 4. 验证发布

访问 https://hub.docker.com/r/your-username/element-skin-backend

---

## 🔐 发布到私有仓库

### 方案一：GitHub Container Registry (GHCR)

#### 1. 创建 Personal Access Token

在 GitHub Settings → Developer settings → Personal access tokens 创建 token，权限：
- `write:packages`
- `read:packages`
- `delete:packages`

#### 2. 登录 GHCR

```bash
echo $GITHUB_TOKEN | docker login ghcr.io -u your-github-username --password-stdin
```

#### 3. 标记并推送

```bash
# 标记镜像
docker tag element-skin-backend:latest ghcr.io/your-username/element-skin-backend:latest
docker tag element-skin-frontend:latest ghcr.io/your-username/element-skin-frontend:latest

# 推送镜像
docker push ghcr.io/your-username/element-skin-backend:latest
docker push ghcr.io/your-username/element-skin-frontend:latest
```

#### 4. 使用 GHCR 镜像

修改 `docker-compose.yml`：

```yaml
services:
  backend:
    image: ghcr.io/your-username/element-skin-backend:latest
    # 不需要 build 部分
    
  frontend:
    image: ghcr.io/your-username/element-skin-frontend:latest
```

### 方案二：阿里云容器镜像服务

#### 1. 登录阿里云

```bash
docker login --username=your-aliyun-account registry.cn-hangzhou.aliyuncs.com
```

#### 2. 标记并推送

```bash
# 标记镜像
docker tag element-skin-backend:latest \
  registry.cn-hangzhou.aliyuncs.com/your-namespace/element-skin-backend:latest

# 推送镜像
docker push registry.cn-hangzhou.aliyuncs.com/your-namespace/element-skin-backend:latest
```

### 方案三：Harbor 私有仓库

#### 1. 登录 Harbor

```bash
docker login harbor.yourdomain.com
```

#### 2. 标记并推送

```bash
docker tag element-skin-backend:latest \
  harbor.yourdomain.com/library/element-skin-backend:latest

docker push harbor.yourdomain.com/library/element-skin-backend:latest
```

---

## 🤖 自动化构建（GitHub Actions）

创建 `.github/workflows/docker-build.yml`：

```yaml
name: Build and Push Docker Images

on:
  push:
    branches: [ main ]
    tags: [ 'v*' ]
  pull_request:
    branches: [ main ]

env:
  REGISTRY: ghcr.io
  BACKEND_IMAGE: ${{ github.repository }}-backend
  FRONTEND_IMAGE: ${{ github.repository }}-frontend

jobs:
  build-backend:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to Container Registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY }}/${{ env.BACKEND_IMAGE }}
          tags: |
            type=ref,event=branch
            type=ref,event=pr
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=semver,pattern={{major}}
            type=sha

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: ./skin-backend
          push: ${{ github.event_name != 'pull_request' }}
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max

  build-frontend:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to Container Registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY }}/${{ env.FRONTEND_IMAGE }}
          tags: |
            type=ref,event=branch
            type=ref,event=pr
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=semver,pattern={{major}}
            type=sha

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: ./element-skin
          push: ${{ github.event_name != 'pull_request' }}
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

---

## 🔍 镜像优化

### 减小镜像大小

#### 1. 使用 .dockerignore

确保 `.dockerignore` 排除不必要的文件：

```
# 后端 .dockerignore
__pycache__/
*.pyc
.git/
.venv/
*.db
textures/

# 前端 .dockerignore
node_modules/
dist/
.git/
coverage/
```

#### 2. 多阶段构建

Dockerfile 已使用多阶段构建，构建阶段不会包含在最终镜像中。

#### 3. 压缩镜像层

```bash
# 使用 docker-slim
docker-slim build element-skin-backend:latest

# 或使用 dive 分析镜像
docker run --rm -it \
  -v /var/run/docker.sock:/var/run/docker.sock \
  wagoodman/dive:latest element-skin-backend:latest
```

### 安全扫描

```bash
# 使用 Trivy 扫描漏洞
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
  aquasec/trivy image element-skin-backend:latest

# 使用 Snyk 扫描
snyk container test element-skin-backend:latest

# 使用 Clair 扫描
docker run -d --name clair-db arminc/clair-db:latest
docker run -p 6060:6060 --link clair-db:postgres -d arminc/clair-local-scan:latest
```

---

## 📊 镜像管理

### 查看本地镜像

```bash
# 列出所有 element-skin 镜像
docker images | grep element-skin

# 查看镜像详情
docker inspect element-skin-backend:latest

# 查看镜像历史
docker history element-skin-backend:latest

# 查看镜像大小
docker images element-skin-backend --format "{{.Repository}}:{{.Tag}} {{.Size}}"
```

### 清理镜像

```bash
# 删除特定镜像
docker rmi element-skin-backend:v1.0.0

# 删除所有未使用的镜像
docker image prune -a

# 删除所有 element-skin 镜像
docker images | grep element-skin | awk '{print $3}' | xargs docker rmi
```

### 导出和导入镜像

```bash
# 导出镜像
docker save element-skin-backend:latest | gzip > element-skin-backend.tar.gz
docker save element-skin-frontend:latest | gzip > element-skin-frontend.tar.gz

# 导入镜像
docker load < element-skin-backend.tar.gz
docker load < element-skin-frontend.tar.gz

# 传输到其他服务器
scp element-skin-backend.tar.gz user@server:/tmp/
ssh user@server "docker load < /tmp/element-skin-backend.tar.gz"
```

---

## 🔄 更新策略

### 滚动更新

```bash
# 1. 构建新镜像
docker compose build

# 2. 逐个重启服务
docker compose up -d --no-deps --build backend
sleep 10
docker compose up -d --no-deps --build frontend

# 3. 验证健康状态
docker compose ps
```

### 蓝绿部署

```bash
# 1. 启动新版本（使用不同端口）
docker run -d --name backend-v2 -p 8001:8000 element-skin-backend:v2.0.0

# 2. 测试新版本
curl http://localhost:8001/

# 3. 切换流量（修改 Nginx 配置）
# upstream backend {
#     server backend-v2:8000;
# }

# 4. 移除旧版本
docker stop backend-v1
docker rm backend-v1
```

---

## 🌍 多架构构建

### 构建多平台镜像

```bash
# 创建构建器
docker buildx create --name multiarch --use

# 构建并推送多架构镜像
docker buildx build \
  --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t your-username/element-skin-backend:latest \
  --push \
  ./skin-backend

# 查看镜像支持的架构
docker buildx imagetools inspect your-username/element-skin-backend:latest
```

---

## 📝 版本发布检查清单

发布新版本前，请确认：

- [ ] 所有测试通过
- [ ] 更新版本号（package.json、pyproject.toml）
- [ ] 更新 CHANGELOG.md
- [ ] 构建并测试镜像
- [ ] 扫描安全漏洞
- [ ] 标记版本（git tag）
- [ ] 推送镜像到仓库
- [ ] 更新文档
- [ ] 发布 GitHub Release

---

## 💡 最佳实践

1. **版本管理**
   - 使用语义化版本（SemVer）
   - 保持 `latest` 标签指向最新稳定版
   - 为每个发布创建版本标签

2. **安全性**
   - 定期更新基础镜像
   - 扫描漏洞并及时修复
   - 不在镜像中包含敏感信息
   - 使用最小权限运行容器

3. **性能优化**
   - 利用构建缓存
   - 合并镜像层
   - 使用 .dockerignore
   - 压缩静态资源

4. **可追溯性**
   - 记录构建时间和构建者
   - 使用 Git SHA 作为镜像标签
   - 保留构建日志

---

## 🆘 常见问题

### Q: 构建速度慢？

A: 使用 BuildKit 和缓存：
```bash
export DOCKER_BUILDKIT=1
docker build --cache-from=your-image:latest .
```

### Q: 镜像过大？

A: 检查镜像层，清理不必要的文件：
```bash
docker history element-skin-backend:latest
```

### Q: 推送失败？

A: 检查登录状态和权限：
```bash
docker login
docker push your-image:tag --debug
```

---

**祝您构建顺利！** 🐳
