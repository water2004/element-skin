"""管理员模块路由"""

from fastapi import (
    APIRouter,
    HTTPException,
    Depends,
    Body,
    UploadFile,
    File,
)
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
import os
import uuid

from utils.jwt_utils import decode_jwt_token
from database_module import Database
from config_loader import Config

router = APIRouter()
security = HTTPBearer()


def setup_routes(db: Database, admin_backend, rate_limiter, config: Config):
    """设置路由（注入依赖）"""

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

    # ========== Settings (Granular) ==========

    @router.get("/admin/settings/site")
    async def get_site_settings(payload: dict = Depends(admin_required)):
        return await admin_backend.get_site_settings()

    @router.post("/admin/settings/site")
    async def save_site_settings(payload: dict = Depends(admin_required), body: dict = Body(...)):
        await admin_backend.save_settings_group("site", body)
        return {"ok": True}

    @router.get("/admin/settings/security")
    async def get_security_settings(payload: dict = Depends(admin_required)):
        return await admin_backend.get_security_settings()

    @router.post("/admin/settings/security")
    async def save_security_settings(payload: dict = Depends(admin_required), body: dict = Body(...)):
        await admin_backend.save_settings_group("security", body)
        return {"ok": True}

    @router.get("/admin/settings/auth")
    async def get_auth_settings(payload: dict = Depends(admin_required)):
        return await admin_backend.get_auth_settings()

    @router.post("/admin/settings/auth")
    async def save_auth_settings(payload: dict = Depends(admin_required), body: dict = Body(...)):
        await admin_backend.save_settings_group("auth", body)
        return {"ok": True}

    @router.get("/admin/settings/microsoft")
    async def get_microsoft_settings(payload: dict = Depends(admin_required)):
        return await admin_backend.get_microsoft_settings()

    @router.post("/admin/settings/microsoft")
    async def save_microsoft_settings(payload: dict = Depends(admin_required), body: dict = Body(...)):
        await admin_backend.save_settings_group("microsoft", body)
        return {"ok": True}

    @router.get("/admin/settings/email")
    async def get_email_settings(payload: dict = Depends(admin_required)):
        return await admin_backend.get_email_settings()

    @router.post("/admin/settings/email")
    async def save_email_settings(payload: dict = Depends(admin_required), body: dict = Body(...)):
        await admin_backend.save_settings_group("email", body)
        return {"ok": True}

    @router.get("/admin/settings/fallback")
    async def get_fallback_settings(payload: dict = Depends(admin_required)):
        return await admin_backend.get_fallback_settings()

    @router.post("/admin/settings/fallback")
    async def save_fallback_settings(payload: dict = Depends(admin_required), body: dict = Body(...)):
        # This handles both the strategy and the endpoints
        await admin_backend.save_settings_group("fallback", body)
        if "fallbacks" in body:
            await admin_backend.save_settings_group("fallback_endpoints", body)
        return {"ok": True}

    # ========== Legacy compatibility ==========

    @router.get("/admin/settings")
    async def get_admin_settings(payload: dict = Depends(admin_required)):
        return await admin_backend.get_admin_settings()

    @router.post("/admin/settings")
    async def save_admin_settings(
        payload: dict = Depends(admin_required), body: dict = Body(...)
    ):
        await admin_backend.save_admin_settings(body)
        return {"ok": True}

    # ========== Users ==========

    @router.get("/admin/users")
    async def get_admin_users(
        cursor: str | None = None,
        limit: int = 15,
        payload: dict = Depends(admin_required)
    ):
        """获取用户列表（仅支持游标分页）"""
        from utils.pagination import CursorEncoder

        last_id = None
        if cursor:
            cursor_data = CursorEncoder.decode(cursor)
            if not cursor_data or "last_id" not in cursor_data:
                raise HTTPException(status_code=400, detail="Invalid cursor")
            last_id = cursor_data["last_id"]

        return await db.user.list_users_cursor(limit=limit, last_id=last_id)

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
        from utils.pagination import CursorEncoder

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
        await db.user.unban(user_id)
        return {"ok": True}

    @router.post("/admin/users/reset-password")
    async def reset_user_password(payload: dict = Depends(admin_required), body: dict = Body(...)):
        user_id = body.get("user_id")
        new_password = body.get("new_password")
        if not user_id or not new_password:
            raise HTTPException(status_code=400, detail="user_id and new_password required")
        return await admin_backend.reset_user_password(user_id, new_password)

    # ========== Invites ==========

    @router.get("/admin/invites")
    async def get_admin_invites(
        cursor: str | None = None,
        limit: int = 15,
        payload: dict = Depends(admin_required)
    ):
        """获取邀请码列表（仅支持游标分页）"""
        from utils.pagination import CursorEncoder

        last_created_at = None
        last_code = None
        if cursor:
            cursor_data = CursorEncoder.decode(cursor)
            if not cursor_data or "last_created_at" not in cursor_data or "last_code" not in cursor_data:
                raise HTTPException(status_code=400, detail="Invalid cursor")
            last_created_at = cursor_data["last_created_at"]
            last_code = cursor_data["last_code"]

        result = await db.user.list_invites_cursor(
            limit=limit,
            last_created_at=last_created_at,
            last_code=last_code,
        )
        return {
            "items": [
                {
                    "code": row.code,
                    "created_at": row.created_at,
                    "used_by": row.used_by,
                    "total_uses": row.total_uses,
                    "used_count": row.used_count,
                    "note": row.note,
                }
                for row in result["items"]
            ],
            "has_next": result["has_next"],
            "next_cursor": result["next_cursor"],
            "page_size": result["page_size"],
        }

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
        await db.user.delete_invite(code)
        return {"ok": True}

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
        
        filename = f"{uuid.uuid4().hex}{ext}"
        content = await file.read()
        return await admin_backend.upload_carousel_image(filename, content)

    @router.delete("/admin/carousel/{filename}")
    async def delete_carousel(filename: str, payload: dict = Depends(admin_required)):
        return await admin_backend.delete_carousel_image(filename)

    return router
