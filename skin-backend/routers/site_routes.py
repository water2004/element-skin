"""皮肤站主模块路由（用户、公共设置、材质等）"""

from fastapi import (
    APIRouter,
    Request,
    HTTPException,
    Depends,
    Body,
    UploadFile,
    File,
    Form,
)
from fastapi.responses import JSONResponse
from typing import Optional

from utils.jwt_utils import get_access_cookie_settings, get_refresh_cookie_settings
from utils.pagination import clamp_limit
from routers.deps import get_current_user
from config_loader import Config

router = APIRouter()


def _set_session_cookies(response: JSONResponse, access_token: str, refresh_token: str) -> None:
    """在响应上写入 access + refresh 两个 httponly cookie。"""
    access_cookie = get_access_cookie_settings()
    access_cookie["value"] = access_token
    response.set_cookie(**access_cookie)

    refresh_cookie = get_refresh_cookie_settings()
    refresh_cookie["value"] = refresh_token
    response.set_cookie(**refresh_cookie)


def setup_routes(site_backend, profile_import_backend, settings_backend, rate_limiter, config: Config):
    """设置路由（注入依赖）"""

    @router.post("/site-login")
    async def site_login(req: dict, request: Request):
        await rate_limiter.check(request, is_auth_endpoint=True)
        result = await site_backend.login(req.get("email"), req.get("password"))
        rate_limiter.reset(request.client.host, request.url.path)

        response = JSONResponse(content={"user_id": result["user_id"], "is_admin": result["is_admin"]})
        _set_session_cookies(response, result["access_token"], result["refresh_token"])
        return response

    @router.post("/site-logout")
    async def site_logout(request: Request):
        raw_refresh = request.cookies.get("refresh_token")
        if raw_refresh:
            await site_backend.revoke_refresh_token(raw_refresh)
        response = JSONResponse(content={"ok": True})
        response.delete_cookie("access_token", path="/")
        response.delete_cookie("refresh_token", path="/")
        return response

    @router.post("/register")
    async def register(req: dict, request: Request):
        await rate_limiter.check(request, is_auth_endpoint=True)

        email = req.get("email")
        password = req.get("password")
        username = req.get("username")
        invite = req.get("invite")
        code = req.get("code")

        if not email or not password or not username:
            raise HTTPException(status_code=400, detail="email, password and username required")

        user_id = await site_backend.register(email, password, username, invite, code)
        return {"id": user_id}

    @router.post("/send-verification-code")
    async def send_code(req: dict, request: Request):
        # Using auth endpoint rate limit for code sending
        await rate_limiter.check(request, is_auth_endpoint=True)
        email = req.get("email")
        type = req.get("type", "register") # register or reset
        if not email:
            raise HTTPException(status_code=400, detail="email required")
        return await site_backend.send_verification_code(email, type)

    @router.post("/reset-password")
    async def reset_password(req: dict, request: Request):
        await rate_limiter.check(request, is_auth_endpoint=True)
        email = req.get("email")
        password = req.get("password")
        code = req.get("code")
        if not email or not password or not code:
            raise HTTPException(status_code=400, detail="email, password and code required")
        await site_backend.reset_password(email, password, code)
        return {"ok": True}

    @router.get("/me")
    async def me(payload: dict = Depends(get_current_user)):
        return await site_backend.get_user_info(payload.get("sub"))

    @router.post("/me/refresh-token")
    async def refresh_jwt(request: Request):
        raw_refresh = request.cookies.get("refresh_token")
        if not raw_refresh:
            raise HTTPException(status_code=401, detail="not authenticated")
        result = await site_backend.rotate_refresh_token(raw_refresh)
        response = JSONResponse(content={"is_admin": result["is_admin"]})
        _set_session_cookies(response, result["access_token"], result["refresh_token"])
        return response

    @router.patch("/me")
    async def me_update(
        payload: dict = Depends(get_current_user), body: dict = Body(...)
    ):
        await site_backend.update_user_info(payload.get("sub"), body)
        return {"ok": True}

    @router.delete("/me")
    async def delete_me(payload: dict = Depends(get_current_user)):
        await site_backend.delete_user(payload.get("sub"), is_admin_action=False)
        return {"ok": True}

    @router.post("/me/password")
    async def change_password(
        payload: dict = Depends(get_current_user), body: dict = Body(...)
    ):
        await site_backend.change_password(
            payload.get("sub"), body.get("old_password"), body.get("new_password")
        )
        return {"ok": True, "message": "密码修改成功"}

    @router.post("/remote-ygg/get-profiles")
    async def get_ygg_profiles(
        payload: dict = Depends(get_current_user), body: dict = Body(...)
    ):
        return await profile_import_backend.get_ygg_profiles(
            body.get("api_url"), body.get("username"), body.get("password")
        )

    @router.post("/remote-ygg/import-profile")
    async def import_ygg_profile(
        payload: dict = Depends(get_current_user), body: dict = Body(...)
    ):
        return await profile_import_backend.import_ygg_profile(
            payload.get("sub"),
            body.get("api_url"),
            body.get("profile_id"),
            body.get("profile_name"),
        )

    @router.post("/remote-ygg/import-profiles")
    async def import_ygg_profiles(
        payload: dict = Depends(get_current_user), body: dict = Body(...)
    ):
        return await profile_import_backend.import_ygg_profiles(
            payload.get("sub"),
            body.get("api_url"),
            body.get("profiles", []),
        )

    @router.post("/me/profiles")
    async def create_profile(
        payload: dict = Depends(get_current_user), body: dict = Body(...)
    ):
        return await site_backend.create_profile(
            payload.get("sub"), body.get("name"), body.get("model", "default")
        )

    @router.patch("/me/profiles/{pid}")
    async def update_profile(
        pid: str, payload: dict = Depends(get_current_user), body: dict = Body(...)
    ):
        await site_backend.update_profile(payload.get("sub"), pid, body.get("name"))
        return {"ok": True}

    @router.delete("/me/profiles/{pid}")
    async def delete_profile(pid: str, payload: dict = Depends(get_current_user)):
        await site_backend.delete_profile(payload.get("sub"), pid)
        return {"ok": True}

    @router.delete("/me/profiles/{pid}/skin")
    async def clear_profile_skin(pid: str, payload: dict = Depends(get_current_user)):
        await site_backend.clear_profile_texture(payload.get("sub"), pid, "skin")
        return {"ok": True}

    @router.delete("/me/profiles/{pid}/cape")
    async def clear_profile_cape(pid: str, payload: dict = Depends(get_current_user)):
        await site_backend.clear_profile_texture(payload.get("sub"), pid, "cape")
        return {"ok": True}

    @router.post("/me/textures")
    async def upload_texture_to_library(
        payload: dict = Depends(get_current_user),
        file: UploadFile = File(...),
        texture_type: str = Form(...),
        note: str = Form(""),
        is_public: str = Form("false"),
        model: str = Form("default"),
    ):
        user_id = payload.get("sub")
        content = await file.read()
        public_bool = is_public.lower() == "true"
        try:
            texture_hash, texture_type = await site_backend.upload_texture_to_library(
                user_id, content, texture_type, note, is_public=public_bool, model=model
            )
            return {"hash": texture_hash, "type": texture_type, "note": note, "is_public": 1 if public_bool else 0, "model": model}
        except ValueError as e:
            raise HTTPException(status_code=400, detail=str(e))

    @router.get("/me/textures")
    async def list_my_textures(
        cursor: str | None = None,
        limit: int = 20,
        texture_type: Optional[str] = None,
        payload: dict = Depends(get_current_user)
    ):
        """获取我的材质列表（仅支持游标分页）"""
        return await site_backend.list_my_textures(
            payload.get("sub"), cursor, clamp_limit(limit), texture_type
        )

    @router.get("/me/profiles")
    async def list_my_profiles(
        cursor: str | None = None,
        limit: int = 20,
        payload: dict = Depends(get_current_user)
    ):
        """获取我的角色列表（仅支持游标分页）"""
        return await site_backend.list_my_profiles(payload.get("sub"), cursor, clamp_limit(limit))

    @router.get("/me/textures/{hash}/{texture_type}")
    async def get_my_texture_detail(
        hash: str,
        texture_type: str,
        payload: dict = Depends(get_current_user)
    ):
        return await site_backend.get_my_texture_detail(
            payload.get("sub"), hash, texture_type
        )

    @router.patch("/me/textures/{hash}/{texture_type}")
    async def update_my_texture(
        hash: str,
        texture_type: str,
        payload: dict = Depends(get_current_user),
        body: dict = Body(...),
    ):
        return await site_backend.update_my_texture(
            payload.get("sub"), hash, texture_type, body
        )

    @router.delete("/me/textures/{hash}/{texture_type}")
    async def delete_my_texture(
        hash: str, texture_type: str, payload: dict = Depends(get_current_user)
    ):
        await site_backend.remove_my_texture(payload.get("sub"), hash, texture_type)
        return {"ok": True}

    @router.post("/me/textures/{hash}/add")
    async def add_texture_to_wardrobe(
        hash: str, payload: dict = Depends(get_current_user)
    ):
        await site_backend.add_texture_to_wardrobe(payload.get("sub"), hash)
        return {"ok": True}

    @router.get("/public/skin-library")
    async def get_skin_library(
        cursor: str | None = None,
        limit: int = 20,
        texture_type: Optional[str] = None,
        q: str | None = None,
    ):
        """获取公开皮肤库（支持游标分页、类型过滤与名称搜索）"""
        return await site_backend.get_public_skin_library(
            cursor, clamp_limit(limit), texture_type,
            query=q.strip() if q and q.strip() else None,
        )

    @router.post("/me/textures/{hash}/apply")
    async def apply_texture_to_profile(
        hash: str, payload: dict = Depends(get_current_user), body: dict = Body(...)
    ):
        user_id = payload.get("sub")
        profile_id = body.get("profile_id")
        texture_type = body.get("texture_type")
        try:
            await site_backend.apply_texture_to_profile(
                user_id, profile_id, hash, texture_type
            )
            return {"ok": True}
        except ValueError as e:
            raise HTTPException(status_code=403, detail=str(e))

    @router.post("/textures/upload")
    async def textures_upload(
        payload: dict = Depends(get_current_user),  # Enforce JWT auth
        file: UploadFile = File(...),
        uuid: str = Form(...),
        texture_type: str = Form(...),
        model: str = Form(""),
        is_public: str = Form("false"),
    ):
        """
        前端直接上传材质接口.
        此接口现在是 Web API 的一部分，强制使用 JWT 进行身份验证。
        """
        content = await file.read()
        public_bool = is_public.lower() == "true"
        try:
            return await site_backend.upload_and_apply_texture(
                payload.get("sub"), uuid, content, texture_type, model, public_bool
            )
        except ValueError as e:
            raise HTTPException(status_code=403, detail=str(e))

    @router.get("/public/settings")
    async def get_public_settings():
        return await settings_backend.get_public_settings()

    @router.get("/public/carousel")
    async def get_carousel():
        return await site_backend.list_carousel_images()

    return router
