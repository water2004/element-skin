from typing import Any, List, Dict
import time
import secrets
import os
import re
from fastapi import HTTPException

from utils.typing import InviteCode
from database_module import Database
from config_loader import Config

class AdminBackend:
    def __init__(self, db: Database, config: Config):
        self.db = db
        self.config = config

    # ========== Settings Management (Granular) ==========

    async def get_site_settings(self):
        s = await self.db.setting.get_all()
        return {
            "site_name": s.get("site_name", "皮肤站"),
            "site_subtitle": s.get("site_subtitle", "简洁、高效、现代的 Minecraft 皮肤管理站"),
            "require_invite": s.get("require_invite", "false") == "true",
            "allow_register": s.get("allow_register", "true") == "true",
            "enable_skin_library": s.get("enable_skin_library", "true") == "true",
            "max_texture_size": int(s.get("max_texture_size", "1024")),
            "footer_text": s.get("footer_text", ""),
            "filing_icp": s.get("filing_icp", ""),
            "filing_icp_link": s.get("filing_icp_link", ""),
            "filing_mps": s.get("filing_mps", ""),
            "filing_mps_link": s.get("filing_mps_link", ""),
            "profile_uuid_mode": s.get("profile_uuid_mode", "random"),
        }

    async def get_security_settings(self):
        s = await self.db.setting.get_all()
        return {
            "rate_limit_enabled": s.get("rate_limit_enabled", "true") == "true",
            "rate_limit_auth_attempts": int(s.get("rate_limit_auth_attempts", "5")),
            "rate_limit_auth_window": int(s.get("rate_limit_auth_window", "15")),
            "enable_strong_password_check": s.get("enable_strong_password_check", "false") == "true",
        }

    async def get_auth_settings(self):
        s = await self.db.setting.get_all()
        return {
            "jwt_expire_days": int(s.get("jwt_expire_days", "7")),
        }

    async def get_microsoft_settings(self):
        s = await self.db.setting.get_all()
        return {
            "microsoft_client_id": s.get("microsoft_client_id", ""),
            "microsoft_client_secret": s.get("microsoft_client_secret", ""),
            "microsoft_redirect_uri": s.get("microsoft_redirect_uri", ""),
        }

    async def get_email_settings(self):
        s = await self.db.setting.get_all()
        return {
            "email_verify_enabled": s.get("email_verify_enabled", "false") == "true",
            "email_verify_ttl": int(s.get("email_verify_ttl", "300")),
            "smtp_host": s.get("smtp_host", ""),
            "smtp_port": int(s.get("smtp_port", "465")),
            "smtp_user": s.get("smtp_user", ""),
            "smtp_ssl": s.get("smtp_ssl", "true") == "true",
            "smtp_sender": s.get("smtp_sender", ""),
        }

    async def get_fallback_settings(self):
        s = await self.db.setting.get_all()
        return {
            "fallback_strategy": s.get("fallback_strategy", "serial"),
            "fallbacks": await self.db.fallback.list_endpoints(),
        }

    async def save_settings_group(self, group: str, body: dict):
        allowed_keys = {
            "site": [
                "site_name",
                "site_subtitle",
                "require_invite",
                "allow_register",
                "enable_skin_library",
                "max_texture_size",
                "footer_text",
                "filing_icp",
                "filing_icp_link",
                "filing_mps",
                "filing_mps_link",
                "profile_uuid_mode",
            ],
            "security": ["rate_limit_enabled", "rate_limit_auth_attempts", "rate_limit_auth_window", "enable_strong_password_check"],
            "auth": ["jwt_expire_days"],
            "microsoft": ["microsoft_client_id", "microsoft_client_secret", "microsoft_redirect_uri"],
            "email": ["email_verify_enabled", "email_verify_ttl", "smtp_host", "smtp_port", "smtp_user", "smtp_password", "smtp_ssl", "smtp_sender"],
            "fallback": ["fallback_strategy"]
        }
        
        if group not in allowed_keys and group != "fallback_endpoints":
            raise HTTPException(status_code=400, detail="Invalid settings group")

        if group == "fallback_endpoints":
            if "fallbacks" in body:
                fallbacks = self._validate_fallback_services(body.get("fallbacks"))
                await self.db.fallback.save_endpoints(fallbacks)
            return

        for key in allowed_keys[group]:
            if key in body:
                val = body[key]
                if key == "profile_uuid_mode":
                    mode = str(val).strip().lower()
                    if mode not in ["random", "offline"]:
                        raise HTTPException(status_code=400, detail="profile_uuid_mode must be random or offline")
                    val = mode
                # Special handling for password
                if key == "smtp_password" and not val:
                    continue
                
                value = "true" if isinstance(val, bool) and val else ("false" if isinstance(val, bool) else str(val))
                await self.db.setting.set(key, value)
        
        # If fallback strategy was saved, update endpoints if they were also passed (though we prefer separate)
        if group == "fallback" and "fallbacks" in body:
            fallbacks = self._validate_fallback_services(body.get("fallbacks"))
            await self.db.fallback.save_endpoints(fallbacks)

    # ========== Legacy compatibility (can be removed later) ==========

    async def get_admin_settings(self):
        site = await self.get_site_settings()
        sec = await self.get_security_settings()
        auth = await self.get_auth_settings()
        ms = await self.get_microsoft_settings()
        fallback = await self.get_fallback_settings()
        email = await self.get_email_settings()
        return {**site, **sec, **auth, **ms, **fallback, **email}

    async def save_admin_settings(self, body: dict):
        # Determine which groups are present and save them
        for group in ["site", "security", "auth", "microsoft", "email", "fallback"]:
            await self.save_settings_group(group, body)
        if "fallbacks" in body:
            await self.save_settings_group("fallback_endpoints", body)

    # ========== Other Methods ==========

    def _validate_fallback_services(self, services: Any) -> list[dict]:
        if not isinstance(services, list):
            raise HTTPException(status_code=400, detail="fallbacks must be a list")

        normalized: list[dict] = []
        for idx, entry in enumerate(services, start=1):
            if not isinstance(entry, dict):
                raise HTTPException(status_code=400, detail="invalid fallback entry")

            endpoint_id = entry.get("id")
            if endpoint_id is not None:
                try:
                    endpoint_id = int(endpoint_id)
                except (TypeError, ValueError):
                    raise HTTPException(status_code=400, detail=f"fallback[{idx}] id invalid")
            
            session_url = str(entry.get("session_url", "")).strip()
            account_url = str(entry.get("account_url", "")).strip()
            services_url = str(entry.get("services_url", "")).strip()
            cache_ttl = entry.get("cache_ttl", 60)
            raw_domains = entry.get("skin_domains", "")
            
            if not session_url or not account_url or not services_url:
                raise HTTPException(status_code=400, detail=f"fallback[{idx}] urls are required")

            if isinstance(raw_domains, list):
                skin_domains = [str(item).strip() for item in raw_domains if str(item).strip()]
            else:
                skin_domains = [item.strip() for item in str(raw_domains).split(",") if item.strip()]
            
            try:
                cache_ttl = int(cache_ttl)
            except (TypeError, ValueError):
                raise HTTPException(status_code=400, detail=f"fallback[{idx}] cache_ttl invalid")
            
            if cache_ttl < 0:
                raise HTTPException(status_code=400, detail=f"fallback[{idx}] cache_ttl must be non-negative")

            normalized.append({
                "id": endpoint_id,
                "session_url": session_url,
                "account_url": account_url,
                "services_url": services_url,
                "cache_ttl": cache_ttl,
                "skin_domains": ",".join(skin_domains),
                "enable_profile": bool(entry.get("enable_profile", True)),
                "enable_hasjoined": bool(entry.get("enable_hasjoined", True)),
                "enable_whitelist": bool(entry.get("enable_whitelist", False)),
                "note": str(entry.get("note", "")).strip(),
            })
        return normalized

    async def get_user_info(self, user_id: str) -> Dict[str, Any]:
        user_row = await self.db.user.get_by_id(user_id)
        if not user_row:
            raise HTTPException(status_code=404, detail="user not found")

        profile_count = await self.db.user.count_profiles_by_user(user_id)
        texture_count = await self.db.texture.count_for_user(user_id)

        return {
            "id": user_row.id,
            "email": user_row.email,
            "lang": user_row.preferredLanguage,
            "display_name": user_row.display_name,
            "is_admin": bool(user_row.is_admin),
            "banned_until": user_row.banned_until,
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

    async def reset_user_password(self, user_id: str, new_password: str):
        from utils.password_utils import hash_password
        user_row = await self.db.user.get_by_id(user_id)
        if not user_row:
            raise HTTPException(status_code=404, detail="user not found")
        
        password_hash = hash_password(new_password)
        await self.db.user.update_password(user_id, password_hash)
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
