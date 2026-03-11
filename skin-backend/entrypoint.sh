#!/bin/bash
set -e

# 1. 确保静态资源子目录存在
# 这些目录位于挂载的 /app/frontend 卷内，会被持久化
mkdir -p /app/frontend/static/textures
mkdir -p /app/frontend/static/carousel

# --- 2. 释放前端编译产物 ---
echo "正在释放前端静态文件到 /app/frontend..."

# 保护 static 目录，仅清空其它的前端入口文件（index.html, assets 等）
if [ -d "/app/frontend" ]; then
    find /app/frontend -mindepth 1 -maxdepth 1 ! -name 'static' -exec rm -rf {} +
fi

# 复制新前端产物
cp -rf /app/frontend_dist/* /app/frontend/
echo "前端文件释放完成。"

# --- 3. 密钥生成逻辑 ---
if [ ! -f "private.pem" ] || [ ! -f "public.pem" ]; then
    echo "密钥文件不存在，正在生成..."
    python3 gen_key.py
    echo "密钥已生成。"
else
    echo "密钥文件已存在，跳过生成。"
fi

# 启动应用
exec "$@"
