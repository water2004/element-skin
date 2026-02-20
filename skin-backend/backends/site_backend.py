from typing import Optional, Dict, List, Any
import re
import time
import secrets
import os
import random
import string
from fastapi import HTTPException

from utils.password_utils import hash_password, verify_password, needs_rehash
from utils.password_utils import validate_strong_password
from utils.jwt_utils import create_jwt_token
from utils.email_utils import EmailSender
from utils.uuid_utils import generate_random_uuid
from utils.typing import User, InviteCode, PlayerProfile
from database_module import Database
from config_loader import Config


class SiteBackend:
    def __init__(
        self, db: Database, config: Config
    ):  # Use forward reference for type hint
        self.db = db
        self.config = config
        self.email_sender = EmailSender(db)

    # ========== Auth & User ==========

    async def send_verification_code(self, email: str, type: str):
        # Check if email verification is enabled
        enabled = await self.db.setting.get("email_verify_enabled", "false")
        if enabled != "true":
            raise HTTPException(status_code=400, detail="Email verification is disabled")

        # Validate email format
        if not re.match(r"[^@]+@[^@]+\.[^@]+", email):
            raise HTTPException(status_code=400, detail="Invalid email format")

        # For reset password, check if user exists
        if type == "reset":
            user = await self.db.user.get_by_email(email)
            if not user:
                return {"ok": True, "ttl": 0} 

        # For register, check if user exists
        if type == "register":
            user = await self.db.user.get_by_email(email)
            if user:
                raise HTTPException(status_code=400, detail="Email already registered")

        # 8 chars uppercase letters + digits
        code = "".join(random.choices(string.ascii_uppercase + string.digits, k=8))
        ttl = int(await self.db.setting.get("email_verify_ttl", "300"))
        
        await self.db.verification.create_code(email, code, type, ttl)
        
        sent = await self.email_sender.send_verification_code(email, code, type)
        if not sent:
            raise HTTPException(status_code=500, detail="Failed to send verification email")

        return {"ok": True, "ttl": ttl}

    async def verify_code(self, email: str, code: str, type: str) -> bool:
        record = await self.db.verification.get_code(email, type)
        if not record:
            return False
        
        db_code, expires_at = record
        if str(db_code).upper() != str(code).upper():
            return False
            
        if int(time.time() * 1000) > expires_at:
            return False
            
        return True

    async def login(self, email, password) -> Dict[str, Any]:
        user_row = await self.db.user.get_by_email(email)
        if not user_row:
            raise HTTPException(status_code=401, detail="Invalid credentials")

        user_id, email, password_hash, is_admin = (
            user_row.id,
            user_row.email,
            user_row.password,
            user_row.is_admin,
        )

        if not verify_password(password, password_hash):
            raise HTTPException(status_code=401, detail="Invalid credentials")

        if needs_rehash(password_hash):
            new_hash = hash_password(password)
            await self.db.user.update_password(user_id, new_hash)

        expire_days_str = await self.db.setting.get("jwt_expire_days", "7")
        expire_days = int(expire_days_str)
        token = create_jwt_token(user_id, bool(is_admin), expire_days)

        return {"token": token, "user_id": user_id}

    async def register(self, email, password, invite_code=None, verification_code=None) -> str:
        enable_strong_password_check = await self.db.setting.get("enable_strong_password_check", "false") == "true"
        if enable_strong_password_check:
            errors = validate_strong_password(password)
            if errors:
                raise HTTPException(
                    status_code=400, detail="；".join(errors)
                )
        
        # if len(password) < 6:
        #     raise HTTPException(
        #         status_code=400, detail="password must be at least 6 characters"
        #     )

        allow_register = await self.db.setting.get("allow_register", "true")
        if allow_register != "true":
            raise HTTPException(status_code=403, detail="registration is disabled")

        # Email Verification Check
        email_verify_enabled = await self.db.setting.get("email_verify_enabled", "false") == "true"
        if email_verify_enabled:
            if not verification_code:
                raise HTTPException(status_code=400, detail="Verification code required")
            
            is_valid = await self.verify_code(email, verification_code, "register")
            if not is_valid:
                raise HTTPException(status_code=400, detail="Invalid or expired verification code")
            
            # Delete code after usage
            await self.db.verification.delete_code(email, "register")

        require_invite = await self.db.setting.get("require_invite", "false")
        if require_invite == "true":
            if not invite_code:
                raise HTTPException(status_code=400, detail="invite code required")

            invite_row = await self.db.user.get_invite(invite_code)
            if not invite_row:
                raise HTTPException(status_code=400, detail="invalid invite code")

            if (
                invite_row.total_uses is not None
                and invite_row.used_count >= invite_row.total_uses
            ):
                raise HTTPException(
                    status_code=400, detail="invite code has no remaining uses"
                )

        user_count = await self.db.user.count()
        is_first_user = user_count == 0
        password_hash = hash_password(password)
        user_id = generate_random_uuid()
        try:
            await self.db.user.create(
                User(user_id, email, password_hash, 1 if is_first_user else 0)
            )
        except Exception:
            raise HTTPException(status_code=400, detail="Email already registered")

        base_name = email.split("@")[0]
        base_name = re.sub(r"[^a-zA-Z0-9_]", "_", base_name)[:12]
        profile_name = base_name
        suffix = 1
        while True:
            existing = await self.db.user.get_profile_by_name(profile_name)
            if not existing:
                break
            profile_name = f"{base_name}_{suffix}"
            suffix += 1
            if suffix > 100:
                raise HTTPException(status_code=500, detail="无法生成唯一角色名")

        profile_id = generate_random_uuid()
        await self.db.user.create_profile(
            PlayerProfile(profile_id, user_id, profile_name, "default")
        )

        if require_invite == "true" and invite_code:
            await self.db.user.use_invite(invite_code, email)

        return user_id

    async def get_user_info(self, user_id: str) -> Dict[str, Any]:
        user_row = await self.db.user.get_by_id(user_id)
        if not user_row:
            raise HTTPException(status_code=404, detail="user not found")

        profiles = await self.db.user.get_profiles_by_user(user_id)
        profiles_list = [
            {
                "id": p.id,
                "name": p.name,
                "model": p.texture_model,
                "skin_hash": p.skin_hash,
                "cape_hash": p.cape_hash,
            }
            for p in profiles
        ]

        return {
            "id": user_row.id,
            "email": user_row.email,
            "lang": user_row.preferredLanguage,
            "display_name": user_row.display_name,
            "is_admin": bool(user_row.is_admin),
            "banned_until": user_row.banned_until,
            "profiles": profiles_list,
        }

    async def refresh_token(self, user_id: str) -> Dict[str, Any]:
        user_row = await self.db.user.get_by_id(user_id)
        if not user_row:
            raise HTTPException(status_code=404, detail="user not found")

        is_admin = bool(user_row.is_admin)
        expire_days_str = await self.db.setting.get("jwt_expire_days", "7")
        expire_days = int(expire_days_str)
        token = create_jwt_token(user_id, is_admin, expire_days)

        return {"token": token, "is_admin": is_admin}

    async def update_user_info(self, user_id: str, data: Dict[str, Any]):
        if "email" in data and data["email"]:
            await self.db.user.update_email(user_id, data["email"])
        if "display_name" in data and data["display_name"] is not None:
            await self.db.user.update_display_name(user_id, data["display_name"])

        if "preferred_language" in data and data["preferred_language"]:
            async with self.db.get_conn() as conn:
                await conn.execute(
                    "UPDATE users SET preferred_language=? WHERE id=?",
                    (data["preferred_language"], user_id),
                )
                await conn.commit()

        return True

    async def delete_user(self, user_id: str, is_admin_action=False):
        user_row = await self.db.user.get_by_id(user_id)
        if not user_row:
            raise HTTPException(status_code=404, detail="user not found")

        if user_row.is_admin and not is_admin_action:
            raise HTTPException(status_code=403, detail="管理员不能删除自己的账号")

        if user_row.is_admin and is_admin_action:
            raise HTTPException(status_code=403, detail="cannot delete admin user")

        await self.db.user.delete(user_id)
        return True

    async def reset_password(self, email: str, new_password: str, verification_code: str):
        enable_strong_password_check = await self.db.setting.get("enable_strong_password_check", "false") == "true"
        if enable_strong_password_check:
            errors = validate_strong_password(new_password)
            if errors:
                raise HTTPException(
                    status_code=400, detail="；".join(errors)
                )
             
        email_verify_enabled = await self.db.setting.get("email_verify_enabled", "false") == "true"
        if not email_verify_enabled:
            raise HTTPException(status_code=403, detail="Password reset via email is disabled")

        is_valid = await self.verify_code(email, verification_code, "reset")
        if not is_valid:
            raise HTTPException(status_code=400, detail="Invalid or expired verification code")

        user = await self.db.user.get_by_email(email)
        if not user:
            raise HTTPException(status_code=404, detail="User not found")

        new_hash = hash_password(new_password)
        await self.db.user.update_password(user.id, new_hash)
        
        await self.db.verification.delete_code(email, "reset")
        return True

    async def change_password(self, user_id: str, old_password, new_password):
        enable_strong_password_check = await self.db.setting.get("enable_strong_password_check", "false") == "true"
        if enable_strong_password_check:
            errors = validate_strong_password(new_password)
            if errors:
                raise HTTPException(
                    status_code=400, detail="；".join(errors)
                )

        user_row = await self.db.user.get_by_id(user_id)
        if not user_row:
            raise HTTPException(status_code=404, detail="用户不存在")

        if not verify_password(old_password, user_row.password):
            raise HTTPException(status_code=403, detail="旧密码错误")

        new_hash = hash_password(new_password)
        await self.db.user.update_password(user_id, new_hash)
        return True

    # ========== Profile ==========

    async def create_profile(self, user_id, name, model="default"):
        if not name:
            raise HTTPException(status_code=400, detail="name required")

        if not re.match(r"^[a-zA-Z0-9_]{1,16}$", name):
            raise HTTPException(
                status_code=400,
                detail="角色名只能包含字母、数字、下划线，长度1-16字符",
            )

        existing = await self.db.user.get_profile_by_name(name)
        if existing:
            raise HTTPException(status_code=400, detail="角色名已被占用，请换一个名称")

        profile_id = generate_random_uuid()
        await self.db.user.create_profile(
            PlayerProfile(profile_id, user_id, name, model)
        )
        return {"id": profile_id, "name": name, "model": model}

    async def delete_profile(self, user_id, pid):
        profile_row = await self.db.user.get_profile_by_id(pid)
        if not profile_row:
            raise HTTPException(status_code=404, detail="profile not found")
        if profile_row.user_id != user_id:
            raise HTTPException(status_code=403, detail="not allowed")

        await self.db.user.delete_profile(pid)

    async def clear_profile_texture(self, user_id, pid, texture_type):
        is_owner = await self.db.user.verify_profile_ownership(user_id, pid)
        if not is_owner:
            raise ValueError("Not allowed")

        if texture_type.lower() == "skin":
            await self.db.user.update_profile_skin(pid, None)
        elif texture_type.lower() == "cape":
            await self.db.user.update_profile_cape(pid, None)
        else:
            raise ValueError("Invalid texture_type")

    # ========== Admin ==========

    async def get_admin_settings(self):
        settings = await self.db.setting.get_all()
        return {
            "site_name": settings.get("site_name", "皮肤站"),
            "site_url": settings.get("site_url", ""),
            "require_invite": settings.get("require_invite", "false") == "true",
            "allow_register": settings.get("allow_register", "true") == "true",
            "max_texture_size": int(settings.get("max_texture_size", "1024")),
            "rate_limit_enabled": settings.get("rate_limit_enabled", "true") == "true",
            "rate_limit_auth_attempts": int(
                settings.get("rate_limit_auth_attempts", "5")
            ),
            "rate_limit_auth_window": int(settings.get("rate_limit_auth_window", "15")),
            "jwt_expire_days": int(settings.get("jwt_expire_days", "7")),
            "microsoft_client_id": settings.get("microsoft_client_id", ""),
            "microsoft_client_secret": settings.get("microsoft_client_secret", ""),
            "microsoft_redirect_uri": settings.get(
                "microsoft_redirect_uri", "http://localhost:8000/microsoft/callback"
            ),
            # Mojang API Settings (URLs from static config, switches from DB)
            "mojang_session_url": self.config.get("mojang.session_url"),
            "mojang_account_url": self.config.get("mojang.account_url"),
            "mojang_services_url": self.config.get("mojang.services_url"),
            "mojang_skin_domains": ",".join(self.config.get("mojang.skin_domains", [])),
            "mojang_cache_ttl": self.config.get("mojang.cache_ttl"),
            "fallback_mojang_profile": settings.get("fallback_mojang_profile", "false")
            == "true",
            "fallback_mojang_hasjoined": settings.get(
                "fallback_mojang_hasjoined", "false"
            )
            == "true",
            "enable_official_whitelist": settings.get(
                "enable_official_whitelist", "false"
            )
            == "true",
            # SMTP & Email Verification
            "email_verify_enabled": settings.get("email_verify_enabled", "false") == "true",
            "email_verify_ttl": int(settings.get("email_verify_ttl", "300")),
            "enable_strong_password_check": settings.get("enable_strong_password_check", "false") == "true",
            "smtp_host": settings.get("smtp_host", ""),
            "smtp_port": settings.get("smtp_port", "465"),
            "smtp_user": settings.get("smtp_user", ""),
            "smtp_ssl": settings.get("smtp_ssl", "true") == "true",
            "smtp_sender": settings.get("smtp_sender", ""),
            # "password_strength_enabled": settings.get(
            #     "password_strength_enabled", "true"
            # )
            # == "true",
        }

    async def save_admin_settings(self, body: dict):
        for key in [
            "site_name",
            "site_url",
            "require_invite",
            "allow_register",
            "max_texture_size",
            "rate_limit_enabled",
            "rate_limit_auth_attempts",
            "rate_limit_auth_window",
            "jwt_expire_days",
            "microsoft_client_id",
            "microsoft_client_secret",
            "microsoft_redirect_uri",
            "fallback_mojang_profile",
            "fallback_mojang_hasjoined",
            "enable_official_whitelist",
            "email_verify_enabled",
            "email_verify_ttl",
            "enable_strong_password_check",
            "smtp_host",
            "smtp_port",
            "smtp_user",
            "smtp_password",
            "smtp_ssl",
            "smtp_sender",
            # "password_strength_enabled",
        ]:
            if key in body:
                val = body[key]
                if isinstance(val, bool):
                    value = "true" if val else "false"
                else:
                    value = str(val)
                # Don't save empty password if not provided
                if key == "smtp_password" and not value:
                    continue
                await self.db.setting.set(key, value)

    async def get_official_whitelist(self):
        return await self.db.user.list_official_whitelist_users()

    async def add_official_whitelist_user(self, username: str):
        if not username:
            raise HTTPException(status_code=400, detail="Username required")
        await self.db.user.add_official_whitelist_user(username)
        return {"ok": True}

    async def remove_official_whitelist_user(self, username: str):
        await self.db.user.remove_official_whitelist_user(username)
        return {"ok": True}

    # ========== Carousel ==========

    async def list_carousel_images(self) -> List[str]:
        directory = self.config.get("carousel.directory", "carousel")
        if not os.path.exists(directory):
            return []

        # List files and filter for images
        files = os.listdir(directory)
        images = [
            f for f in files if f.lower().endswith((".png", ".jpg", ".jpeg", ".webp"))
        ]
        # Sort by name (or could be by mtime)
        images.sort()
        return images

    async def upload_carousel_image(self, filename: str, content: bytes):
        directory = self.config.get("carousel.directory", "carousel")
        os.makedirs(directory, exist_ok=True)

        file_path = os.path.join(directory, filename)
        with open(file_path, "wb") as f:
            f.write(content)
        return {"filename": filename}

    async def delete_carousel_image(self, filename: str):
        directory = self.config.get("carousel.directory", "carousel")
        file_path = os.path.join(directory, filename)

        # Security check: ensure the filename doesn't contain path traversal
        if os.path.dirname(os.path.abspath(file_path)) != os.path.abspath(directory):
            raise HTTPException(status_code=400, detail="Invalid filename")

        if os.path.exists(file_path):
            os.remove(file_path)
            return {"ok": True}
        raise HTTPException(status_code=404, detail="File not found")

    async def get_admin_users(self):
        users = await self.db.user.list_users(limit=1000, offset=0)
        result = []
        for row in users:
            user_id = row.id
            profile_count = await self.db.user.count_profiles_by_user(user_id)
            result.append(
                {
                    "id": row.id,
                    "email": row.email,
                    "display_name": row.display_name or "",
                    "is_admin": bool(row.is_admin),
                    "banned_until": row.banned_until,
                    "profile_count": profile_count,
                }
            )
        return result

    async def toggle_user_admin(self, user_id: str, actor_id: str):
        if actor_id == user_id:
            raise HTTPException(
                status_code=403, detail="cannot change own admin status"
            )

        new_status = await self.db.user.toggle_admin(user_id)
        if new_status == -1:
            raise HTTPException(status_code=404, detail="user not found")

    async def ban_user(self, user_id, banned_until, actor_id):
        user_row = await self.db.user.get_by_id(user_id)
        if not user_row:
            raise HTTPException(status_code=404, detail="user not found")
        if user_row.is_admin:
            raise HTTPException(status_code=403, detail="cannot ban admin user")

        await self.db.user.ban(user_id, banned_until)
        return banned_until

    async def create_invite(self, code, total_uses, note: str = ""):
        if code:
            if len(code) < 6 or len(code) > 32:
                raise HTTPException(status_code=400, detail="Invalid code length")
            if not re.match(r"^[a-zA-Z0-9_-]+$", code):
                raise HTTPException(status_code=400, detail="Invalid characters")
        else:
            code = secrets.token_urlsafe(16)

        existing = await self.db.user.get_invite(code)
        if existing:
            raise HTTPException(status_code=400, detail="invite code already exists")

        created_at = int(time.time() * 1000)
        await self.db.user.create_invite(
            InviteCode(code, created_at, total_uses=total_uses, note=note)
        )
        return code

    async def apply_texture_to_profile(
        self, user_id, profile_id, texture_hash, texture_type
    ):
        if not await self.db.texture.verify_ownership(
            user_id, texture_hash, texture_type
        ):
            raise ValueError("Texture not found in your library")

        if not await self.db.user.verify_profile_ownership(user_id, profile_id):
            raise ValueError("Profile not yours")

        if texture_type.lower() == "skin":
            await self.db.user.update_profile_skin(profile_id, texture_hash)
        elif texture_type.lower() == "cape":
            await self.db.user.update_profile_cape(profile_id, texture_hash)
        else:
            raise ValueError("Invalid texture_type")
