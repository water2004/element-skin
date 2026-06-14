from typing import Dict, List, Any
import asyncio
import re
import time
import os
import secrets
import string
import asyncpg
from fastapi import HTTPException

from utils.password_utils import hash_password, hash_password_async, verify_password_async, needs_rehash
from utils.password_utils import validate_strong_password
from utils.jwt_utils import create_access_token, generate_refresh_token, hash_refresh_token
from utils.email_utils import EmailSender
from utils.uuid_utils import generate_random_uuid, get_offline_uuid
from utils.profile_naming import is_valid_profile_name, generate_unique_profile_name
from utils.pagination import decode_cursor, encode_next
from utils.typing import User, PlayerProfile, normalize_texture_model, serialize_profile_summary
from database_module import Database
from database_module.modules.user import InviteExhaustedError, DisplayNameConflictError
from config_loader import Config
from services import TextureStorage, assert_texture_size


# 预先计算的 bcrypt 哈希，用于登录时对"用户不存在"分支做等时校验，
# 抹平用户枚举的计时侧信道。值本身无意义（不会有人能匹配）。
_DUMMY_PASSWORD_HASH = hash_password("dummy-password-for-timing-equalization")

# 邮箱格式校验：基础但足够实用的正则（非严格 RFC 5322）
_EMAIL_RE = re.compile(r"^[^@\s]+@[^@\s]+\.[^@\s]+$")


def is_valid_email(email: str) -> bool:
    """校验邮箱格式：fullmatch + 显式拒绝 CRLF，防头注入与未捕获 500。

    re.match 的 `$` 会匹配末尾换行前位置，单独 `.match` 仍可放行 `a@b.com\\n`，
    故用 fullmatch；再显式排除 \\r/\\n 双保险，杜绝 `a@x.com\\r\\nBcc: ...` 这类
    头注入载荷进入邮件发送链路。
    """
    return bool(_EMAIL_RE.fullmatch(email)) and "\r" not in email and "\n" not in email


class SiteBackend:
    def __init__(
        self, db: Database, config: Config, texture_storage: TextureStorage
    ):  # Use forward reference for type hint
        self.db = db
        self.config = config
        self.texture_storage = texture_storage
        self.email_sender = EmailSender(db)

    async def upload_texture_to_library(
        self,
        user_id: str,
        file_bytes: bytes,
        texture_type: str,
        note: str = "",
        is_public: bool = False,
        model: str = "default",
    ) -> tuple[str, str]:
        """处理材质（落盘）并记录到用户库，返回 (hash, type)。校验失败抛 ValueError。"""
        await assert_texture_size(self.db, file_bytes)
        texture_hash, created = await self.texture_storage.process_and_save_async_tracked(
            file_bytes, texture_type
        )
        try:
            await self.db.texture.add_to_library(
                user_id, texture_hash, texture_type, note, is_public, model
            )
        except Exception:
            if created:
                try:
                    if not await self.db.texture.exists(texture_hash, texture_type):
                        await asyncio.to_thread(self.texture_storage.delete_file, texture_hash)
                except Exception:
                    pass
            raise
        return texture_hash, texture_type

    async def upload_and_apply_texture(
        self,
        user_id: str,
        profile_id: str,
        file_bytes: bytes,
        texture_type: str,
        model: str = "",
        is_public: bool = False,
    ):
        """上传材质到用户库 → 应用到角色 →（皮肤时）更新模型。校验失败抛 ValueError。"""
        texture_hash, _ = await self.upload_texture_to_library(
            user_id,
            file_bytes,
            texture_type,
            f"Direct upload to profile {profile_id}",
            is_public=is_public,
        )
        await self.apply_texture_to_profile(
            user_id, profile_id, texture_hash, texture_type
        )
        if texture_type.lower() == "skin":
            m_val = normalize_texture_model(model)
            await self.db.user.update_profile_texture_model(profile_id, m_val)
        return {"ok": True}

    async def list_my_textures(
        self,
        user_id: str,
        cursor: str | None,
        limit: int,
        texture_type: str | None,
    ) -> dict:
        try:
            key = decode_cursor(cursor, ("last_created_at", "last_hash"))
        except ValueError:
            raise HTTPException(status_code=400, detail="Invalid cursor")
        result = await self.db.texture.get_for_user_cursor(
            user_id,
            texture_type=texture_type,
            limit=limit,
            last_created_at=(key or {}).get("last_created_at"),
            last_hash=(key or {}).get("last_hash"),
        )
        result["next_cursor"] = encode_next(result.pop("next_key"))
        return result

    async def list_my_profiles(self, user_id: str, cursor: str | None, limit: int) -> dict:
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

    async def get_my_texture_detail(self, user_id: str, texture_hash: str, texture_type: str) -> dict:
        info = await self.db.texture.get_texture_info(user_id, texture_hash, texture_type)
        if not info:
            raise HTTPException(status_code=404, detail="Texture not found")
        return info

    async def update_my_texture(
        self, user_id: str, texture_hash: str, texture_type: str, data: Dict[str, Any]
    ) -> dict:
        note = data.get("note") if "note" in data else None
        model = None
        if "model" in data:
            m = str(data["model"])
            if m not in ("default", "slim"):
                raise HTTPException(status_code=400, detail="invalid model")
            model = m
        is_public = None
        if "is_public" in data:
            v = data["is_public"]
            if isinstance(v, bool):
                is_public = v
            elif isinstance(v, (int, float)):
                is_public = bool(v)
            else:
                raise HTTPException(status_code=400, detail="invalid is_public")

        ok = await self.db.texture.patch_for_user(
            user_id, texture_hash, texture_type,
            note=note, model=model, is_public=is_public,
        )
        if not ok:
            raise HTTPException(status_code=404, detail="Texture not found")

        info = await self.db.texture.get_texture_info(user_id, texture_hash, texture_type)
        return {"ok": True, **info}

    async def remove_my_texture(self, user_id: str, texture_hash: str, texture_type: str):
        await self.db.texture.delete_from_library(user_id, texture_hash, texture_type)

    async def add_texture_to_wardrobe(self, user_id: str, texture_hash: str):
        success = await self.db.texture.add_to_user_wardrobe(user_id, texture_hash)
        if not success:
            raise HTTPException(status_code=404, detail="Texture not found in library")

    async def get_public_skin_library(
        self, cursor: str | None, limit: int, texture_type: str | None,
        query: str | None = None,
    ) -> dict:
        enabled = await self.db.setting.get("enable_skin_library", "true")
        if enabled != "true":
            raise HTTPException(status_code=403, detail="Skin library is disabled by administrator")

        try:
            key = decode_cursor(cursor, ("last_created_at", "last_skin_hash"))
        except ValueError:
            raise HTTPException(status_code=400, detail="Invalid cursor")
        result = await self.db.texture.get_from_library_cursor(
            limit=limit,
            texture_type=texture_type,
            only_public=True,
            last_created_at=(key or {}).get("last_created_at"),
            last_skin_hash=(key or {}).get("last_skin_hash"),
            query=query,
        )
        # uploader_name 已由 LEFT JOIN 直接返回，无需二次查库
        return {
            "items": [
                {**item, "uploader_name": item.get("uploader_display_name", "")}
                for item in result["items"]
            ],
            "has_next": result["has_next"],
            "next_cursor": encode_next(result["next_key"]),
            "page_size": result["page_size"],
        }

    async def _generate_profile_uuid(self, profile_name: str) -> str:
        mode = (await self.db.setting.get("profile_uuid_mode", "random") or "random").strip().lower()
        if mode == "offline":
            profile_id = get_offline_uuid(profile_name)
        else:
            profile_id = generate_random_uuid()

        existing_profile = await self.db.user.get_profile_by_id(profile_id)
        if existing_profile:
            raise HTTPException(status_code=400, detail="角色 UUID 冲突，无法新建角色")
        return profile_id

    # ========== Auth & User ==========

    async def send_verification_code(self, email: str, type: str):
        # Check if email verification is enabled
        enabled = await self.db.setting.get("email_verify_enabled", "false")
        if enabled != "true":
            raise HTTPException(status_code=400, detail="Email verification is disabled")

        # Validate email format（fullmatch + 拒绝 CRLF，防头注入）
        if not is_valid_email(email):
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

        # 8 chars uppercase letters + digits（密码学安全随机源，把守密码重置）
        code = "".join(secrets.choice(string.ascii_uppercase + string.digits) for _ in range(8))
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
            # 对不存在的用户也执行一次等价的 bcrypt 校验，使响应耗时与
            # "用户存在但密码错误"相近，避免通过计时差异枚举注册邮箱。
            await verify_password_async(password, _DUMMY_PASSWORD_HASH)
            raise HTTPException(status_code=401, detail="Invalid credentials")

        user_id, email, password_hash, is_admin = (
            user_row.id,
            user_row.email,
            user_row.password,
            user_row.is_admin,
        )

        if not await verify_password_async(password, password_hash):
            raise HTTPException(status_code=401, detail="Invalid credentials")

        if needs_rehash(password_hash):
            new_hash = await hash_password_async(password)
            await self.db.user.update_password(user_id, new_hash)

        return await self._issue_session(user_id, bool(is_admin), extra={"user_id": user_id})

    async def _issue_session(self, user_id: str, is_admin: bool, extra: Dict[str, Any] | None = None) -> Dict[str, Any]:
        """签发一对 access + refresh：access 为无状态 JWT，refresh 入库（存哈希）。"""
        expire_days = int(await self.db.setting.get("jwt_expire_days", "7"))
        now_ms = int(time.time() * 1000)
        expires_at = now_ms + expire_days * 24 * 3600 * 1000

        access_token = create_access_token(user_id, is_admin)
        raw_refresh, refresh_hash = generate_refresh_token()
        await self.db.user.add_refresh_token(refresh_hash, user_id, expires_at, now_ms)

        result = {
            "access_token": access_token,
            "refresh_token": raw_refresh,
            "is_admin": is_admin,
        }
        if extra:
            result.update(extra)
        return result

    async def register(self, email, password, username, invite_code=None, verification_code=None) -> str:
        if not username or not username.strip():
            raise HTTPException(status_code=400, detail="Username is required")

        username = username.strip()

        # 校验邮箱格式（fullmatch + 拒绝 CRLF），防头注入与未捕获 500
        if not email or not is_valid_email(email.strip()):
            raise HTTPException(status_code=400, detail="Invalid email format")
        email = email.strip()

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
        verified_email = False
        if email_verify_enabled:
            if not verification_code:
                raise HTTPException(status_code=400, detail="Verification code required")

            is_valid = await self.verify_code(email, verification_code, "register")
            if not is_valid:
                raise HTTPException(status_code=400, detail="Invalid or expired verification code")

            verified_email = True

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

        base_name = email.split("@")[0]
        base_name = re.sub(r"[^a-zA-Z0-9_]", "_", base_name)[:12]

        async def _name_exists(n: str) -> bool:
            return await self.db.user.get_profile_by_name(n) is not None

        try:
            profile_name = await generate_unique_profile_name(base_name, _name_exists)
        except ValueError:
            raise HTTPException(status_code=500, detail="无法生成唯一角色名")

        profile_id = await self._generate_profile_uuid(profile_name)

        user_count = await self.db.user.count()
        is_first_user = user_count == 0
        password_hash = await hash_password_async(password)
        user_id = generate_random_uuid()

        new_user = User(user_id, email, password_hash, is_first_user)
        new_user.display_name = username
        profile = PlayerProfile(profile_id, user_id, profile_name, "default")
        try:
            await self.db.user.create_user_with_profile(
                new_user,
                profile,
                invite_code=(invite_code if require_invite == "true" else None),
                used_by=email,
            )
        except asyncpg.UniqueViolationError:
            raise HTTPException(status_code=400, detail="Email already registered")
        except DisplayNameConflictError:
            raise HTTPException(status_code=400, detail="Username already exists")
        except InviteExhaustedError:
            raise HTTPException(status_code=400, detail="invite code has no remaining uses")

        if verified_email:
            try:
                await self.db.verification.delete_code(email, "register")
            except Exception:
                pass

        return user_id

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

    async def rotate_refresh_token(self, raw_refresh: str) -> Dict[str, Any]:
        """原子轮换 refresh：删除旧 token 与写入新 token 在同一事务内完成。

        若新 token 写入失败（数据库异常）则旧 token 不会被删除，调用方仍可继续使用
        原 refresh，避免单步失败导致用户被强制登出。
        校验失败（缺失/未知/过期/用户已删/被并发消费）一律抛 401。
        """
        old_hash = hash_refresh_token(raw_refresh)

        row = await self.db.user.get_refresh_token(old_hash)
        if not row:
            raise HTTPException(status_code=401, detail="invalid refresh token")

        user_id = row["user_id"]
        if int(time.time() * 1000) >= row["expires_at"]:
            await self.db.user.delete_refresh_token(old_hash)
            raise HTTPException(status_code=401, detail="refresh token expired")

        user_row = await self.db.user.get_by_id(user_id)
        if not user_row:
            raise HTTPException(status_code=401, detail="invalid refresh token")

        expire_days = int(await self.db.setting.get("jwt_expire_days", "7"))
        now_ms = int(time.time() * 1000)
        expires_at = now_ms + expire_days * 24 * 3600 * 1000
        access_token = create_access_token(user_id, bool(user_row.is_admin))
        new_raw, new_hash = generate_refresh_token()

        rotated = await self.db.user.rotate_refresh_token(
            old_hash, new_hash, user_id, expires_at, now_ms
        )
        if not rotated:
            raise HTTPException(status_code=401, detail="invalid refresh token")

        return {
            "access_token": access_token,
            "refresh_token": new_raw,
            "is_admin": bool(user_row.is_admin),
        }

    async def revoke_refresh_token(self, raw_refresh: str):
        """撤销单个 refresh token（登出用）。找不到也无所谓。"""
        await self.db.user.delete_refresh_token(hash_refresh_token(raw_refresh))

    async def update_user_info(self, user_id: str, data: Dict[str, Any]):
        if "email" in data and data["email"]:
            new_email = data["email"].strip()
            # 基本邮箱格式校验（fullmatch + 拒绝 CRLF，防头注入）
            if not is_valid_email(new_email):
                raise HTTPException(status_code=400, detail="Invalid email format")
            # 唯一性预检：直接写入会撞 DB 的 UNIQUE 约束抛出未处理异常(500)，
            # 这里先查重并返回明确的 400。
            existing = await self.db.user.get_by_email(new_email)
            if existing and existing.id != user_id:
                raise HTTPException(status_code=400, detail="Email already in use")
            await self.db.user.update_email(user_id, new_email)
        
        if "display_name" in data and data["display_name"]:
            new_name = data["display_name"].strip()
            if not new_name:
                raise HTTPException(status_code=400, detail="Username cannot be empty")

            try:
                ok = await self.db.user.update_display_name_safely(user_id, new_name)
            except DisplayNameConflictError:
                raise HTTPException(status_code=400, detail="Username already exists")
            if not ok:
                raise HTTPException(status_code=404, detail="user not found")

        if "preferred_language" in data and data["preferred_language"]:
            await self.db.user.update_preferred_language(
                user_id, data["preferred_language"]
            )

        if "avatar_hash" in data:
            # allow None/null to clear avatar
            await self.db.user.update_avatar_hash(user_id, data["avatar_hash"])

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

        new_hash = await hash_password_async(new_password)
        # 先吊销外部凭证，任何失败都不应改变密码
        await self.db.user.delete_tokens_by_user(user.id)
        await self.db.user.delete_refresh_tokens_by_user(user.id)
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

        if not await verify_password_async(old_password, user_row.password):
            raise HTTPException(status_code=403, detail="旧密码错误")

        new_hash = await hash_password_async(new_password)
        # 先撤销外部凭证；若任一步失败应保留旧密码
        await self.db.user.delete_tokens_by_user(user_id)
        await self.db.user.delete_refresh_tokens_by_user(user_id)
        await self.db.user.update_password(user_id, new_hash)
        return True

    # ========== Profile ==========

    async def create_profile(self, user_id, name, model="default"):
        if not name:
            raise HTTPException(status_code=400, detail="name required")

        if not is_valid_profile_name(name):
            raise HTTPException(
                status_code=400,
                detail="角色名只能包含字母、数字、下划线，长度1-16字符",
            )

        existing = await self.db.user.get_profile_by_name(name)
        if existing:
            raise HTTPException(status_code=400, detail="角色名已被占用，请换一个名称")

        profile_id = await self._generate_profile_uuid(name)
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

        if not is_valid_profile_name(name):
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

        # 级联删除：同时清掉该 profile 的 Yggdrasil 游戏 token，避免孤儿 token
        await self.db.user.delete_profile_cascade(pid)

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
            await self.db.user.update_profile_skin_and_model(
                profile_id, texture_hash, tex_info.get("model", "default")
            )
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
