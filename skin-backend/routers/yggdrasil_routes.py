"""Yggdrasil 游戏登录模块路由"""

from fastapi import (
    APIRouter,
    Request,
    HTTPException,
    File,
    Header,
    UploadFile,
    Form,
)
from fastapi.responses import Response
from utils.schemas import AuthRequest, RefreshRequest, JoinRequest
from backends.yggdrasil_backend import YggdrasilBackend
from backends.fallback_backend import FallbackBackend
from database_module import Database

router = APIRouter()


def _require_bearer_token(authorization: str | None) -> str:
    if authorization and authorization.startswith("Bearer "):
        token = authorization.split(" ", 1)[1]
        if token:
            return token
    raise HTTPException(status_code=401, detail="access token required")


def setup_routes(backend: YggdrasilBackend, db: Database, rate_limiter):
    """设置路由（注入依赖）"""

    fallback_backend = FallbackBackend(db)

    @router.post("/authserver/authenticate")
    async def authenticate(req: AuthRequest, request: Request):
        """游戏认证接口"""
        await rate_limiter.check(request, is_auth_endpoint=True)
        resp = await backend.build_authenticate_response(
            req.username, req.password, req.clientToken, req.requestUser
        )
        rate_limiter.reset(request.client.host, request.url.path)
        return resp

    @router.post("/authserver/refresh")
    async def refresh(req: RefreshRequest):
        """刷新令牌"""
        # 兼容 Pydantic 对象和 dict
        selected_profile = getattr(req, "selectedProfile", None)
        selected_profile_uuid = None

        if selected_profile:
            if isinstance(selected_profile, dict):
                selected_profile_uuid = selected_profile.get("id")
            elif hasattr(selected_profile, "id"):
                selected_profile_uuid = selected_profile.id

        request_user = getattr(req, "requestUser", False)

        return await backend.refresh(
            req.accessToken, req.clientToken, selected_profile_uuid, request_user
        )

    @router.post("/authserver/validate")
    async def validate(req: dict):
        """验证令牌"""
        await backend.validate(req)
        return Response(status_code=204)

    @router.post("/authserver/invalidate")
    async def invalidate(req: dict):
        """吊销令牌"""
        token = req.get("accessToken")
        if token:
            await backend.invalidate(token)
        return Response(status_code=204)

    @router.post("/authserver/signout")
    async def signout(req: dict, request: Request):
        """登出：吊销用户的所有令牌"""
        await rate_limiter.check(request, is_auth_endpoint=True)
        username = req.get("username")
        password = req.get("password")
        if not username or not password:
            raise HTTPException(status_code=400, detail="Missing username or password")
        await backend.signout(username, password)
        rate_limiter.reset(request.client.host, request.url.path)
        return Response(status_code=204)

    @router.post("/sessionserver/session/minecraft/join")
    async def join_server(req: JoinRequest, request: Request):
        """加入服务器"""
        ip = request.client.host
        await backend.join_server(
            req.accessToken, req.selectedProfile, req.serverId, ip
        )
        return Response(status_code=204)

    @router.get("/sessionserver/session/minecraft/hasJoined")
    async def has_joined(
        request: Request, username: str, serverId: str, ip: str = None
    ):
        """检查是否已加入服务器"""
        profile = await backend.has_joined(username, serverId)
        if profile:
            return backend.build_profile_json(profile, sign=True)

        # Fallback to configured services
        fallback_resp = await fallback_backend.has_joined(username, serverId, ip)
        if fallback_resp:
            return fallback_resp

        return Response(status_code=204)

    @router.get("/sessionserver/session/minecraft/profile/{uuid}")
    async def get_profile(request: Request, uuid: str, unsigned: bool = True):
        """获取角色信息"""
        profile = await backend.get_profile(uuid)
        if profile:
            return backend.build_profile_json(profile, sign=not unsigned)

        # Fallback to configured services
        fallback_resp = await fallback_backend.get_profile(uuid, unsigned)
        if fallback_resp:
            return fallback_resp

        return Response(status_code=204)

    async def _local_or_fallback_lookup(player_name: str, fallback_fn):
        local = await backend.lookup_profile_by_name(player_name)
        if local:
            return local
        fallback_resp = await fallback_fn(player_name)
        if fallback_resp:
            return fallback_resp
        return Response(status_code=204)

    @router.get("/api/users/profiles/minecraft/{playerName}")
    @router.get("/users/profiles/minecraft/{playerName}")
    @router.get("/api/profiles/minecraft/{playerName}")
    async def get_profile_by_name_mojang(playerName: str):
        """单个玩家名转 UUID (Proxy to Mojang Account API)"""
        return await _local_or_fallback_lookup(
            playerName, fallback_backend.get_profile_by_name
        )

    @router.post("/api/profiles/minecraft")
    async def get_profiles_by_names(req: list[str], request: Request):
        """按名称批量查询角色"""
        if not isinstance(req, list):
            raise HTTPException(status_code=400, detail="Request body must be an array")

        # 1. 查询本地
        local_profiles = await backend.get_profiles_by_names(req)

        # 2. 如果启用了转发，查询 Fallback 服务补全缺失的
        found_names = {p["name"].lower() for p in local_profiles}
        missing_names = [n for n in req if n.lower() not in found_names]
        if missing_names:
            mojang_profiles = await fallback_backend.bulk_lookup(missing_names)
            if isinstance(mojang_profiles, list):
                local_profiles.extend(mojang_profiles)

        return local_profiles

    @router.get("/")
    async def get_api_metadata(request: Request):
        """API元数据端点 (Yggdrasil服务发现)"""
        return await backend.build_metadata(str(request.base_url))

    @router.get("/api/minecraft/profile/lookup/name/{playerName}")
    @router.get("/minecraft/profile/lookup/name/{playerName}")
    async def lookup_profile_by_name(playerName: str):
        """[Proxy] Minecraft Services Profile Lookup"""
        return await _local_or_fallback_lookup(
            playerName, fallback_backend.services_lookup
        )

    @router.put("/api/user/profile/{uuid}/{textureType}")
    async def api_put_profile(
        uuid: str,
        textureType: str,
        file: UploadFile = File(...),
        model: str = Form(""),
        authorization: str = Header(None),
    ):
        """材质上传（PUT 方法）"""
        token = _require_bearer_token(authorization)
        content = await file.read()
        await backend.upload_texture(token, uuid, textureType, content, model)
        return Response(status_code=204)

    @router.delete("/api/user/profile/{uuid}/{textureType}")
    async def api_delete_profile(
        uuid: str, textureType: str, authorization: str = Header(None)
    ):
        """删除材质"""
        token = _require_bearer_token(authorization)
        await backend.delete_texture(token, uuid, textureType)
        return Response(status_code=204)

    return router
