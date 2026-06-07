#!/bin/sh
set -e

mkdir -p /app/frontend/static/textures
mkdir -p /app/frontend/static/carousel

echo "Releasing frontend static files to /app/frontend..."

USER_FAVICON=""
if [ -f "/app/frontend/favicon.ico" ]; then
  USER_FAVICON="$(mktemp)"
  cp -f /app/frontend/favicon.ico "$USER_FAVICON"
fi

if [ -d "/app/frontend" ]; then
  find /app/frontend -mindepth 1 -maxdepth 1 ! -name 'static' ! -name 'favicon.ico' -exec rm -rf {} +
fi

cp -rf /app/frontend_dist/* /app/frontend/

if [ -n "$USER_FAVICON" ]; then
  cp -f "$USER_FAVICON" /app/frontend/favicon.ico
  rm -f "$USER_FAVICON"
fi

BASE_PATH=${VITE_BASE_PATH:-/}
API_BASE=${VITE_API_BASE:-/skinapi}

case "$BASE_PATH" in
  /*) ;;
  *) BASE_PATH="/$BASE_PATH" ;;
esac
case "$BASE_PATH" in
  */) ;;
  *) BASE_PATH="$BASE_PATH/" ;;
esac

echo "Replacing frontend placeholders: BASE=$BASE_PATH, API=$API_BASE"
find /app/frontend -type f \( -name "*.js" -o -name "*.html" \) -exec sed -i "s|/VITE_BASE_PATH_PLACEHOLDER/|$BASE_PATH|g" {} +
find /app/frontend -type f \( -name "*.js" -o -name "*.html" \) -exec sed -i "s|VITE_API_BASE_PLACEHOLDER|$API_BASE|g" {} +

exec "$@"
