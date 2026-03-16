from typing import Optional, Dict, List, Any
import re
import time
import os
import random
import string
from fastapi import HTTPException

from utils.password_utils import hash_password, verify_password, needs_rehash
from utils.password_utils import validate_strong_password
from utils.jwt_utils import create_jwt_token
from utils.email_utils import EmailSender
from utils.uuid_utils import generate_random_uuid
from backends.yggdrasil_client import YggdrasilClient, download_texture
from utils.typing import User, PlayerProfile
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

    async def get_ygg_profiles(self, api_url: str, username: str, password: str):
        client = YggdrasilClient(api_url)
        try:
            result = await client.authenticate(username, password)
            # Standard Yggdrasil authenticate response
            profiles = result.get("availableProfiles", [])
            return {"profiles": profiles}
        except Exception as e:
            raise HTTPException(status_code=400, detail=str(e))

    async def import_ygg_profile(self, user_id: str, api_url: str, profile_id: str, profile_name: str):
        client = YggdrasilClient(api_url)
        try:
            # 1. Fetch detailed profile with textures
            profile_data = await client.get_profile_with_textures(profile_id)
            
            # 2. Check if profile name is taken locally
            # We try to use the original name, but if taken, append a suffix
            target_name = profile_name
            suffix = 1
            while True:
                existing = await self.db.user.get_profile_by_name(target_name)
                if not existing:
                    break
                target_name = f"{profile_name}_{suffix}"
                suffix += 1
                if suffix > 100:
                     raise HTTPException(status_code=400, detail="无法生成唯一的角色名称")
            
            # 3. Download and upload textures
            skin_hash = None
            skin_model = "default"
            if profile_data.get("skins"):
                skin_url = profile_data["skins"][0]["url"]
                skin_variant = profile_data["skins"][0].get("variant", "classic")
                skin_model = "slim" if skin_variant == "slim" else "default"
                try:
                    skin_bytes = await download_texture(skin_url)
                    skin_hash, _ = await self.db.texture.upload(
                        user_id, skin_bytes, "skin", f"Imported from {api_url}", is_public=False, model=skin_model
                    )
                except Exception as e:
                    print(f"Failed to download/upload skin: {e}")

            cape_hash = None
            if profile_data.get("capes"):
                cape_url = profile_data["capes"][0]["url"]
                try:
                    cape_bytes = await download_texture(cape_url)
                    cape_hash, _ = await self.db.texture.upload(
                        user_id, cape_bytes, "cape", f"Imported from {api_url}", is_public=False
                    )
                except Exception as e:
                    print(f"Failed to download/upload cape: {e}")

            # 4. Create local profile
            local_profile_id = generate_random_uuid()
            await self.db.user.create_profile(
                PlayerProfile(local_profile_id, user_id, target_name, skin_model)
            )
            
            # 5. Apply textures
            if skin_hash:
                await self.db.user.update_profile_skin(local_profile_id, skin_hash)
            if cape_hash:
                await self.db.user.update_profile_cape(local_profile_id, cape_hash)

            return {"id": local_profile_id, "name": target_name}
        except Exception as e:
            if isinstance(e, HTTPException):
                raise e
            raise HTTPException(status_code=400, detail=str(e))

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

    async def register(self, email, password, username, invite_code=None, verification_code=None) -> str:
        if not username or not username.strip():
            raise HTTPException(status_code=400, detail="Username is required")
        
        username = username.strip()

        # Check if username (display_name) is taken
        if await self.db.user.is_display_name_taken(username):
            raise HTTPException(status_code=400, detail="Username already exists")

        enable_strong_password_check = await self.db.setting.get("enable_strong_password_check", "false") == "true"
        if enable_strong_password_check:
            errors = validate_strong_password(password)
            if errors:
                raise HTTPException(
                    status_code=400, detail="；".join(errors)
                )

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
            new_user = User(user_id, email, password_hash, is_first_user)
            new_user.display_name = username
            await self.db.user.create(new_user)
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
        
        if "display_name" in data and data["display_name"]:
            new_name = data["display_name"].strip()
            if not new_name:
                raise HTTPException(status_code=400, detail="Username cannot be empty")
            
            # Check for uniqueness if changed
            user_row = await self.db.user.get_by_id(user_id)
            if user_row and user_row.display_name != new_name:
                if await self.db.user.is_display_name_taken(
                    new_name, exclude_user_id=user_id
                ):
                    raise HTTPException(status_code=400, detail="Username already exists")
            
            await self.db.user.update_display_name(user_id, new_name)

        if "preferred_language" in data and data["preferred_language"]:
            await self.db.user.update_preferred_language(
                user_id, data["preferred_language"]
            )

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

    async def update_profile(self, user_id, pid, name):
        profile_row = await self.db.user.get_profile_by_id(pid)
        if not profile_row:
            raise HTTPException(status_code=404, detail="profile not found")
        if profile_row.user_id != user_id:
            raise HTTPException(status_code=403, detail="not allowed")

        if not name:
            raise HTTPException(status_code=400, detail="name required")
        
        if not re.match(r"^[a-zA-Z0-9_]{1,16}$", name):
            raise HTTPException(
                status_code=400,
                detail="角色名只能包含字母、数字、下划线，长度1-16字符",
            )

        if profile_row.name != name:
            existing = await self.db.user.get_profile_by_name(name)
            if existing:
                raise HTTPException(status_code=400, detail="角色名已被占用")

        await self.db.user.update_profile_name(pid, name)
        return True

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

    async def apply_texture_to_profile(
        self, user_id, profile_id, texture_hash, texture_type
    ):
        if not await self.db.texture.verify_ownership(
            user_id, texture_hash, texture_type
        ):
            raise ValueError("Texture not found in your library")

        if not await self.db.user.verify_profile_ownership(user_id, profile_id):
            raise ValueError("Profile not yours")

        # Get texture info to get the model
        tex_info = await self.db.texture.get_texture_info(user_id, texture_hash, texture_type)
        if not tex_info:
            raise ValueError("Texture info not found")

        if texture_type.lower() == "skin":
            await self.db.user.update_profile_skin(profile_id, texture_hash)
            # Also update profile's model to match skin's model
            await self.db.user.update_profile_texture_model(profile_id, tex_info.get("model", "default"))
        elif texture_type.lower() == "cape":
            await self.db.user.update_profile_cape(profile_id, texture_hash)
        else:
            raise ValueError("Invalid texture_type")

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
    
    async def get_fallback_services(self) -> list[dict]:
        return await self.db.fallback.list_endpoints()
