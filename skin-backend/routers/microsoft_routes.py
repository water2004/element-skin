"""Microsoft 正版验证模块路由"""

from fastapi import APIRouter, HTTPException, Depends, Body
from fastapi.responses import Response
import time
import secrets
import urllib.parse

from backends.microsoft_backend import MicrosoftBackend
from routers.deps import get_current_user

router = APIRouter(prefix="/microsoft")


def setup_routes(db, config, texture_storage):
    """设置路由（注入依赖）"""

    microsoft_backend = MicrosoftBackend(db, config, texture_storage)

    # OAuth state 存储（生产环境应使用 Redis）
    oauth_states = {}

    @router.get("/auth-url")
    async def microsoft_get_auth_url(payload: dict = Depends(get_current_user)):
        """获取微软 OAuth 授权 URL"""
        state = secrets.token_urlsafe(32)
        auth_url = await microsoft_backend.get_authorization_url(state)
        oauth_states[state] = {
            "user_id": payload.get("sub"),
            "expires_at": time.time() + 600,
        }
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

        frontend_url = config.get("server.site_url", "http://localhost:5173").rstrip("/")
        try:
            profile = await microsoft_backend.complete_auth_flow(code)

            if not profile.get("profile"):
                raise Exception("No Minecraft Java Edition profile found for this account.")

            # 将 profile 数据临时存储，供前端获取
            temp_token = secrets.token_urlsafe(32)
            oauth_states[temp_token] = {
                "user_id": user_id,
                "profile": profile,
                "expires_at": time.time() + 300,  # 5分钟
            }
            location = f"{frontend_url}/dashboard/roles?ms_token={temp_token}"
        except Exception as e:
            error_msg = urllib.parse.quote(str(e).replace("\n", " "))
            location = f"{frontend_url}/dashboard/roles?error={error_msg}"

        return Response(status_code=302, headers={"Location": location})

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
