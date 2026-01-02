"""
Element Skin Backend - 主入口文件
"""

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
from fastapi.staticfiles import StaticFiles
from fastapi.middleware.cors import CORSMiddleware
from contextlib import asynccontextmanager
import os

from config_loader import config
from database_module import Database
from backends.yggdrasil_backend import YggdrasilBackend, YggdrasilError
from backends.site_backend import SiteBackend
from utils.crypto import CryptoUtils
from utils.rate_limiter import RateLimiter
from routers import yggdrasil_routes, site_routes, microsoft_routes

# ========== 初始化核心组件 ==========
db_path = config.get("database.path", "yggdrasil.db")
db = Database(db_path)
private_key_path = config.get("keys.private_key", "private.pem")
crypto = CryptoUtils(private_key_path)
rate_limiter = RateLimiter(db)  # New dependency-injected rate limiter
ygg_backend = YggdrasilBackend(db, crypto)
site_backend = SiteBackend(db, config)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """应用生命周期管理"""
    await db.connect()
    await db.init()
    try:
        yield
    finally:
        await db.close()


# ========== 创建 FastAPI 应用 ==========
app = FastAPI(
    title="Element Skin Backend",
    description="Yggdrasil 皮肤站后端服务",
    version="1.0.0",
    lifespan=lifespan,
    root_path=config.get("server.root_path", ""),
)

# ========== 中间件配置 ==========

# CORS 跨域配置
cors_origins = config.get("cors.allow_origins", ["*"])
cors_credentials = config.get("cors.allow_credentials", True)
app.add_middleware(
    CORSMiddleware,
    allow_origins=cors_origins,
    allow_credentials=cors_credentials,
    allow_methods=["*"],
    allow_headers=["*"],
)

# 挂载静态材质目录
textures_path = config.get("textures.directory", "textures")
os.makedirs(textures_path, exist_ok=True)
app.mount("/static/textures", StaticFiles(directory=textures_path), name="textures")


# ========== 异常处理器 ==========


@app.exception_handler(YggdrasilError)
async def ygg_exception_handler(request: Request, exc: YggdrasilError):
    return JSONResponse(
        status_code=exc.status_code,
        content={"error": exc.error, "errorMessage": exc.message},
    )


# ========== 注册路由模块 ==========

yggdrasil_router = yggdrasil_routes.setup_routes(ygg_backend, db, crypto, rate_limiter)
app.include_router(yggdrasil_router)

site_router = site_routes.setup_routes(db, site_backend, rate_limiter, config)
app.include_router(site_router)

microsoft_router = microsoft_routes.setup_routes(db, config)
app.include_router(microsoft_router)

# ========== 应用启动 ==========

if __name__ == "__main__":
    import uvicorn

    host = config.get("server.host", "0.0.0.0")
    port = config.get("server.port", 8000)
    debug = config.get("server.debug", False)

    print(f"Starting Element Skin Backend Server...")
    print(f"Host: {host}")
    print(f"Port: {port}")
    print(f"Debug: {debug}")

    uvicorn.run(
        "routes_reference:app",
        host=host,
        port=port,
        reload=debug,
        log_level="info" if debug else "warning",
    )
