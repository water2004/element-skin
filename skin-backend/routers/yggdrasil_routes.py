"""Yggdrasil 游戏登录模块路由"""

from fastapi import (
    APIRouter,
    Request,
    HTTPException,
    Depends,
    File,
    Header,
    UploadFile,
    Form,
)
from fastapi.responses import Response
from typing import Dict
from utils.schemas import AuthRequest, RefreshRequest, JoinRequest
from utils.typing import PlayerProfile
from utils.crypto import CryptoUtils
import json
import base64
import time
import logging
from backends.yggdrasil_backend import YggdrasilBackend
from database_module import Database
from config_loader import config

# 配置日志
logger = logging.getLogger("yggdrasil.fallback")

router = APIRouter()


async def get_profile_json(
    profile: PlayerProfile,
    crypto: CryptoUtils,
    sign: bool = False,
    base_url: str = None,
) -> Dict:
    """构建角色 JSON，包含 textures 和签名"""
    pid = profile.id
    name = profile.name
    model = profile.texture_model
    skin_hash = profile.skin_hash
    cape_hash = profile.cape_hash

    profile_data = {"id": pid, "name": name, "properties": []}

    # 构建 Textures 属性
    textures_payload = {
        "timestamp": int(time.time() * 1000),
        "profileId": pid,
        "profileName": name,
        "textures": {},
    }

    # 材质基础 URL
    base_texture_url = "/static/textures/"
    if base_url:
        base_texture_url = base_url.rstrip("/") + "/static/textures/"

    if skin_hash:
        textures_payload["textures"]["SKIN"] = {
            "url": base_texture_url + skin_hash + ".png"
        }
        if model == "slim":
            textures_payload["textures"]["SKIN"]["metadata"] = {"model": "slim"}

    if cape_hash:
        textures_payload["textures"]["CAPE"] = {
            "url": base_texture_url + cape_hash + ".png"
        }

    # 序列化 Textures
    textures_json = json.dumps(textures_payload)
    textures_base64 = base64.b64encode(textures_json.encode("utf-8")).decode("utf-8")

    prop = {"name": "textures", "value": textures_base64}
    if sign:
        prop["signature"] = crypto.sign_data(textures_base64)

    profile_data["properties"].append(prop)

    # 添加 uploadableTextures 扩展属性
    profile_data["properties"].append(
        {"name": "uploadableTextures", "value": "skin,cape"}
    )

    return profile_data


def setup_routes(backend: YggdrasilBackend, db: Database, crypto, rate_limiter):
    """设置路由（注入依赖）"""

    @router.post("/authserver/authenticate")
    async def authenticate(req: AuthRequest, request: Request):
        """游戏认证接口"""
        await rate_limiter.check(request, is_auth_endpoint=True)

        try:
            access_token, avail_players, selected_profile, user_id = (
                await backend.authenticate(req.username, req.password, req.clientToken)
            )
            # 登录成功，重置速率限制
            rate_limiter.reset(request.client.host, request.url.path)

            # 构建标准响应
            resp = {
                "accessToken": access_token,
                "clientToken": req.clientToken
                or access_token,  # 确保 clientToken 总有值
                "availableProfiles": [
                    {"id": p.id, "name": p.name} for p in avail_players
                ],
            }
            if selected_profile:
                resp["selectedProfile"] = {
                    "id": selected_profile.id,
                    "name": selected_profile.name,
                }

            # 按规范添加 user 对象（如果请求）
            if req.requestUser:
                user_obj = await db.user.get_by_id(user_id)
                if user_obj:
                    resp["user"] = {
                        "id": user_id,
                        "properties": [
                            {
                                "name": "preferredLanguage",
                                "value": user_obj.preferredLanguage,
                            }
                        ],
                    }

            return resp

        except Exception as e:
            raise e

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
        site_url = await db.setting.get("site_url", str(request.base_url))
        profile = await backend.has_joined(username, serverId)
        if profile:
            return await get_profile_json(profile, crypto, sign=True, base_url=site_url)

        # Fallback to Mojang
        if await db.setting.get("fallback_mojang_hasjoined", "false") == "true":
            session_url = config.get("mojang.session_url")
            import aiohttp

            try:
                params = {"username": username, "serverId": serverId}
                if ip:
                    params["ip"] = ip
                target_url = f"{session_url}/session/minecraft/hasJoined"
                logger.info(
                    f"[Fallback] Checking hasJoined via: {target_url} | User: {username}"
                )

                async with aiohttp.ClientSession() as session:
                    async with session.get(
                        target_url, params=params, timeout=5
                    ) as resp:
                        logger.info(f"[Fallback] hasJoined response: {resp.status}")
                        if resp.status == 200:
                            content = await resp.read()
                            return Response(
                                content=content,
                                status_code=200,
                                media_type="application/json",
                            )
                        elif resp.status != 204:
                            logger.warning(
                                f"[Fallback] hasJoined returned unexpected status: {resp.status}"
                            )
            except Exception as e:
                logger.error(f"[Fallback] hasJoined failed: {e}")

        return Response(status_code=204)

    @router.get("/sessionserver/session/minecraft/profile/{uuid}")
    async def get_profile(request: Request, uuid: str, unsigned: bool = True):
        """获取角色信息"""
        site_url = await db.setting.get("site_url", str(request.base_url))
        profile = await backend.get_profile(uuid)
        if profile:
            return await get_profile_json(
                profile, crypto, sign=not unsigned, base_url=site_url
            )

        # Fallback to Mojang
        if await db.setting.get("fallback_mojang_profile", "false") == "true":
            session_url = config.get("mojang.session_url")
            import aiohttp

            try:
                target_url = f"{session_url}/session/minecraft/profile/{uuid}?unsigned={str(unsigned).lower()}"
                logger.info(f"[Fallback] Fetching profile via: {target_url}")

                async with aiohttp.ClientSession() as session:
                    async with session.get(target_url, timeout=5) as resp:
                        logger.info(f"[Fallback] Profile response: {resp.status}")
                        if resp.status == 200:
                            content = await resp.read()
                            return Response(
                                content=content,
                                status_code=200,
                                media_type="application/json",
                            )
                        elif resp.status != 204:
                            logger.warning(
                                f"[Fallback] Profile fetch returned unexpected status: {resp.status}"
                            )
            except Exception as e:
                logger.error(f"[Fallback] Profile fetch failed: {e}")

        return Response(status_code=204)

    @router.get("/api/users/profiles/minecraft/{playerName}")
    @router.get("/users/profiles/minecraft/{playerName}")
    @router.get("/api/profiles/minecraft/{playerName}")
    async def get_profile_by_name_mojang(playerName: str):
        """单个玩家名转 UUID (Proxy to Mojang Account API)"""
        # 先查本地
        p = await db.user.get_profile_by_name(playerName)
        if p:
            return {"id": p.id, "name": p.name}

        # Fallback to Mojang
        if await db.setting.get("fallback_mojang_profile", "false") == "true":
            account_url = config.get("mojang.account_url")
            import aiohttp

            try:
                target_url = f"{account_url}/users/profiles/minecraft/{playerName}"
                logger.info(f"[Fallback] Lookup UUID via: {target_url}")

                async with aiohttp.ClientSession() as session:
                    async with session.get(target_url, timeout=5) as resp:
                        logger.info(f"[Fallback] UUID lookup response: {resp.status}")
                        if resp.status == 200:
                            content = await resp.read()
                            return Response(
                                content=content,
                                status_code=200,
                                media_type="application/json",
                            )
                        elif resp.status != 204:
                            logger.warning(
                                f"[Fallback] UUID lookup returned unexpected status: {resp.status}"
                            )
            except Exception as e:
                logger.error(f"[Fallback] UUID lookup failed: {e}")

        return Response(status_code=204)

    @router.post("/api/profiles/minecraft")
    async def get_profiles_by_names(req: list[str], request: Request):
        """按名称批量查询角色"""
        if not isinstance(req, list):
            raise HTTPException(status_code=400, detail="Request body must be an array")

        site_url = await db.setting.get("site_url", str(request.base_url))
        # 1. 查询本地
        local_profiles = await backend.get_profiles_by_names(req, base_url=site_url)

        # 2. 如果启用了转发，查询 Mojang 补全缺失的
        if await db.setting.get("fallback_mojang_profile", "false") == "true":
            found_names = {p["name"].lower() for p in local_profiles}
            missing_names = [n for n in req if n.lower() not in found_names]
            if missing_names:
                account_url = config.get("mojang.account_url")
                import aiohttp

                try:
                    target_url = f"{account_url}/profiles/minecraft"
                    logger.info(
                        f"[Fallback] Bulk lookup via: {target_url} | Missing: {len(missing_names)}"
                    )

                    async with aiohttp.ClientSession() as session:
                        async with session.post(
                            target_url, json=missing_names, timeout=5
                        ) as resp:
                            logger.info(
                                f"[Fallback] Bulk lookup response: {resp.status}"
                            )
                            if resp.status == 200:
                                mojang_profiles = await resp.json()
                                if isinstance(mojang_profiles, list):
                                    logger.info(
                                        f"[Fallback] Found {len(mojang_profiles)} profiles from Mojang"
                                    )
                                    local_profiles.extend(mojang_profiles)
                except Exception as e:
                    logger.error(f"[Fallback] Bulk lookup failed: {e}")

        return local_profiles

    @router.get("/")
    async def get_api_metadata(request: Request):
        """API元数据端点 (Yggdrasil服务发现)"""
        site_name = await db.setting.get("site_name", "Yggdrasil 皮肤站")
        site_url = await db.setting.get("site_url", str(request.base_url).rstrip("/"))

        # 读取公钥
        public_key_pem = crypto.get_public_key_pem()

        # 构建元数据响应
        metadata = {
            "meta": {
                "serverName": site_name,
                "implementationName": "element-skin",
                "implementationVersion": "1.0.0",
                "links": {
                    "homepage": site_url,
                    "register": f"{site_url}/register" if site_url else None,
                },
                "feature.non_email_login": True,
            },
            "skinDomains": config.get("mojang.skin_domains", [])
            + [
                (
                    site_url.replace("https://", "")
                    .replace("http://", "")
                    .split("/")[0]
                    if site_url
                    else "localhost"
                )
            ],
            "signaturePublickey": public_key_pem,
        }

        return metadata

    @router.get("/api/minecraft/profile/lookup/name/{playerName}")
    @router.get("/minecraft/profile/lookup/name/{playerName}")
    async def lookup_profile_by_name(playerName: str):
        """[Proxy] Minecraft Services Profile Lookup"""
        # 1. Local Lookup
        p = await db.user.get_profile_by_name(playerName)
        if p:
            return {"id": p.id, "name": p.name}

        # 2. Fallback

        if await db.setting.get("fallback_mojang_profile", "false") == "true":

            services_url = config.get("mojang.services_url")

            import aiohttp

            try:

                target_url = (
                    f"{services_url}/minecraft/profile/lookup/name/{playerName}"
                )

                logger.info(f"[Fallback] Services lookup via: {target_url}")

                async with aiohttp.ClientSession() as session:

                    async with session.get(target_url, timeout=5) as resp:

                        logger.info(
                            f"[Fallback] Services lookup response: {resp.status}"
                        )

                        if resp.status == 200:

                            content = await resp.read()

                            return Response(
                                content=content,
                                status_code=200,
                                media_type="application/json",
                            )

                        elif resp.status != 204:

                            logger.warning(
                                f"[Fallback] Services lookup returned unexpected status: {resp.status}"
                            )

            except Exception as e:

                logger.error(f"[Fallback] Services lookup failed: {e}")

        return Response(status_code=204)

    @router.put("/api/user/profile/{uuid}/{textureType}")
    async def api_put_profile(
        uuid: str,
        textureType: str,
        file: UploadFile = File(...),
        model: str = Form(""),
        authorization: str = Header(None),
    ):
        """材质上传（PUT 方法）"""
        token = None
        if authorization and authorization.startswith("Bearer "):
            token = authorization.split(" ", 1)[1]
        if not token:
            raise HTTPException(status_code=401, detail="access token required")
        content = await file.read()
        await backend.upload_texture(token, uuid, textureType, content, model)
        return Response(status_code=204)

    @router.delete("/api/user/profile/{uuid}/{textureType}")
    async def api_delete_profile(
        uuid: str, textureType: str, authorization: str = Header(None)
    ):
        """删除材质"""
        token = None
        if authorization and authorization.startswith("Bearer "):
            token = authorization.split(" ", 1)[1]
        if not token:
            raise HTTPException(status_code=401, detail="access token required")
        await backend.delete_texture(token, uuid, textureType)
        return Response(status_code=204)

    return router
