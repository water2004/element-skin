"""管理员模块路由"""

from fastapi import (
    APIRouter,
    HTTPException,
    Depends,
    Body,
    UploadFile,
    File,
)
import os

from routers.deps import admin_required
from utils.uuid_utils import generate_random_uuid

router = APIRouter()


def setup_routes(admin_backend, settings_backend):
    """设置路由（注入依赖）"""

    # ========== Settings (Granular) ==========

    @router.get("/admin/settings/site")
    async def get_site_settings(payload: dict = Depends(admin_required)):
        return await settings_backend.get_site_settings()

    @router.post("/admin/settings/site")
    async def save_site_settings(payload: dict = Depends(admin_required), body: dict = Body(...)):
        await settings_backend.save_settings_group("site", body)
        return {"ok": True}

    @router.get("/admin/settings/security")
    async def get_security_settings(payload: dict = Depends(admin_required)):
        return await settings_backend.get_security_settings()

    @router.post("/admin/settings/security")
    async def save_security_settings(payload: dict = Depends(admin_required), body: dict = Body(...)):
        await settings_backend.save_settings_group("security", body)
        return {"ok": True}

    @router.get("/admin/settings/auth")
    async def get_auth_settings(payload: dict = Depends(admin_required)):
        return await settings_backend.get_auth_settings()

    @router.post("/admin/settings/auth")
    async def save_auth_settings(payload: dict = Depends(admin_required), body: dict = Body(...)):
        await settings_backend.save_settings_group("auth", body)
        return {"ok": True}

    @router.get("/admin/settings/microsoft")
    async def get_microsoft_settings(payload: dict = Depends(admin_required)):
        return await settings_backend.get_microsoft_settings()

    @router.post("/admin/settings/microsoft")
    async def save_microsoft_settings(payload: dict = Depends(admin_required), body: dict = Body(...)):
        await settings_backend.save_settings_group("microsoft", body)
        return {"ok": True}

    @router.get("/admin/settings/email")
    async def get_email_settings(payload: dict = Depends(admin_required)):
        return await settings_backend.get_email_settings()

    @router.post("/admin/settings/email")
    async def save_email_settings(payload: dict = Depends(admin_required), body: dict = Body(...)):
        await settings_backend.save_settings_group("email", body)
        return {"ok": True}

    @router.get("/admin/settings/fallback")
    async def get_fallback_settings(payload: dict = Depends(admin_required)):
        return await settings_backend.get_fallback_settings()

    @router.post("/admin/settings/fallback")
    async def save_fallback_settings(payload: dict = Depends(admin_required), body: dict = Body(...)):
        # save_settings_group("fallback") persists both the strategy and endpoints
        await settings_backend.save_settings_group("fallback", body)
        return {"ok": True}

    # ========== Users ==========

    @router.get("/admin/users")
    async def get_admin_users(
        cursor: str | None = None,
        limit: int = 15,
        q: str | None = None,
        payload: dict = Depends(admin_required)
    ):
        """获取用户列表（支持搜索和游标分页）"""
        return await admin_backend.list_users(cursor, limit, q)

    @router.get("/admin/users/{user_id}")
    async def get_single_user_admin(user_id: str, payload: dict = Depends(admin_required)):
        return await admin_backend.get_user_info(user_id)

    @router.get("/admin/users/{user_id}/profiles")
    async def get_user_profiles_admin(
        user_id: str,
        cursor: str | None = None,
        limit: int = 20,
        payload: dict = Depends(admin_required)
    ):
        """获取用户的角色列表（仅支持游标分页）"""
        return await admin_backend.get_user_profiles(user_id, cursor, limit)

    @router.post("/admin/users/{user_id}/toggle-admin")
    async def toggle_user_admin(user_id: str, payload: dict = Depends(admin_required)):
        actor_id = payload.get("sub")
        await admin_backend.toggle_user_admin(user_id, actor_id)
        return {"ok": True}

    @router.delete("/admin/users/{user_id}")
    async def delete_user_admin(user_id: str, payload: dict = Depends(admin_required)):
        await admin_backend.delete_user(user_id, is_admin_action=True)
        return {"ok": True}

    @router.post("/admin/users/{user_id}/ban")
    async def ban_user(
        user_id: str, payload: dict = Depends(admin_required), body: dict = Body(...)
    ):
        banned_until = body.get("banned_until")
        if banned_until is None:
            raise HTTPException(status_code=400, detail="banned_until is required")
        res = await admin_backend.ban_user(user_id, banned_until, payload.get("sub"))
        return {"ok": True, "banned_until": res}

    @router.post("/admin/users/{user_id}/unban")
    async def unban_user(user_id: str, payload: dict = Depends(admin_required)):
        await admin_backend.unban_user(user_id)
        return {"ok": True}

    @router.post("/admin/users/reset-password")
    async def reset_user_password(payload: dict = Depends(admin_required), body: dict = Body(...)):
        user_id = body.get("user_id")
        new_password = body.get("new_password")
        if not user_id or not new_password:
            raise HTTPException(status_code=400, detail="user_id and new_password required")
        return await admin_backend.reset_user_password(user_id, new_password)

    # ========== Admin Profile Management ==========

    @router.get("/admin/profiles")
    async def get_admin_profiles(
        cursor: str | None = None,
        limit: int = 20,
        q: str | None = None,
        payload: dict = Depends(admin_required)
    ):
        """获取所有角色列表（支持搜索和游标分页）"""
        return await admin_backend.get_all_profiles(
            limit=limit,
            cursor=cursor,
            query=q.strip() if q and q.strip() else None,
        )

    @router.patch("/admin/profiles/{profile_id}")
    async def update_admin_profile(
        profile_id: str,
        payload: dict = Depends(admin_required),
        body: dict = Body(...)
    ):
        return await admin_backend.update_profile(
            profile_id,
            name=body.get("name"),
        )

    @router.delete("/admin/profiles/{profile_id}")
    async def delete_admin_profile(
        profile_id: str,
        payload: dict = Depends(admin_required)
    ):
        return await admin_backend.delete_profile(profile_id)

    @router.patch("/admin/profiles/{profile_id}/skin")
    async def update_admin_profile_skin(
        profile_id: str,
        payload: dict = Depends(admin_required),
        body: dict = Body(...)
    ):
        return await admin_backend.update_profile_skin(
            profile_id,
            skin_hash=body.get("hash"),
        )

    @router.patch("/admin/profiles/{profile_id}/cape")
    async def update_admin_profile_cape(
        profile_id: str,
        payload: dict = Depends(admin_required),
        body: dict = Body(...)
    ):
        return await admin_backend.update_profile_cape(
            profile_id,
            cape_hash=body.get("hash"),
        )

    # ========== Admin Texture Management ==========

    @router.get("/admin/textures")
    async def get_admin_textures(
        cursor: str | None = None,
        limit: int = 20,
        q: str | None = None,
        type: str | None = None,
        payload: dict = Depends(admin_required)
    ):
        """获取所有材质列表（支持搜索、类型过滤和游标分页）"""
        return await admin_backend.get_all_textures(
            limit=limit,
            cursor=cursor,
            query=q.strip() if q and q.strip() else None,
            type_filter=type,
        )

    @router.patch("/admin/textures/{hash}")
    async def update_admin_texture(
        hash: str,
        payload: dict = Depends(admin_required),
        body: dict = Body(...)
    ):
        updated = False
        if "model" in body:
            await admin_backend.update_texture_model(hash, body["model"])
            updated = True
        if "note" in body:
            await admin_backend.update_texture_note(hash, body["note"])
            updated = True
        if "is_public" in body:
            await admin_backend.update_texture_public(hash, body["is_public"])
            updated = True

        if not updated:
            raise HTTPException(status_code=400, detail="至少需要一个更新字段: model, note, is_public")

        return {"ok": True}

    @router.delete("/admin/textures/{hash}")
    async def delete_admin_texture(
        hash: str,
        type: str = "skin",
        user_id: str | None = None,
        force: bool = False,
        payload: dict = Depends(admin_required)
    ):
        return await admin_backend.delete_texture(hash, type, user_id, force)

    # ========== Invites ==========

    @router.get("/admin/invites")
    async def get_admin_invites(
        cursor: str | None = None,
        limit: int = 15,
        payload: dict = Depends(admin_required)
    ):
        """获取邀请码列表（仅支持游标分页）"""
        return await admin_backend.list_invites(cursor, limit)

    @router.post("/admin/invites")
    async def create_admin_invite(
        payload: dict = Depends(admin_required), body: dict = Body(None)
    ):
        code = body.get("code") if body else None
        total_uses = body.get("total_uses", 1) if body else 1
        note = body.get("note", "") if body else ""
        new_code = await admin_backend.create_invite(code, total_uses, note)
        return {"code": new_code, "total_uses": total_uses, "note": note}

    @router.delete("/admin/invites/{code}")
    async def delete_admin_invite(code: str, payload: dict = Depends(admin_required)):
        return await admin_backend.delete_invite(code)

    # ========== Fallback Whitelist ==========

    @router.get("/admin/official-whitelist")
    async def get_official_whitelist(endpoint_id: int, payload: dict = Depends(admin_required)):
        return await admin_backend.get_official_whitelist(endpoint_id)

    @router.post("/admin/official-whitelist")
    async def add_official_whitelist(payload: dict = Depends(admin_required), body: dict = Body(...)):
        username = body.get("username")
        endpoint_id = body.get("endpoint_id")
        if endpoint_id is None:
            raise HTTPException(status_code=400, detail="endpoint_id is required")
        return await admin_backend.add_official_whitelist_user(username, endpoint_id)

    @router.delete("/admin/official-whitelist/{username}")
    async def remove_official_whitelist(username: str, endpoint_id: int, payload: dict = Depends(admin_required)):
        return await admin_backend.remove_official_whitelist_user(username, endpoint_id)

    # ========== Carousel ==========

    @router.post("/admin/carousel")
    async def upload_carousel(
        file: UploadFile = File(...),
        payload: dict = Depends(admin_required)
    ):
        ext = os.path.splitext(file.filename)[1].lower()
        if ext not in [".png", ".jpg", ".jpeg", ".webp"]:
            raise HTTPException(status_code=400, detail="Unsupported file format")
        
        filename = f"{generate_random_uuid()}{ext}"
        content = await file.read()
        return await admin_backend.upload_carousel_image(filename, content)

    @router.delete("/admin/carousel/{filename}")
    async def delete_carousel(filename: str, payload: dict = Depends(admin_required)):
        return await admin_backend.delete_carousel_image(filename)

    return router
