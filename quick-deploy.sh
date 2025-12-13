#!/bin/bash

# Element-Skin 快速部署脚本
# 使用方法: ./quick-deploy.sh

set -e

echo "======================================"
echo "Element-Skin 快速部署脚本"
echo "======================================"
echo ""

# 检查 Docker
if ! command -v docker &> /dev/null; then
    echo "❌ 未检测到 Docker，请先安装 Docker"
    exit 1
fi

# 检查 Docker Compose
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
echo "JWT_SECRET=$JWT_SECRET" > .env
echo "TZ=Asia/Shanghai" >> .env
echo "LOG_LEVEL=INFO" >> .env
echo "✅ JWT 密钥已生成并保存到 .env"
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
# Element-Skin 配置文件
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

# 询问是否构建镜像
read -p "是否现在构建 Docker 镜像？(y/n) [y]: " BUILD_NOW
BUILD_NOW=${BUILD_NOW:-y}

if [ "$BUILD_NOW" = "y" ] || [ "$BUILD_NOW" = "Y" ]; then
    echo ""
    echo "🔨 开始构建 Docker 镜像..."
    docker compose build
    echo "✅ Docker 镜像构建完成"
    echo ""
    
    # 询问是否启动服务
    read -p "是否现在启动服务？(y/n) [y]: " START_NOW
    START_NOW=${START_NOW:-y}
    
    if [ "$START_NOW" = "y" ] || [ "$START_NOW" = "Y" ]; then
        echo ""
        echo "🚀 启动服务..."
        docker compose up -d
        echo ""
        echo "⏳ 等待服务启动..."
        sleep 10
        
        # 检查服务状态
        echo ""
        echo "📊 服务状态："
        docker compose ps
        
        echo ""
        echo "======================================"
        echo "✅ 部署完成！"
        echo "======================================"
        echo ""
        echo "📍 访问地址："
        echo "   前端: http://localhost/"
        echo "   后端: http://localhost:8000/"
        echo "   API 文档: http://localhost:8000/docs"
        echo ""
        echo "📝 首次使用："
        echo "   1. 访问前端页面"
        echo "   2. 点击「注册」创建账号"
        echo "   3. 第一个注册的用户将自动成为管理员"
        echo "   4. 登录后进入「管理面板」→「设置」配置站点信息"
        echo ""
        echo "🔍 查看日志: docker compose logs -f"
        echo "🛑 停止服务: docker compose down"
        echo "🔄 重启服务: docker compose restart"
        echo ""
    fi
fi

# 保存重要信息
cat > DEPLOYMENT_INFO.txt << EOF
Element-Skin 部署信息
====================

部署时间: $(date)
JWT 密钥: $JWT_SECRET

重要提醒:
1. 请妥善保管 .env 文件和 config/keys/ 目录
2. 首次部署后请立即访问站点并注册管理员账号
3. 在管理面板中配置站点 URL（必须与实际访问地址一致）
4. 建议启用 HTTPS 和速率限制

常用命令:
- 查看日志: docker compose logs -f
- 停止服务: docker compose down
- 启动服务: docker compose up -d
- 重启服务: docker compose restart
- 查看状态: docker compose ps

文档链接:
- 完整文档: README.md
- 部署指南: DEPLOYMENT.md
EOF

echo "💾 部署信息已保存到 DEPLOYMENT_INFO.txt"
echo ""
