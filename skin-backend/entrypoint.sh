#!/bin/bash
set -e

# 确保 /data 目录存在
mkdir -p /data/textures
mkdir -p /data/carousel

# 如果密钥文件不存在，则生成
if [ ! -f "/data/private.pem" ] || [ ! -f "/data/public.pem" ]; then
    echo "密钥文件不存在，正在生成..."
    cd /app && python gen_key.py
    mv -f private.pem public.pem /data/
    echo "密钥已生成并保存到 /data/"
else
    echo "密钥文件已存在，跳过生成"
fi

# 启动应用
exec "$@"
