@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

echo ======================================
echo Element-Skin 快速部署脚本 (Windows)
echo ======================================
echo.

REM 检查 Docker
docker --version >nul 2>&1
if errorlevel 1 (
    echo ❌ 未检测到 Docker，请先安装 Docker Desktop
    pause
    exit /b 1
)

REM 检查 Docker Compose
docker compose version >nul 2>&1
if errorlevel 1 (
    echo ❌ 未检测到 Docker Compose，请更新 Docker Desktop
    pause
    exit /b 1
)

echo ✅ Docker 环境检查通过
echo.

REM 创建目录结构
echo 📁 创建目录结构...
if not exist "config\keys" mkdir config\keys
if not exist "data" mkdir data
if not exist "logs" mkdir logs

REM 生成 JWT 密钥
echo 🔑 生成 JWT 密钥...
for /f "delims=" %%i in ('powershell -Command "[Convert]::ToBase64String((1..32|%%{Get-Random -Max 256}))"') do set JWT_SECRET=%%i
echo JWT_SECRET=!JWT_SECRET!> .env
echo TZ=Asia/Shanghai>> .env
echo LOG_LEVEL=INFO>> .env
echo ✅ JWT 密钥已生成并保存到 .env
echo.

REM 生成 RSA 密钥对
echo 🔐 生成 RSA 密钥对...
if not exist "config\keys\private.pem" (
    cd skin-backend
    python gen_key.py
    if exist "private.pem" (
        move private.pem ..\config\keys\private.pem >nul 2>&1
        move public.pem ..\config\keys\public.pem >nul 2>&1
        cd ..
        echo ✅ RSA 密钥对已生成
    ) else (
        cd ..
        echo ⚠️  RSA 密钥生成失败，将在容器启动时自动生成
    )
) else (
    echo ✅ RSA 密钥对已存在
)
echo.

REM 创建配置文件
echo ⚙️  创建配置文件...
if not exist "config\config.yaml" (
    (
        echo # Element-Skin 配置文件
        echo jwt:
        echo   secret: "!JWT_SECRET!"
        echo.
        echo database:
        echo   path: "/data/yggdrasil.db"
        echo.
        echo textures:
        echo   directory: "/data/textures"
        echo.
        echo server:
        echo   host: "0.0.0.0"
        echo   port: 8000
    ) > config\config.yaml
    echo ✅ 配置文件已创建
) else (
    echo ✅ 配置文件已存在
)
echo.

REM 询问是否构建镜像
set /p BUILD_NOW="是否现在构建 Docker 镜像？(y/n) [y]: "
if "!BUILD_NOW!"=="" set BUILD_NOW=y

if /i "!BUILD_NOW!"=="y" (
    echo.
    echo 🔨 开始构建 Docker 镜像...
    docker compose build
    echo ✅ Docker 镜像构建完成
    echo.
    
    REM 询问是否启动服务
    set /p START_NOW="是否现在启动服务？(y/n) [y]: "
    if "!START_NOW!"=="" set START_NOW=y
    
    if /i "!START_NOW!"=="y" (
        echo.
        echo 🚀 启动服务...
        docker compose up -d
        echo.
        echo ⏳ 等待服务启动...
        timeout /t 10 /nobreak >nul
        
        REM 检查服务状态
        echo.
        echo 📊 服务状态：
        docker compose ps
        
        echo.
        echo ======================================
        echo ✅ 部署完成！
        echo ======================================
        echo.
        echo 📍 访问地址：
        echo    前端: http://localhost/
        echo    后端: http://localhost:8000/
        echo    API 文档: http://localhost:8000/docs
        echo.
        echo 📝 首次使用：
        echo    1. 访问前端页面
        echo    2. 点击「注册」创建账号
        echo    3. 第一个注册的用户将自动成为管理员
        echo    4. 登录后进入「管理面板」→「设置」配置站点信息
        echo.
        echo 🔍 查看日志: docker compose logs -f
        echo 🛑 停止服务: docker compose down
        echo 🔄 重启服务: docker compose restart
        echo.
    )
)

REM 保存重要信息
(
    echo Element-Skin 部署信息
    echo ====================
    echo.
    echo 部署时间: %date% %time%
    echo JWT 密钥: !JWT_SECRET!
    echo.
    echo 重要提醒:
    echo 1. 请妥善保管 .env 文件和 config\keys\ 目录
    echo 2. 首次部署后请立即访问站点并注册管理员账号
    echo 3. 在管理面板中配置站点 URL（必须与实际访问地址一致）
    echo 4. 建议启用 HTTPS 和速率限制
    echo.
    echo 常用命令:
    echo - 查看日志: docker compose logs -f
    echo - 停止服务: docker compose down
    echo - 启动服务: docker compose up -d
    echo - 重启服务: docker compose restart
    echo - 查看状态: docker compose ps
    echo.
    echo 文档链接:
    echo - 完整文档: README.md
    echo - 部署指南: DEPLOYMENT.md
) > DEPLOYMENT_INFO.txt

echo 💾 部署信息已保存到 DEPLOYMENT_INFO.txt
echo.
pause
