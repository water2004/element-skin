#!/bin/bash

# Element-Skin 子目录部署脚本
# 使用方法: ./quick-deploy-subdir.sh /skin/

set -e

# 检查参数
if [ -z "$1" ]; then
    echo "请指定子目录路径（如 /skin/）"
    echo "用法: ./quick-deploy-subdir.sh /skin/"
    exit 1
fi

SUBDIR_PATH="$1"

# 验证路径格式
if [[ ! "$SUBDIR_PATH" =~ ^/.*/ ]]; then
    echo "❌ 错误：路径必须以 / 开头和结尾，如：/skin/"
    exit 1
fi

echo "======================================"
echo "Element-Skin 子目录部署脚本"
echo "部署路径: $SUBDIR_PATH"
echo "======================================"
echo ""

# 检查 Docker
if ! command -v docker &> /dev/null; then
    echo "❌ 未检测到 Docker，请先安装 Docker"
    exit 1
fi

if ! docker compose version &> /dev/null; then
    echo "❌ 未检测到 Docker Compose，请先安装"
    exit 1
fi

echo "✅ Docker 环境检查通过"
echo ""

# 创建目录结构
echo "📁 创建目录结构..."
mkdir -p config/keys data logs
chmod 755 config data logs

# 生成 JWT 密钥
echo "🔑 生成 JWT 密钥..."
JWT_SECRET=$(openssl rand -base64 32 2>/dev/null || python3 -c "import secrets; print(secrets.token_urlsafe(32))")
cat > .env << EOF
JWT_SECRET=$JWT_SECRET
TZ=Asia/Shanghai
LOG_LEVEL=INFO

# 子目录部署配置
VITE_BASE_PATH=$SUBDIR_PATH
VITE_API_BASE=
EOF
echo "✅ JWT 密钥已生成并保存到 .env"
echo "✅ 前端部署路径设置为: $SUBDIR_PATH"
echo ""

# 生成 RSA 密钥对
echo "🔐 生成 RSA 密钥对..."
if [ ! -f "config/keys/private.pem" ]; then
    cd skin-backend
    python3 gen_key.py
    mv private.pem public.pem ../config/keys/ 2>/dev/null || true
    cd ..
    
    if [ -f "config/keys/private.pem" ]; then
        echo "✅ RSA 密钥对已生成"
    else
        echo "⚠️  RSA 密钥生成失败，将在容器启动时自动生成"
    fi
else
    echo "✅ RSA 密钥对已存在"
fi
echo ""

# 创建配置文件
echo "⚙️  创建配置文件..."
if [ ! -f "config/config.yaml" ]; then
    cat > config/config.yaml << EOF
jwt:
  secret: "$JWT_SECRET"

database:
  path: "/data/yggdrasil.db"

textures:
  directory: "/data/textures"

server:
  host: "0.0.0.0"
  port: 8000
EOF
    echo "✅ 配置文件已创建"
else
    echo "✅ 配置文件已存在"
fi
echo ""

# 创建 Nginx 子目录配置
echo "🌐 创建 Nginx 子目录配置..."
SUBDIR_NAME=$(echo "$SUBDIR_PATH" | sed 's/\///g')
cat > config/nginx-custom.conf << 'NGINX_EOF'
server {
    listen 80;
    server_name _;

    # 前端（子目录）
    location SUBDIR_PATH_PLACEHOLDER {
        proxy_pass http://frontend/;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_redirect off;
    }

    # 后端 API（根路径）
    location ~ ^/(authserver|sessionserver|admin|register|textures|static|api|me|public|docs|openapi.json) {
        proxy_pass http://backend:8000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # 根路径重定向到前端
    location = / {
        return 302 SUBDIR_PATH_PLACEHOLDER;
    }
}
NGINX_EOF

# 替换占位符
sed -i "s|SUBDIR_PATH_PLACEHOLDER|$SUBDIR_PATH|g" config/nginx-custom.conf
echo "✅ Nginx 配置已创建: config/nginx-custom.conf"
echo ""

# 修改 docker-compose.yml
echo "📝 修改 docker-compose.yml..."
if [ -f "docker-compose.yml" ]; then
    # 备份原文件
    cp docker-compose.yml docker-compose.yml.backup
    
    # 使用 sed 修改前端构建参数
    sed -i "/VITE_BASE_PATH/c\        - VITE_BASE_PATH=$SUBDIR_PATH" docker-compose.yml
    
    echo "✅ docker-compose.yml 已更新"
    echo "   备份文件: docker-compose.yml.backup"
else
    echo "⚠️  未找到 docker-compose.yml"
fi
echo ""

# 询问是否构建镜像
read -p "是否现在构建 Docker 镜像？(y/n) [y]: " BUILD_NOW
BUILD_NOW=${BUILD_NOW:-y}

if [ "$BUILD_NOW" = "y" ] || [ "$BUILD_NOW" = "Y" ]; then
    echo ""
    echo "🔨 开始构建 Docker 镜像..."
    docker compose build --no-cache
    echo "✅ Docker 镜像构建完成"
    echo ""
    
    read -p "是否现在启动服务？(y/n) [y]: " START_NOW
    START_NOW=${START_NOW:-y}
    
    if [ "$START_NOW" = "y" ] || [ "$START_NOW" = "Y" ]; then
        echo ""
        echo "🚀 启动服务..."
        docker compose up -d
        echo ""
        echo "⏳ 等待服务启动..."
        sleep 10
        
        echo ""
        echo "📊 服务状态："
        docker compose ps
        
        echo ""
        echo "======================================"
        echo "✅ 部署完成！"
        echo "======================================"
        echo ""
        echo "📍 访问地址："
        echo "   前端: http://localhost$SUBDIR_PATH"
        echo "   后端: http://localhost:8000/"
        echo "   API 文档: http://localhost:8000/docs"
        echo ""
        echo "📝 重要配置："
        echo "   1. 访问前端: http://localhost$SUBDIR_PATH"
        echo "   2. 注册第一个账号（自动成为管理员）"
        echo "   3. 登录后进入「管理面板」→「设置」"
        echo "   4. 设置站点 URL 为: http://your-domain.com$(echo $SUBDIR_PATH | sed 's/\/$//')"
        echo "      （注意：不带末尾斜杠）"
        echo ""
        echo "🔍 常用命令:"
        echo "   查看日志: docker compose logs -f"
        echo "   停止服务: docker compose down"
        echo "   重启服务: docker compose restart"
        echo ""
    fi
fi

# 保存部署信息
cat > DEPLOYMENT_INFO.txt << EOF
Element-Skin 子目录部署信息
========================

部署时间: $(date)
部署路径: $SUBDIR_PATH
JWT 密钥: $JWT_SECRET

访问地址:
- 前端: http://your-domain.com$SUBDIR_PATH
- 后端: http://your-domain.com/authserver
- API 文档: http://your-domain.com/docs

管理面板配置:
- 站点 URL: http://your-domain.com$(echo $SUBDIR_PATH | sed 's/\/$//')
  （重要：必须与实际访问地址一致，不带末尾斜杠）

Nginx 配置:
- 已创建: config/nginx-custom.conf
- 如使用自定义 Nginx，请参考该配置文件

常用命令:
- 查看日志: docker compose logs -f
- 停止服务: docker compose down
- 启动服务: docker compose up -d
- 重启服务: docker compose restart
- 重新构建: docker compose build --no-cache

故障排查:
- 如样式丢失，检查 VITE_BASE_PATH 是否正确
- 如 API 404，检查 Nginx 配置的后端路径
- 如路由跳转 404，检查 Nginx 的 try_files 配置
- 如材质不显示，检查管理面板的站点 URL 设置

文档链接:
- 完整文档: README.md
- 部署指南: DEPLOYMENT.md（子目录部署指南章节）
EOF

echo "💾 部署信息已保存到 DEPLOYMENT_INFO.txt"
echo ""
