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

# OAuth state / 临时 token 存储：一次性、带 TTL。
# 内存实现仅支持单实例；多实例需换 Redis（见 utils/state_store.py）。
# 模块级单例：与 router 同生命周期，且便于测试直接注入会话。
oauth_states = InMemoryStateStore()


def setup_routes(db, config, texture_storage):
    """设置路由（注入依赖）"""

    microsoft_backend = MicrosoftBackend(db, config, texture_storage)

    @router.get("/auth-url")
    async def microsoft_get_auth_url(payload: dict = Depends(get_current_user)):
        """获取微软 OAuth 授权 URL"""
        state = secrets.token_urlsafe(32)
        auth_url = await microsoft_backend.get_authorization_url(state)
        oauth_states.put(
            state,
            {"user_id": payload.get("sub"), "kind": "oauth_state"},
            ttl_seconds=600,
        )
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

        # 验证 state（pop 取出即删，过期返回 None）。校验 kind 防止 token 类型混用。
        session_data = oauth_states.pop(state)
        if not session_data or session_data.get("kind") != "oauth_state":
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
                {"user_id": user_id, "profile": profile, "kind": "profile"},
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
        """使用临时 token 获取 profile 数据，并换发一次性导入 token。"""
        user_id = payload.get("sub")

        # pop 取出即删（一次性），过期返回 None。校验 kind 防止 token 类型混用。
        session_data = oauth_states.pop(ms_token)
        if not session_data or session_data.get("kind") != "profile":
            raise HTTPException(status_code=400, detail="Invalid or expired token")

        # 验证用户 ID
        if session_data["user_id"] != user_id:
            raise HTTPException(status_code=403, detail="Unauthorized")

        mc_profile = session_data["profile"]["profile"]

        verified = {
            "id": mc_profile["id"],
            "name": mc_profile["name"],
            "skins": mc_profile.get("skins", []),
            "capes": mc_profile.get("capes", []),
        }

        # 换发一次性导入 token：导入只信任这里固化的、经服务端验证过的资料，
        # 避免前端在 get-profile 之后用任意 profile_id/url 伪造导入（见 phase-7）。
        import_token = secrets.token_urlsafe(32)
        oauth_states.put(
            import_token,
            {"user_id": user_id, "kind": "import", "profile": verified},
            ttl_seconds=300,  # 5分钟
        )

        return {
            "profile": verified,
            "has_game": session_data["profile"].get("has_game", False),
            "import_token": import_token,
        }

    @router.post("/import-profile")
    async def microsoft_import_profile(
        ms_token: str = Body(..., embed=True),
        payload: dict = Depends(get_current_user),
    ):
        """导入正版角色。

        仅接受 get-profile 换发的一次性导入 token；导入字段全部取自服务端固化的、
        经验证的微软资料，忽略任何前端传入的 profile 字段，杜绝伪造导入。
        """
        user_id = payload.get("sub")

        session_data = oauth_states.pop(ms_token)
        if not session_data or session_data.get("kind") != "import":
            raise HTTPException(status_code=400, detail="Invalid or expired token")

        if session_data["user_id"] != user_id:
            raise HTTPException(status_code=403, detail="Unauthorized")

        mc_profile = session_data["profile"]
        skins = mc_profile.get("skins") or []
        capes = mc_profile.get("capes") or []
        skin = skins[0] if skins else {}
        cape = capes[0] if capes else {}

        return await microsoft_backend.import_profile(
            user_id,
            mc_profile["id"],
            mc_profile["name"],
            skin.get("url"),
            skin.get("variant", "classic"),
            cape.get("url"),
        )

    return router
