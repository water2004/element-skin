"""皮肤站主模块路由（用户、管理、材质等）"""

from fastapi import (
    APIRouter,
    Request,
    HTTPException,
    Depends,
    Body,
    UploadFile,
    File,
    Form,
    Header,
)
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
from fastapi.responses import Response
import os
import uuid

from utils.jwt_utils import decode_jwt_token
from database_module import Database
from config_loader import Config

router = APIRouter()
security = HTTPBearer()


def setup_routes(db: Database, backend, rate_limiter, config: Config):
    """设置路由（注入依赖）"""

    site_backend = backend

    async def get_current_user(creds: HTTPAuthorizationCredentials = Depends(security)):
        token = creds.credentials
        payload = decode_jwt_token(token)
        if not payload:
            raise HTTPException(status_code=401, detail="invalid or expired token")
        return payload

    def admin_required(payload: dict = Depends(get_current_user)):
        if not payload.get("is_admin"):
            raise HTTPException(status_code=403, detail="admin required")
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
        invite = req.get("invite")
        code = req.get("code")

        if not email or not password:
            raise HTTPException(status_code=400, detail="email and password required")

        user_id = await site_backend.register(email, password, invite, code)
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

    @router.post("/me/profiles")
    async def create_profile(
        payload: dict = Depends(get_current_user), body: dict = Body(...)
    ):
        return await site_backend.create_profile(
            payload.get("sub"), body.get("name"), body.get("model", "default")
        )

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
    ):
        user_id = payload.get("sub")
        content = await file.read()
        try:
            texture_hash, texture_type = await db.texture.upload(
                user_id, content, texture_type, note
            )
            return {"hash": texture_hash, "type": texture_type, "note": note}
        except ValueError as e:
            raise HTTPException(status_code=400, detail=str(e))

    @router.get("/me/textures")
    async def list_my_textures(payload: dict = Depends(get_current_user)):
        textures = await db.texture.get_for_user(payload.get("sub"))
        return [
            {"hash": r[0], "type": r[1], "note": r[2], "created_at": r[3]}
            for r in textures
        ]

    @router.patch("/me/textures/{hash}/{texture_type}")
    async def update_my_texture_note(
        hash: str,
        texture_type: str,
        payload: dict = Depends(get_current_user),
        body: dict = Body(...),
    ):
        user_id = payload.get("sub")
        note = body.get("note", "")
        await db.texture.update_note(user_id, hash, texture_type, note)
        return {"ok": True, "hash": hash, "texture_type": texture_type, "note": note}

    @router.delete("/me/textures/{hash}/{texture_type}")
    async def delete_my_texture(
        hash: str, texture_type: str, payload: dict = Depends(get_current_user)
    ):
        await db.texture.delete_from_library(payload.get("sub"), hash, texture_type)
        return {"ok": True}

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
    ):
        """
        前端直接上传材质接口.
        此接口现在是 Web API 的一部分，强制使用 JWT 进行身份验证。
        """
        content = await file.read()
        user_id = payload.get("sub")

        try:
            # 1. 上传材质到用户库 (或直接保存文件)
            texture_hash, _ = await db.texture.upload(
                user_id, content, texture_type, f"Direct upload to profile {uuid}"
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
        return {
            "site_name": settings.get("site_name", "皮肤站"),
            "site_url": settings.get("site_url", ""),
            "allow_register": settings.get("allow_register", "true") == "true",
            "email_verify_enabled": settings.get("email_verify_enabled", "false") == "true",
            "mojang_status_urls": {
                "session": settings.get(
                    "mojang_session_url", "https://sessionserver.mojang.com"
                ),
                "account": settings.get("mojang_account_url", "https://api.mojang.com"),
                "services": settings.get(
                    "mojang_services_url", "https://api.minecraftservices.com"
                ),
            },
        }

    @router.get("/admin/settings")
    async def get_admin_settings(payload: dict = Depends(admin_required)):
        return await site_backend.get_admin_settings()

    @router.post("/admin/settings")
    async def save_admin_settings(
        payload: dict = Depends(admin_required), body: dict = Body(...)
    ):
        await site_backend.save_admin_settings(body)
        return {"ok": True}

    @router.get("/admin/users")
    async def get_admin_users(payload: dict = Depends(admin_required)):
        return await site_backend.get_admin_users()

    @router.post("/admin/users/{user_id}/toggle-admin")
    async def toggle_user_admin(user_id: str, payload: dict = Depends(admin_required)):
        actor_id = payload.get("sub")
        await site_backend.toggle_user_admin(user_id, actor_id)
        return {"ok": True}

    @router.delete("/admin/users/{user_id}")
    async def delete_user_admin(user_id: str, payload: dict = Depends(admin_required)):
        await site_backend.delete_user(user_id, is_admin_action=True)
        return {"ok": True}

    @router.post("/admin/users/{user_id}/ban")
    async def ban_user(
        user_id: str, payload: dict = Depends(admin_required), body: dict = Body(...)
    ):
        banned_until = body.get("banned_until")
        if banned_until is None:
            raise HTTPException(status_code=400, detail="banned_until is required")
        res = await site_backend.ban_user(user_id, banned_until, payload.get("sub"))
        return {"ok": True, "banned_until": res}

    @router.post("/admin/users/{user_id}/unban")
    async def unban_user(user_id: str, payload: dict = Depends(admin_required)):
        await db.user.unban(user_id)
        return {"ok": True}

    @router.get("/admin/invites")
    async def get_admin_invites(payload: dict = Depends(admin_required)):
        invites = await db.user.list_invites()
        return [
            {
                "code": row.code,
                "created_at": row.created_at,
                "used_by": row.used_by,
                "total_uses": row.total_uses,
                "used_count": row.used_count,
                "note": row.note,
            }
            for row in invites
        ]

    @router.post("/admin/invites")
    async def create_admin_invite(
        payload: dict = Depends(admin_required), body: dict = Body(None)
    ):
        code = body.get("code") if body else None
        total_uses = body.get("total_uses", 1) if body else 1
        note = body.get("note", "") if body else ""
        new_code = await site_backend.create_invite(code, total_uses, note)
        return {"code": new_code, "total_uses": total_uses, "note": note}

    @router.delete("/admin/invites/{code}")
    async def delete_admin_invite(code: str, payload: dict = Depends(admin_required)):
        await db.user.delete_invite(code)
        return {"ok": True}

    @router.get("/admin/official-whitelist")
    async def get_official_whitelist(payload: dict = Depends(admin_required)):
        return await site_backend.get_official_whitelist()

    @router.post("/admin/official-whitelist")
    async def add_official_whitelist(payload: dict = Depends(admin_required), body: dict = Body(...)):
        username = body.get("username")
        return await site_backend.add_official_whitelist_user(username)

    @router.delete("/admin/official-whitelist/{username}")
    async def remove_official_whitelist(username: str, payload: dict = Depends(admin_required)):
        return await site_backend.remove_official_whitelist_user(username)

    @router.get("/public/carousel")
    async def get_carousel():
        return await site_backend.list_carousel_images()

    @router.post("/admin/carousel")
    async def upload_carousel(
        file: UploadFile = File(...),
        payload: dict = Depends(admin_required)
    ):
        ext = os.path.splitext(file.filename)[1].lower()
        if ext not in [".png", ".jpg", ".jpeg", ".webp"]:
            raise HTTPException(status_code=400, detail="Unsupported file format")
        
        filename = f"{uuid.uuid4().hex}{ext}"
        content = await file.read()
        return await site_backend.upload_carousel_image(filename, content)

    @router.delete("/admin/carousel/{filename}")
    async def delete_carousel(filename: str, payload: dict = Depends(admin_required)):
        return await site_backend.delete_carousel_image(filename)

    return router
