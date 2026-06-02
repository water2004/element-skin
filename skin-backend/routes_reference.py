"""
Element Skin Backend - 主入口文件
"""

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
from fastapi.middleware.cors import CORSMiddleware
from contextlib import asynccontextmanager
import os

import logging

from config_loader import config
from utils.logging_config import setup_logging

# 尽早配置日志（级别由 server.debug 驱动），使后续各模块的 logger 生效
setup_logging(config.get("server.debug", False))

logger = logging.getLogger(__name__)

from database_module import Database
from backends.yggdrasil_backend import YggdrasilBackend, YggdrasilError
from backends.site_backend import SiteBackend
from backends.profile_import_backend import ProfileImportBackend
from backends.admin_backend import AdminBackend
from backends.settings_backend import SettingsBackend
from services import TextureStorage
from utils.crypto import CryptoUtils
from utils.rate_limiter import RateLimiter
from routers import yggdrasil_routes, site_routes, microsoft_routes, admin_routes
from routers import deps as _deps

# ========== 初始化核心组件 ==========
db_dsn = config.get("database.dsn", "postgresql://elementskin:password@localhost:5432/elementskin")
max_conns = config.get("database.max_connections", 10)
db = Database(db_dsn, max_connections=max_conns)
private_key_path = config.get("keys.private_key", "private.pem")
crypto = CryptoUtils(private_key_path)
rate_limiter = RateLimiter(db)  # New dependency-injected rate limiter
texture_storage = TextureStorage(config.get("textures.directory", "textures"))
ygg_backend = YggdrasilBackend(db, crypto, texture_storage, config)
site_backend = SiteBackend(db, config, texture_storage)
profile_import_backend = ProfileImportBackend(db, texture_storage)
admin_backend = AdminBackend(db, config)
settings_backend = SettingsBackend(db)

# 让鉴权依赖能查库校验封禁/管理员实时状态
_deps.bind_db(db)


async def _refresh_cleanup_loop(db, interval_seconds: int = 3600):
    """周期清理过期 refresh token。单实例运行，进程内任务即可。

    清理失败不应中断循环：记录后继续，等待下一轮。
    """
    import asyncio
    import time

    while True:
        try:
            await asyncio.sleep(interval_seconds)
            await db.user.delete_expired_refresh_tokens(int(time.time() * 1000))
        except asyncio.CancelledError:
            break
        except Exception:
            logger.warning("refresh token cleanup failed", exc_info=True)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """应用生命周期管理"""
    import asyncio
    import time
    from utils.jwt_utils import assert_jwt_secret_ok
    # 启动期 fail-fast：JWT 密钥缺失/默认/过短即拒绝起服务，杜绝可伪造 token 的致命路径
    assert_jwt_secret_ok()
    await db.connect()
    await db.init()
    # 启动时清理一次过期的站点 refresh token
    await db.user.delete_expired_refresh_tokens(int(time.time() * 1000))
    # 周期性清理过期 refresh token：仅启动清一次会让过期行无限累积（每次登录/新设备一行）。
    # 单实例约束下进程内 asyncio 任务即可，无需外部定时器。
    cleanup_task = asyncio.create_task(_refresh_cleanup_loop(db))
    try:
        yield
    finally:
        cleanup_task.cancel()
        try:
            await cleanup_task
        except asyncio.CancelledError:
            pass
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

# # 统一路径结尾斜杠中间件
# @app.middleware("http")
# async def append_slash_middleware(request: Request, call_next):    
#     # 为非根路径且不以斜杠结尾的路径添加斜杠
#     request.scope["path"] += "/"
#     # 去除重复斜杠
#     request.scope["path"] = request.scope["path"].replace("//", "/")
        
#     response = await call_next(request)
#     return response

# ========== 静态资源目录准备 ==========
# 静态文件现在由前端 Nginx 容器处理，后端仅负责文件的写入和管理
# 材质目录由 TextureStorage 负责创建

carousel_path = config.get("carousel.directory", "carousel")
os.makedirs(carousel_path, exist_ok=True)


@app.exception_handler(YggdrasilError)
async def ygg_exception_handler(request: Request, exc: YggdrasilError):
    return JSONResponse(
        status_code=exc.status_code,
        content={"error": exc.error, "errorMessage": exc.message},
    )


# ========== 注册路由模块 ==========

yggdrasil_router = yggdrasil_routes.setup_routes(ygg_backend, db, rate_limiter)
app.include_router(yggdrasil_router)

site_router = site_routes.setup_routes(site_backend, profile_import_backend, settings_backend, rate_limiter, config)
app.include_router(site_router)

admin_router = admin_routes.setup_routes(admin_backend, settings_backend)
app.include_router(admin_router)

microsoft_router = microsoft_routes.setup_routes(db, config, texture_storage)
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
        loop="asyncio",
        log_level="info" if debug else "warning",
        proxy_headers=True,
        forwarded_allow_ips="*",
    )
