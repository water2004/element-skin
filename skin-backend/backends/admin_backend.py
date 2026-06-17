from typing import Any, Dict
import time
import secrets
import os
import re
from fastapi import HTTPException

from utils.typing import InviteCode, serialize_profile_summary
from utils.pagination import decode_cursor, encode_next
from utils.profile_naming import is_valid_profile_name
from utils.password_utils import hash_password_async
from database_module import Database
from config_loader import Config

class AdminBackend:
    def __init__(self, db: Database, config: Config):
        self.db = db
        self.config = config

    # ========== Other Methods ==========

    async def get_user_info(self, user_id: str) -> Dict[str, Any]:
        user_row = await self.db.user.get_by_id(user_id)
        if not user_row:
            raise HTTPException(status_code=404, detail="user not found")

        profile_count = await self.db.user.count_profiles_by_user(user_id)
        texture_count = await self.db.texture.count_for_user(user_id)

        return {
            "id": user_row.id,
            "email": user_row.email,
            "lang": user_row.preferred_language,
            "display_name": user_row.display_name,
            "is_admin": bool(user_row.is_admin),
            "banned_until": user_row.banned_until,
            "avatar_hash": user_row.avatar_hash,
            "profile_count": profile_count,
            "texture_count": texture_count,
        }

    async def toggle_user_admin(self, user_id: str, actor_id: str):
        if actor_id == user_id:
            raise HTTPException(status_code=403, detail="cannot change own admin status")
        new_status = await self.db.user.toggle_admin(user_id)
        if new_status == -1:
            raise HTTPException(status_code=404, detail="user not found")

    async def delete_user(self, user_id: str, is_admin_action=False):
        user_row = await self.db.user.get_by_id(user_id)
        if not user_row:
            raise HTTPException(status_code=404, detail="user not found")
        if user_row.is_admin and is_admin_action:
            raise HTTPException(status_code=403, detail="cannot delete admin user")
        await self.db.user.delete(user_id)
        return True

    async def ban_user(self, user_id, banned_until, actor_id):
        user_row = await self.db.user.get_by_id(user_id)
        if not user_row:
            raise HTTPException(status_code=404, detail="user not found")
        if user_row.is_admin:
            raise HTTPException(status_code=403, detail="cannot ban admin user")
        await self.db.user.ban(user_id, banned_until)
        return banned_until

    async def unban_user(self, user_id: str):
        await self.db.user.unban(user_id)

    async def list_users(self, cursor: str | None, limit: int, query: str | None) -> dict:
        try:
            key = decode_cursor(cursor, ("last_id",))
        except ValueError:
            raise HTTPException(status_code=400, detail="Invalid cursor")
        last_id = (key or {}).get("last_id")
        if query and query.strip():
            result = await self.db.user.search_users_cursor(query=query.strip(), limit=limit, last_id=last_id)
        else:
            result = await self.db.user.list_users_cursor(limit=limit, last_id=last_id)
        result["next_cursor"] = encode_next(result.pop("next_key"))
        return result

    async def get_user_profiles(self, user_id: str, cursor: str | None, limit: int) -> dict:
        try:
            key = decode_cursor(cursor, ("last_id",))
        except ValueError:
            raise HTTPException(status_code=400, detail="Invalid cursor")
        result = await self.db.user.get_profiles_by_user_cursor(
            user_id, limit=limit, last_id=(key or {}).get("last_id")
        )
        return {
            "items": [serialize_profile_summary(p) for p in result["items"]],
            "has_next": result["has_next"],
            "next_cursor": encode_next(result["next_key"]),
            "page_size": result["page_size"],
        }

    # ========== Profile Management (Admin) ==========

    async def get_all_profiles(self, limit: int = 20, cursor: str | None = None, query: str | None = None) -> dict:
        try:
            key = decode_cursor(cursor, ("last_id",))
        except ValueError:
            raise HTTPException(status_code=400, detail="Invalid cursor")
        result = await self.db.user.list_all_profiles_cursor(
            limit, after_id=(key or {}).get("last_id"), query=query
        )
        result["next_cursor"] = encode_next(result.pop("next_key"))
        return result

    async def update_profile(self, profile_id: str, name: str | None = None) -> dict:
        # 业务验证
        if name is not None:
            if not is_valid_profile_name(name):
                raise HTTPException(status_code=400, detail="角色名只能包含字母、数字、下划线，长度 1-16 字符")
        # 编排 DB 操作
        if name is not None:
            ok = await self.db.user.update_profile_name(profile_id, name)
            if not ok:
                raise HTTPException(status_code=409, detail="角色名已被占用")
        
        return {"ok": True}

    async def delete_profile(self, profile_id: str) -> dict:
        # 检查存在性
        profile = await self.db.user.get_profile_by_id(profile_id)
        if not profile:
            raise HTTPException(status_code=404, detail="角色不存在")
        
        # 事务内级联删除角色及其 Yggdrasil token，避免孤儿 token
        ok = await self.db.user.delete_profile_cascade(profile_id)
        if not ok:
            raise HTTPException(status_code=404, detail="角色不存在")

        return {"ok": True}

    async def update_profile_skin(self, profile_id: str, skin_hash: str | None = None) -> dict:
        profile = await self.db.user.get_profile_by_id(profile_id)
        if not profile:
            raise HTTPException(status_code=404, detail="角色不存在")
        await self.db.user.update_profile_skin(profile_id, skin_hash)
        return {"ok": True}

    async def update_profile_cape(self, profile_id: str, cape_hash: str | None = None) -> dict:
        profile = await self.db.user.get_profile_by_id(profile_id)
        if not profile:
            raise HTTPException(status_code=404, detail="角色不存在")
        await self.db.user.update_profile_cape(profile_id, cape_hash)
        return {"ok": True}

    async def reset_user_password(self, user_id: str, new_password: str):
        user_row = await self.db.user.get_by_id(user_id)
        if not user_row:
            raise HTTPException(status_code=404, detail="user not found")
        
        password_hash = await hash_password_async(new_password)
        # 先撤销外部令牌；任何失败都不应改变密码
        await self.db.user.delete_tokens_by_user(user_id)
        await self.db.user.delete_refresh_tokens_by_user(user_id)
        await self.db.user.update_password(user_id, password_hash)
        return {"ok": True}

    async def list_invites(self, cursor: str | None, limit: int) -> dict:
        try:
            key = decode_cursor(cursor, ("last_created_at", "last_code"))
        except ValueError:
            raise HTTPException(status_code=400, detail="Invalid cursor")
        result = await self.db.user.list_invites_cursor(
            limit=limit,
            last_created_at=(key or {}).get("last_created_at"),
            last_code=(key or {}).get("last_code"),
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
            "next_cursor": encode_next(result["next_key"]),
            "page_size": result["page_size"],
        }

    async def delete_invite(self, code: str):
        await self.db.user.delete_invite(code)
        return {"ok": True}

    async def create_invite(self, code, total_uses, note: str = ""):
        if code:
            if not (6 <= len(code) <= 32) or not re.match(r"^[a-zA-Z0-9_-]+$", code):
                raise HTTPException(status_code=400, detail="Invalid code format")
        else:
            code = secrets.token_urlsafe(16)

        if await self.db.user.get_invite(code):
            raise HTTPException(status_code=400, detail="invite code already exists")

        await self.db.user.create_invite(InviteCode(code, int(time.time() * 1000), total_uses=total_uses, note=note))
        return code

    async def upload_carousel_image(self, filename: str, content: bytes):
        directory = self.config.get("carousel.directory", "carousel")
        os.makedirs(directory, exist_ok=True)
        with open(os.path.join(directory, filename), "wb") as f:
            f.write(content)
        return {"filename": filename}

    async def delete_carousel_image(self, filename: str):
        directory = self.config.get("carousel.directory", "carousel")
        file_path = os.path.join(directory, filename)
        if os.path.dirname(os.path.abspath(file_path)) != os.path.abspath(directory):
            raise HTTPException(status_code=400, detail="Invalid filename")
        if os.path.exists(file_path):
            os.remove(file_path)
            return {"ok": True}
        raise HTTPException(status_code=404, detail="File not found")

    async def get_official_whitelist(self, endpoint_id: int):
        return await self.db.fallback.list_whitelist_users(endpoint_id)

    async def add_official_whitelist_user(self, username: str, endpoint_id: int):
        if not username:
            raise HTTPException(status_code=400, detail="username required")
        await self.db.fallback.add_whitelist_user(username, endpoint_id)
        return {"ok": True}

    async def remove_official_whitelist_user(self, username: str, endpoint_id: int):
        if not username:
            raise HTTPException(status_code=400, detail="username required")
        await self.db.fallback.remove_whitelist_user(username, endpoint_id)
        return {"ok": True}

    # ========== Admin Texture Management ==========

    async def get_all_textures(self, limit: int = 20, cursor: str | None = None, query: str | None = None, type_filter: str | None = None) -> dict:
        try:
            key = decode_cursor(cursor, ("last_created_at", "last_skin_hash"))
        except ValueError:
            raise HTTPException(status_code=400, detail="Invalid cursor")
        result = await self.db.texture.list_all_textures_cursor(
            limit,
            last_created_at=(key or {}).get("last_created_at"),
            last_skin_hash=(key or {}).get("last_skin_hash"),
            query=query,
            type_filter=type_filter,
        )
        result["next_cursor"] = encode_next(result.pop("next_key"))
        return result

    async def _get_uploader_or_404(self, texture_hash: str) -> str:
        texture = await self.db.texture.get_texture_from_library(texture_hash)
        if not texture:
            raise HTTPException(status_code=404, detail="材质不存在")
        return texture["uploader"]

    async def update_texture_public(self, texture_hash: str, is_public: int) -> dict:
        if is_public not in (0, 1):
            raise HTTPException(status_code=400, detail="is_public must be 0 or 1")
        uploader = await self._get_uploader_or_404(texture_hash)
        await self.db.texture.update_is_public(uploader, texture_hash, "skin", bool(is_public))
        return {"success": True}

    async def update_texture_model(self, texture_hash: str, model: str) -> dict:
        if model not in ("default", "slim"):
            raise HTTPException(status_code=400, detail="model must be 'default' or 'slim'")
        uploader = await self._get_uploader_or_404(texture_hash)
        await self.db.texture.update_model(uploader, texture_hash, "skin", model)
        return {"success": True}

    async def update_texture_note(self, texture_hash: str, note: str) -> dict:
        uploader = await self._get_uploader_or_404(texture_hash)
        await self.db.texture.update_note(uploader, texture_hash, "skin", note)
        return {"success": True}

    async def patch_texture(self, texture_hash: str, body: dict) -> dict:
        """事务内同时更新 skin_library/user_textures/profiles，避免分步写入产生错配。

        默认作用于 skin 类型；body 可显式带 texture_type 覆盖。
        """
        note = None
        model = None
        is_public = None
        if "note" in body:
            note = str(body["note"])
        if "model" in body:
            m = str(body["model"])
            if m not in ("default", "slim"):
                raise HTTPException(status_code=400, detail="model must be 'default' or 'slim'")
            model = m
        if "is_public" in body:
            v = body["is_public"]
            if isinstance(v, bool):
                is_public = v
            elif isinstance(v, (int, float)):
                if int(v) not in (0, 1):
                    raise HTTPException(status_code=400, detail="is_public must be 0 or 1")
                is_public = bool(v)
            else:
                raise HTTPException(status_code=400, detail="invalid is_public")
        if note is None and model is None and is_public is None:
            raise HTTPException(status_code=400, detail="至少需要一个更新字段: model, note, is_public")
        texture_type = str(body.get("texture_type", "skin"))
        ok = await self.db.texture.admin_patch(
            texture_hash, texture_type,
            note=note, model=model, is_public=is_public,
        )
        if not ok:
            raise HTTPException(status_code=404, detail="材质不存在")
        return {"ok": True}

    async def delete_texture(self, texture_hash: str, texture_type: str, user_id: str | None = None, force: bool = False) -> dict:
        if not force and not user_id:
            raise HTTPException(status_code=400, detail="per-user deletion requires user_id")
        
        await self.db.texture.delete_texture(
            texture_hash=texture_hash,
            texture_type=texture_type,
            user_id=user_id,
            force=force,
        )
        return {"success": True}
