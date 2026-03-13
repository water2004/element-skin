#!/bin/bash
set -e

# 1. 确保静态资源子目录存在
# 这些目录位于挂载的 /app/frontend 卷内，会被持久化
mkdir -p /app/frontend/static/textures
mkdir -p /app/frontend/static/carousel

# --- 2. 释放前端编译产物 ---
echo "正在释放前端静态文件到 /app/frontend..."

# 保护 static 目录和 favicon.ico，仅清空其它的前端入口文件（index.html, assets 等）
if [ -d "/app/frontend" ]; then
    find /app/frontend -mindepth 1 -maxdepth 1 ! -name 'static' ! -name 'favicon.ico' -exec rm -rf {} +
fi

# 复制新前端产物，但如果目标已存在 favicon.ico 则跳过它
if [ -f "/app/frontend/favicon.ico" ]; then
    echo "检测到已存在 favicon.ico，跳过覆盖。"
    # 使用 rsync 或排除模式，这里用 cp 配合 find 较复杂，改用更通用的逻辑：
    # 先复制所有，如果之前有备份则还原，或者直接排除
    cp -rf /app/frontend_dist/* /app/frontend/
    # 假设 dist 里也有 favicon.ico，如果想保留旧的，这里可以从某处恢复或在 cp 时排除
    # 简单做法：如果 dist 里的 favicon.ico 和宿主机不一样，且用户想保留宿主机的，
    # 可以在 cp 前备份，cp 后还原，或者使用更精确的 cp 命令。
    # 这里我们采用 cp 完后如果不小心覆盖了，则提示（或者更优雅地在 cp 时排除）
    # 由于标准 cp 不支持简单 exclude，我们先备份现有的 favicon.ico
    mv /app/frontend/favicon.ico /app/frontend/favicon.ico.bak
    cp -rf /app/frontend_dist/* /app/frontend/
    mv /app/frontend/favicon.ico.bak /app/frontend/favicon.ico
else
    cp -rf /app/frontend_dist/* /app/frontend/
fi

# --- 2.5 运行时路径替换 ---
# 获取环境变量并处理默认值
BASE_PATH=${VITE_BASE_PATH:-/}
API_BASE=${VITE_API_BASE:-/skinapi}

# 规范化 BASE_PATH：确保以 / 开头且以 / 结尾
[[ $BASE_PATH != /* ]] && BASE_PATH="/$BASE_PATH"
[[ $BASE_PATH != */ ]] && BASE_PATH="$BASE_PATH/"

echo "正在动态替换前端路径: BASE=$BASE_PATH, API=$API_BASE"

# 扫描并替换 index.html 和 JS 文件中的占位符
# 使用 | 作为 sed 分隔符以处理路径中的斜杠
find /app/frontend -type f \( -name "*.js" -o -name "*.html" \) -exec sed -i "s|/VITE_BASE_PATH_PLACEHOLDER/|$BASE_PATH|g" {} +
find /app/frontend -type f \( -name "*.js" -o -name "*.html" \) -exec sed -i "s|VITE_API_BASE_PLACEHOLDER|$API_BASE|g" {} +

echo "前端文件释放及路径配置完成。"

# --- 3. 密钥生成逻辑 ---
KEY_DIR="/app/data"
mkdir -p "$KEY_DIR"
if [ ! -f "$KEY_DIR/private.pem" ] || [ ! -f "$KEY_DIR/public.pem" ]; then
    echo "密钥文件不存在，正在生成到 $KEY_DIR..."
    python3 gen_key.py "$KEY_DIR"
    echo "密钥已生成。"
else
    echo "密钥文件已存在，跳过生成。"
fi

# 启动应用
exec "$@"
