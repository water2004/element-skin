"""Microsoft 正版验证模块路由"""

from fastapi import APIRouter, Request, HTTPException, Depends, Body
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
from fastapi.responses import Response
import time
import secrets
import hashlib
import os

from backends.microsoft_backend import MicrosoftAuthService, download_texture
from utils.jwt_utils import decode_jwt_token
from utils.uuid_utils import generate_random_uuid
from database_module import Database

router = APIRouter(prefix="/microsoft")
security = HTTPBearer()


def setup_routes(db: Database, config):
    """设置路由（注入依赖）"""

    # OAuth state 存储（生产环境应使用 Redis）
    oauth_states = {}

    async def get_current_user(creds: HTTPAuthorizationCredentials = Depends(security)):
        """获取当前用户"""
        token = creds.credentials
        payload = decode_jwt_token(token)
        if not payload:
            raise HTTPException(status_code=401, detail="invalid or expired token")
        return payload

    @router.get("/auth-url")
    async def microsoft_get_auth_url(payload: dict = Depends(get_current_user)):
        """获取微软 OAuth 授权 URL"""
        client_id = await db.setting.get("microsoft_client_id")
        client_secret = await db.setting.get("microsoft_client_secret")

        if not client_id or client_id == "":
            raise HTTPException(
                status_code=500,
                detail="Microsoft OAuth not configured. Please contact administrator.",
            )

        if not client_secret or client_secret == "":
            raise HTTPException(
                status_code=500,
                detail="Microsoft OAuth client_secret not configured. Please contact administrator.",
            )

        try:
            # 生成 state 用于防 CSRF
            state = secrets.token_urlsafe(32)
            user_id = payload.get("sub")

            # 存储 state 与用户 ID 的映射（10分钟过期）
            oauth_states[state] = {
                "user_id": user_id,
                "expires_at": time.time() + 600,
            }

            # 获取 redirect_uri 配置
            default_redirect = config.get("server.site_url", "http://localhost:8000").rstrip("/") + "/microsoft/callback"
            redirect_uri = await db.setting.get(
                "microsoft_redirect_uri", default_redirect
            )

            service = MicrosoftAuthService(client_id, client_secret, redirect_uri)
            auth_url = service.get_authorization_url(state)

            return {"auth_url": auth_url, "state": state}
        except Exception as e:
            raise HTTPException(status_code=500, detail=str(e))

    @router.get("/callback")
    async def microsoft_callback(
        code: str = None, state: str = None, error: str = None
    ):
        """微软 OAuth 回调端点"""
        if error:
            raise HTTPException(
                status_code=400, detail=f"Authorization failed: {error}"
            )

        if not code or not state:
            raise HTTPException(
                status_code=400, detail="Missing code or state parameter"
            )

        # 验证 state
        if state not in oauth_states:
            raise HTTPException(status_code=400, detail="Invalid state parameter")

        session_data = oauth_states[state]

        # 检查是否过期
        if time.time() > session_data["expires_at"]:
            del oauth_states[state]
            raise HTTPException(status_code=400, detail="State expired")

        user_id = session_data["user_id"]
        del oauth_states[state]  # 使用后立即删除

        # 获取 OAuth 配置
        client_id = await db.setting.get("microsoft_client_id")
        client_secret = await db.setting.get("microsoft_client_secret")
        default_redirect = config.get("server.site_url", "http://localhost:8000").rstrip("/") + "/microsoft/callback"
        redirect_uri = await db.setting.get(
            "microsoft_redirect_uri", default_redirect
        )

        try:
            service = MicrosoftAuthService(client_id, client_secret, redirect_uri)

            # 交换授权码获取令牌
            token_data = await service.exchange_code_for_token(code)
            ms_access_token = token_data["access_token"]

            # 执行完整认证链
            profile = await service.complete_auth_flow(ms_access_token)

            if not profile.get("profile"):
                raise Exception("No Minecraft Java Edition profile found for this account.")

            # 将 profile 数据临时存储，供前端获取
            temp_token = secrets.token_urlsafe(32)
            oauth_states[temp_token] = {
                "user_id": user_id,
                "profile": profile,
                "expires_at": time.time() + 300,  # 5分钟
            }

            # 重定向回前端
            # 优先使用 server.site_url 作为前端地址（通常部署时 site_url 指向前端）
            # 开发环境下默认为 localhost:5173
            frontend_url = config.get("server.site_url", "http://localhost:5173")
            
            return Response(
                status_code=302,
                headers={
                    "Location": f"{frontend_url.rstrip('/')}/dashboard/roles?ms_token={temp_token}"
                },
            )

        except Exception as e:
            import urllib.parse

            frontend_url = config.get("server.site_url", "http://localhost:5173")
            error_msg = str(e).replace("\n", " ")
            error_msg_encoded = urllib.parse.quote(error_msg)
            return Response(
                status_code=302,
                headers={
                    "Location": f"{frontend_url.rstrip('/')}/dashboard/roles?error={error_msg_encoded}"
                },
            )

    @router.post("/get-profile")
    async def microsoft_get_profile(
        ms_token: str = Body(..., embed=True),
        payload: dict = Depends(get_current_user),
    ):
        """使用临时 token 获取 profile 数据"""
        user_id = payload.get("sub")

        if ms_token not in oauth_states:
            raise HTTPException(status_code=400, detail="Invalid or expired token")

        session_data = oauth_states[ms_token]

        # 检查是否过期
        if time.time() > session_data["expires_at"]:
            del oauth_states[ms_token]
            raise HTTPException(status_code=400, detail="Token expired")

        # 验证用户 ID
        if session_data["user_id"] != user_id:
            raise HTTPException(status_code=403, detail="Unauthorized")

        profile = session_data["profile"]
        del oauth_states[ms_token]  # 使用后删除

        return {
            "profile": {
                "id": profile["profile"]["id"],
                "name": profile["profile"]["name"],
                "skins": profile["profile"].get("skins", []),
                "capes": profile["profile"].get("capes", []),
            },
            "has_game": profile.get("has_game", False),
        }

    @router.post("/import-profile")
    async def microsoft_import_profile(
        data: dict, payload: dict = Depends(get_current_user)
    ):
        """导入正版角色"""
        user_id = payload.get("sub")
        profile_id = data.get("profile_id")
        profile_name = data.get("profile_name")
        skin_url = data.get("skin_url")
        skin_variant = data.get("skin_variant", "classic")
        cape_url = data.get("cape_url")

        if not profile_id or not profile_name:
            raise HTTPException(status_code=400, detail="Missing required fields")

        existing = await db.user.get_profile_by_name(profile_name)
        if existing:
            raise HTTPException(
                status_code=400, detail=f"Profile name '{profile_name}' already exists"
            )

        skin_hash, cape_hash = None, None

        # 下载并保存皮肤
        if skin_url:
            try:
                skin_data = await download_texture(skin_url)
                skin_hash, _ = await db.texture.upload(
                    user_id,
                    skin_data,
                    "skin",
                    f"From Microsoft account - {profile_name}",
                )
            except Exception as e:
                print(f"Failed to download skin: {e}")

        # 下载并保存披风
        if cape_url:
            try:
                cape_data = await download_texture(cape_url)
                cape_hash, _ = await db.texture.upload(
                    user_id,
                    cape_data,
                    "cape",
                    f"From Microsoft account - {profile_name}",
                )
            except Exception as e:
                print(f"Failed to download cape: {e}")

        from utils.typing import PlayerProfile

        texture_model = "slim" if skin_variant == "slim" else "default"
        await db.user.create_profile(
            PlayerProfile(
                profile_id, user_id, profile_name, texture_model
            )
        )

        if skin_hash:
            await db.user.update_profile_skin(profile_id, skin_hash)
        
        if cape_hash:
            await db.user.update_profile_cape(profile_id, cape_hash)

        return {
            "ok": True,
            "profile": {
                "id": profile_id,
                "name": profile_name,
                "model": texture_model,
                "skin_hash": skin_hash,
                "cape_hash": cape_hash,
            },
        }

    return router
