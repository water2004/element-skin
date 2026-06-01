"""Microsoft 正版验证模块路由"""

import logging
import secrets

from fastapi import APIRouter, HTTPException, Depends, Body
from fastapi.responses import Response

from backends.microsoft_backend import MicrosoftBackend
from routers.deps import get_current_user
from utils.state_store import InMemoryStateStore

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/microsoft")


def setup_routes(db, config, texture_storage):
    """设置路由（注入依赖）"""

    microsoft_backend = MicrosoftBackend(db, config, texture_storage)

    # OAuth state / 临时 token 存储：一次性、带 TTL。
    # 内存实现仅支持单实例；多实例需换 Redis（见 utils/state_store.py）。
    oauth_states = InMemoryStateStore()

    @router.get("/auth-url")
    async def microsoft_get_auth_url(payload: dict = Depends(get_current_user)):
        """获取微软 OAuth 授权 URL"""
        state = secrets.token_urlsafe(32)
        auth_url = await microsoft_backend.get_authorization_url(state)
        oauth_states.put(state, {"user_id": payload.get("sub")}, ttl_seconds=600)
        return {"auth_url": auth_url, "state": state}

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

        # 验证 state（pop 取出即删，过期返回 None）
        session_data = oauth_states.pop(state)
        if not session_data:
            raise HTTPException(status_code=400, detail="Invalid or expired state parameter")

        user_id = session_data["user_id"]

        frontend_url = config.get("server.site_url", "http://localhost:5173").rstrip("/")
        try:
            profile = await microsoft_backend.complete_auth_flow(code)

            if not profile.get("profile"):
                raise Exception("No Minecraft Java Edition profile found for this account.")

            # 将 profile 数据临时存储，供前端获取
            temp_token = secrets.token_urlsafe(32)
            oauth_states.put(
                temp_token,
                {"user_id": user_id, "profile": profile},
                ttl_seconds=300,  # 5分钟
            )
            location = f"{frontend_url}/dashboard/roles?ms_token={temp_token}"
        except Exception as e:
            # 不把内部异常细节回显到重定向 URL（避免信息泄露），仅记录到服务端日志。
            logger.warning("Microsoft OAuth callback failed: %s", e)
            location = f"{frontend_url}/dashboard/roles?error=auth_failed"

        return Response(status_code=302, headers={"Location": location})

    @router.post("/get-profile")
    async def microsoft_get_profile(
        ms_token: str = Body(..., embed=True),
        payload: dict = Depends(get_current_user),
    ):
        """使用临时 token 获取 profile 数据"""
        user_id = payload.get("sub")

        # pop 取出即删（一次性），过期返回 None
        session_data = oauth_states.pop(ms_token)
        if not session_data:
            raise HTTPException(status_code=400, detail="Invalid or expired token")

        # 验证用户 ID
        if session_data["user_id"] != user_id:
            raise HTTPException(status_code=403, detail="Unauthorized")

        profile = session_data["profile"]

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
        profile_id = data.get("profile_id")
        profile_name = data.get("profile_name")
        if not profile_id or not profile_name:
            raise HTTPException(status_code=400, detail="Missing required fields")

        return await microsoft_backend.import_profile(
            payload.get("sub"),
            profile_id,
            profile_name,
            data.get("skin_url"),
            data.get("skin_variant", "classic"),
            data.get("cape_url"),
        )

    return router
