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
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
from typing import Optional

from utils.jwt_utils import decode_jwt_token
from database_module import Database
from config_loader import Config

router = APIRouter()
security = HTTPBearer()


def setup_routes(db: Database, site_backend, rate_limiter, config: Config):
    """设置路由（注入依赖）"""

    async def get_current_user(creds: HTTPAuthorizationCredentials = Depends(security)):
        token = creds.credentials
        payload = decode_jwt_token(token)
        if not payload:
            raise HTTPException(status_code=401, detail="invalid or expired token")
        return payload

    @router.post("/site-login")
    async def site_login(req: dict, request: Request):
        await rate_limiter.check(request, is_auth_endpoint=True)
        result = await site_backend.login(req.get("email"), req.get("password"))
        rate_limiter.reset(request.client.host, request.url.path)
        return result

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
    async def refresh_jwt(payload: dict = Depends(get_current_user)):
        return await site_backend.refresh_token(payload.get("sub"))

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
        return await site_backend.get_ygg_profiles(
            body.get("api_url"), body.get("username"), body.get("password")
        )

    @router.post("/remote-ygg/import-profile")
    async def import_ygg_profile(
        payload: dict = Depends(get_current_user), body: dict = Body(...)
    ):
        return await site_backend.import_ygg_profile(
            payload.get("sub"),
            body.get("api_url"),
            body.get("profile_id"),
            body.get("profile_name"),
        )

    @router.post("/remote-ygg/import-profiles")
    async def import_ygg_profiles(
        payload: dict = Depends(get_current_user), body: dict = Body(...)
    ):
        return await site_backend.import_ygg_profiles(
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
            texture_hash, texture_type = await db.texture.upload(
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
        from utils.pagination import CursorEncoder

        user_id = payload.get("sub")

        last_created_at = None
        last_hash = None
        if cursor:
            cursor_data = CursorEncoder.decode(cursor)
            if not cursor_data or "last_created_at" not in cursor_data or "last_hash" not in cursor_data:
                raise HTTPException(status_code=400, detail="Invalid cursor")
            last_created_at = cursor_data["last_created_at"]
            last_hash = cursor_data["last_hash"]

        return await db.texture.get_for_user_cursor(
            user_id,
            texture_type=texture_type,
            limit=limit,
            last_created_at=last_created_at,
            last_hash=last_hash,
        )

    @router.get("/me/profiles")
    async def list_my_profiles(
        cursor: str | None = None,
        limit: int = 20,
        payload: dict = Depends(get_current_user)
    ):
        """获取我的角色列表（仅支持游标分页）"""
        from utils.pagination import CursorEncoder

        user_id = payload.get("sub")

        last_id = None
        if cursor:
            cursor_data = CursorEncoder.decode(cursor)
            if not cursor_data or "last_id" not in cursor_data:
                raise HTTPException(status_code=400, detail="Invalid cursor")
            last_id = cursor_data["last_id"]

        result = await db.user.get_profiles_by_user_cursor(user_id, limit=limit, last_id=last_id)
        profiles_list = result["items"]
        return {
            "items": [
                {
                    "id": p.id,
                    "name": p.name,
                    "model": p.texture_model,
                    "skin_hash": p.skin_hash,
                    "cape_hash": p.cape_hash,
                }
                for p in profiles_list
            ],
            "has_next": result["has_next"],
            "next_cursor": result["next_cursor"],
            "page_size": result["page_size"],
        }

    @router.get("/me/textures/{hash}/{texture_type}")
    async def get_my_texture_detail(
        hash: str,
        texture_type: str,
        payload: dict = Depends(get_current_user)
    ):
        user_id = payload.get("sub")
        info = await db.texture.get_texture_info(user_id, hash, texture_type)
        if not info:
            raise HTTPException(status_code=404, detail="Texture not found")
        return info

    @router.patch("/me/textures/{hash}/{texture_type}")
    async def update_my_texture(
        hash: str,
        texture_type: str,
        payload: dict = Depends(get_current_user),
        body: dict = Body(...),
    ):
        user_id = payload.get("sub")
        if "note" in body:
            await db.texture.update_note(user_id, hash, texture_type, body["note"])
        if "model" in body:
            await db.texture.update_model(user_id, hash, texture_type, body["model"])
        if "is_public" in body:
            await db.texture.update_is_public(user_id, hash, texture_type, body["is_public"])
        
        info = await db.texture.get_texture_info(user_id, hash, texture_type)
        return {"ok": True, **info}

    @router.delete("/me/textures/{hash}/{texture_type}")
    async def delete_my_texture(
        hash: str, texture_type: str, payload: dict = Depends(get_current_user)
    ):
        await db.texture.delete_from_library(payload.get("sub"), hash, texture_type)
        return {"ok": True}

    @router.post("/me/textures/{hash}/add")
    async def add_texture_to_wardrobe(
        hash: str, payload: dict = Depends(get_current_user)
    ):
        success = await db.texture.add_to_user_wardrobe(payload.get("sub"), hash)
        if not success:
            raise HTTPException(status_code=404, detail="Texture not found in library")
        return {"ok": True}

    @router.get("/public/skin-library")
    async def get_skin_library(
        cursor: str | None = None,
        limit: int = 20,
        texture_type: Optional[str] = None
    ):
        """获取公开皮肤库（仅支持游标分页）"""
        from utils.pagination import CursorEncoder

        enabled = await db.setting.get("enable_skin_library", "true")
        if enabled != "true":
            raise HTTPException(status_code=403, detail="Skin library is disabled by administrator")

        last_created_at = None
        last_skin_hash = None
        if cursor:
            cursor_data = CursorEncoder.decode(cursor)
            if not cursor_data or "last_created_at" not in cursor_data or "last_skin_hash" not in cursor_data:
                raise HTTPException(status_code=400, detail="Invalid cursor")
            last_created_at = cursor_data["last_created_at"]
            last_skin_hash = cursor_data["last_skin_hash"]

        result = await db.texture.get_from_library_cursor(
            limit=limit,
            texture_type=texture_type,
            only_public=True,
            last_created_at=last_created_at,
            last_skin_hash=last_skin_hash,
        )
        items_list = result["items"]
        uploader_ids = list(set(item.get("uploader") for item in items_list if item.get("uploader")))
        uploader_names = await db.user.get_display_names_by_ids(uploader_ids)
        
        return {
            "items": [
                {
                    **item,
                    "uploader_name": uploader_names.get(item.get("uploader"), "")
                }
                for item in items_list
            ],
            "has_next": result["has_next"],
            "next_cursor": result["next_cursor"],
            "page_size": result["page_size"],
        }

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
        user_id = payload.get("sub")
        public_bool = is_public.lower() == "true"

        try:
            # 1. 上传材质到用户库 (或直接保存文件)
            texture_hash, _ = await db.texture.upload(
                user_id, content, texture_type, f"Direct upload to profile {uuid}", is_public=public_bool
            )

            # 2. 应用到角色
            await site_backend.apply_texture_to_profile(
                user_id, uuid, texture_hash, texture_type
            )

            # 3. 如果是皮肤，则更新模型
            if texture_type.lower() == "skin":
                m_val = "slim" if model == "slim" else "default"
                await db.user.update_profile_texture_model(uuid, m_val)

            return {"ok": True}
        except ValueError as e:
            raise HTTPException(status_code=403, detail=str(e))
        except Exception as e:
            print(f"Error during direct texture upload: {e}")
            raise HTTPException(
                status_code=500,
                detail="An unexpected error occurred during texture upload.",
            )

    @router.get("/public/settings")
    async def get_public_settings():
        settings = await db.setting.get_all()
        fallbacks = await site_backend.get_fallback_services()
        primary = fallbacks[0] if fallbacks else None

        return {
            "site_name": settings.get("site_name", "皮肤站"),
            "site_subtitle": settings.get("site_subtitle", "简洁、高效、现代的 Minecraft 皮肤管理站"),
            "allow_register": settings.get("allow_register", "true") == "true",
            "enable_skin_library": settings.get("enable_skin_library", "true") == "true",
            "email_verify_enabled": settings.get("email_verify_enabled", "false") == "true",
            "footer_text": settings.get("footer_text", ""),
            "filing_icp": settings.get("filing_icp", ""),
            "filing_icp_link": settings.get("filing_icp_link", ""),
            "filing_mps": settings.get("filing_mps", ""),
            "filing_mps_link": settings.get("filing_mps_link", ""),
            "mojang_status_urls": {
                "session": (primary or {}).get(
                    "session_url", "https://sessionserver.mojang.com"
                ),
                "account": (primary or {}).get(
                    "account_url", "https://api.mojang.com"
                ),
                "services": (primary or {}).get(
                    "services_url", "https://api.minecraftservices.com"
                ),
            },
        }

    @router.get("/public/carousel")
    async def get_carousel():
        return await site_backend.list_carousel_images()

    return router
