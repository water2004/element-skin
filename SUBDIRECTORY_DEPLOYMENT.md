# 子目录部署快速参考

## 📋 配置清单

### 必须修改的配置

| 配置项 | 位置 | 值 | 说明 |
|-------|------|-----|------|
| `VITE_BASE_PATH` | `.env` | `/skin/` | 前端部署路径，必须以 `/` 开头和结尾 |
| 构建参数 | `docker-compose.yml` | `VITE_BASE_PATH=/skin/` | 传递给前端构建 |
| Nginx 配置 | `config/nginx.conf` | 见下方示例 | 子目录代理配置 |
| 站点 URL | 管理面板 | `http://domain.com/skin` | 不带末尾斜杠 |

### 可选配置

| 配置项 | 默认值 | 何时修改 |
|-------|--------|---------|
| `VITE_API_BASE` | 空 | 仅当后端也在子目录时 |

---

## 🚀 快速部署（3种方法）

### 方法1：一键脚本（最快）

```bash
chmod +x quick-deploy-subdir.sh
./quick-deploy-subdir.sh /skin/
```

### 方法2：手动配置

```bash
# 1. 设置环境变量
echo "VITE_BASE_PATH=/skin/" >> .env

# 2. 修改 docker-compose.yml 的前端构建参数
#    - VITE_BASE_PATH=/skin/

# 3. 使用子目录 Nginx 配置
cp config/nginx-subdir.conf config/nginx.conf

# 4. 构建和启动
docker compose build --no-cache
docker compose up -d
```

### 方法3：传统部署

```bash
# 前端
cd element-skin
export VITE_BASE_PATH=/skin/
npm run build
# 将 dist/ 部署到服务器

# Nginx 配置（见下方）
```

---

## 🌐 Nginx 配置示例

### 最简配置（推荐）

```nginx
server {
    listen 80;
    server_name yourdomain.com;

    # 前端（子目录）
    location /skin/ {
        proxy_pass http://frontend:80/;
        proxy_set_header Host $host;
    }

    # 后端（根路径）
    location ~ ^/(authserver|sessionserver|admin|register|textures|static|api|me|public) {
        proxy_pass http://backend:8000;
        proxy_set_header Host $host;
    }

    # 根路径重定向
    location = / {
        return 302 /skin/;
    }
}
```

### 完整配置（含静态文件部署）

```nginx
server {
    listen 80;
    server_name yourdomain.com;

    # 前端静态文件（alias 方式）
    location /skin/ {
        alias /var/www/element-skin/dist/;
        try_files $uri $uri/ /skin/index.html;
        
        # 静态资源缓存
        location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2)$ {
            expires 1y;
            add_header Cache-Control "public, immutable";
        }
    }

    # 后端 API
    location ~ ^/(authserver|sessionserver|admin|register|textures|static|api|me|public) {
        proxy_pass http://127.0.0.1:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

---

## ✅ 验证步骤

### 1. 检查前端构建产物

```bash
# 查看 index.html 中的资源路径
cat element-skin/dist/index.html | grep -E 'src=|href='

# 应该看到类似：
# <script src="/skin/assets/index-xxx.js">
# <link href="/skin/assets/index-xxx.css">
```

### 2. 测试访问

```bash
# 前端页面
curl http://localhost/skin/
# 应返回 HTML

# 后端 API
curl http://localhost/authserver
# 或
curl http://localhost:8000/authserver
```

### 3. 浏览器测试

1. 访问 `http://your-domain/skin/`
2. 打开浏览器开发者工具（F12）
3. 检查 Network 面板：
   - 静态资源路径应为 `/skin/assets/...`
   - API 请求路径应为 `/authserver/...` 或 `/api/...`
4. 检查 Console 面板，不应有 404 错误

### 4. 管理面板配置

1. 注册账号并登录
2. 进入「管理面板」→「设置」
3. **站点 URL** 设置为：`http://your-domain.com/skin`
   - ⚠️ 注意：不带末尾斜杠
   - ⚠️ 必须与实际访问地址一致
4. 保存后检查 Yggdrasil 元数据：
   ```bash
   curl http://localhost/
   # 检查 skinDomains 和 meta.serverName
   ```

---

## 🐛 常见问题

### Q1: 页面加载但样式丢失

**症状**：页面是白色的，或者样式完全错乱

**原因**：`VITE_BASE_PATH` 未生效或设置错误

**解决**：
```bash
# 1. 确认 .env 文件
cat .env | grep VITE_BASE_PATH
# 应显示：VITE_BASE_PATH=/skin/

# 2. 确认 docker-compose.yml
grep -A5 "VITE_BASE_PATH" docker-compose.yml
# 应在 frontend.build.args 中看到正确值

# 3. 重新构建（必须！）
docker compose build --no-cache frontend
docker compose up -d
```

### Q2: API 请求 404

**症状**：登录、注册等功能不工作，Console 有 404 错误

**原因**：Nginx 后端代理配置不正确

**解决**：
```nginx
# 检查 Nginx 配置，确保后端路径在根目录
location ~ ^/(authserver|sessionserver|...) {
    proxy_pass http://backend:8000;
    # 注意：proxy_pass 末尾没有 /
}
```

### Q3: 路由跳转后 404

**症状**：点击链接跳转后刷新页面出现 404

**原因**：Nginx 缺少 SPA 回退配置

**解决**：
```nginx
location /skin/ {
    # 对于 proxy_pass
    proxy_intercept_errors on;
    error_page 404 = @skin_fallback;
}
location @skin_fallback {
    proxy_pass http://frontend/index.html;
}

# 或对于 alias
location /skin/ {
    alias /path/to/dist/;
    try_files $uri $uri/ /skin/index.html;
}
```

### Q4: Minecraft 客户端无法连接

**症状**：authlib-injector 提示找不到服务器

**原因**：Yggdrasil API 必须在根路径

**解决**：
- ✅ 后端 API 保持在根路径（`/authserver`）
- ✅ 管理面板的站点 URL 设置为根域名
- ❌ 不要将后端也放到子目录

```yaml
# authlib-injector 配置
Yggdrasil 服务器: http://yourdomain.com
# 不是: http://yourdomain.com/skin
```

### Q5: 材质显示不正常

**原因**：站点 URL 配置错误

**解决**：
1. 检查管理面板的「站点 URL」设置
2. 应为：`http://yourdomain.com/skin`（注意无末尾斜杠）
3. 检查 Yggdrasil 元数据：
   ```bash
   curl http://yourdomain.com/ | jq .
   # 检查 skinDomains 和 meta 字段
   ```

---

## 📊 架构对比

### 根目录部署（标准）

```
访问路径：
  前端: http://domain.com/
  后端: http://domain.com/authserver

配置：
  VITE_BASE_PATH: /
  Nginx: location / → frontend
```

### 子目录部署

```
访问路径：
  前端: http://domain.com/skin/
  后端: http://domain.com/authserver  ← 注意：仍在根路径

配置：
  VITE_BASE_PATH: /skin/
  Nginx: location /skin/ → frontend
         location /authserver → backend
```

---

## 💡 最佳实践

1. **前端子目录，后端根路径**（推荐）
   - 前端：`/skin/`
   - 后端：`/authserver`, `/admin` 等
   - 优点：Minecraft 客户端配置简单

2. **构建时设置 base path**
   - 不要在运行时改变 base path
   - 修改后必须重新构建

3. **站点 URL 配置**
   - 管理面板中设置完整 URL
   - 与实际访问地址完全一致
   - 不带末尾斜杠

4. **测试流程**
   - 先测试前端静态资源加载
   - 再测试 API 请求
   - 最后测试 Minecraft 客户端

5. **使用配置文件**
   - 准备好的配置：`config/nginx-subdir.conf`
   - 一键脚本：`quick-deploy-subdir.sh`

---

## 📚 相关文档

- [DEPLOYMENT.md](./DEPLOYMENT.md) - 完整部署指南（含详细子目录部署章节）
- [README.md](./README.md) - 项目介绍和配置说明
- [config/nginx-subdir.conf](./config/nginx-subdir.conf) - 完整 Nginx 配置示例
- [.env.example](./.env.example) - 环境变量模板

---

**提示**：部署前建议先阅读 [DEPLOYMENT.md](./DEPLOYMENT.md) 的"子目录部署指南"章节获取完整信息。
