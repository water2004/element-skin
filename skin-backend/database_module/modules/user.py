from ..core import BaseDB
from utils.typing import User, PlayerProfile, InviteCode, Token, Session, Texture
import time
import uuid
import re
from utils.pagination import CursorEncoder

class UserModule:
    def __init__(self, db: BaseDB):
        self.db = db

    # ========== User ==========
    async def get_by_email(self, email: str) -> User | None:
        row = await self.db.fetchrow(
            "SELECT id, email, password, is_admin, preferred_language, display_name, banned_until FROM users WHERE email=$1",
            email,
        )
        if row:
            return User(*row)
        return None

    async def get_by_id(self, user_id: str) -> User | None:
        row = await self.db.fetchrow(
            "SELECT id, email, password, is_admin, preferred_language, display_name, banned_until FROM users WHERE id=$1",
            user_id,
        )
        if row:
            return User(*row)
        return None

    async def create(self, user: User):
        await self.db.execute(
            "INSERT INTO users (id, email, password, is_admin, display_name) VALUES ($1, $2, $3, $4, $5)",
            user.id, user.email, user.password, user.is_admin, user.display_name,
        )

    async def update_password(self, user_id: str, new_password_hash: str):
        await self.db.execute(
            "UPDATE users SET password=$1 WHERE id=$2", new_password_hash, user_id
        )

    async def update_email(self, user_id: str, new_email: str):
        await self.db.execute(
            "UPDATE users SET email=$1 WHERE id=$2", new_email, user_id
        )
            
    async def update_display_name(self, user_id: str, new_display_name: str):
        await self.db.execute(
            "UPDATE users SET display_name=$1 WHERE id=$2",
            new_display_name, user_id,
        )

    async def update_preferred_language(self, user_id: str, preferred_language: str):
        await self.db.execute(
            "UPDATE users SET preferred_language=$1 WHERE id=$2",
            preferred_language, user_id,
        )

    async def is_display_name_taken(
        self, display_name: str, exclude_user_id: str | None = None
    ) -> bool:
        if exclude_user_id:
            query = "SELECT 1 FROM users WHERE display_name = $1 AND id != $2"
            val = await self.db.fetchval(query, display_name, exclude_user_id)
        else:
            query = "SELECT 1 FROM users WHERE display_name = $1"
            val = await self.db.fetchval(query, display_name)
        return val is not None

    async def delete(self, user_id: str):
        async with self.db.get_conn() as conn:
            async with conn.transaction():
                await conn.execute("DELETE FROM profiles WHERE user_id=$1", user_id)
                await conn.execute("DELETE FROM tokens WHERE user_id=$1", user_id)
                await conn.execute("DELETE FROM user_textures WHERE user_id=$1", user_id)
                await conn.execute("DELETE FROM users WHERE id=$1", user_id)

    async def count(self) -> int:
        return await self.db.fetchval("SELECT COUNT(*) FROM users") or 0

    async def list_users_cursor(self, limit: int = 20, last_id: str | None = None) -> dict:
        """按ID游标分页获取用户列表"""
        actual_limit = limit + 1
        if last_id:
            rows = await self.db.fetch(
                "SELECT id, email, display_name, is_admin, banned_until, preferred_language FROM users WHERE id > $1 ORDER BY id LIMIT $2",
                last_id, actual_limit
            )
        else:
            rows = await self.db.fetch(
                "SELECT id, email, display_name, is_admin, banned_until, preferred_language FROM users ORDER BY id LIMIT $1",
                actual_limit
            )
        
        has_next = len(rows) > limit
        items = [User(r[0], r[1], "", r[3], r[5], r[2], r[4]) for r in rows[:limit]]
        
        next_cursor = None
        if has_next:
            next_cursor = CursorEncoder.encode({"last_id": rows[limit][0]})
        
        return {
            "items": items,
            "has_next": has_next,
            "next_cursor": next_cursor,
            "page_size": len(items),
        }

    async def toggle_admin(self, user_id: str) -> int:
        async with self.db.get_conn() as conn:
            async with conn.transaction():
                is_admin = await conn.fetchval("SELECT is_admin FROM users WHERE id=$1", user_id)
                if is_admin is None:
                    return -1
                new_status = not is_admin
                await conn.execute("UPDATE users SET is_admin=$1 WHERE id=$2", new_status, user_id)
                return 1 if new_status else 0
            
    async def ban(self, user_id: str, banned_until: int):
        await self.db.execute(
            "UPDATE users SET banned_until=$1 WHERE id=$2", banned_until, user_id
        )

    async def unban(self, user_id: str):
        await self.db.execute(
            "UPDATE users SET banned_until=NULL WHERE id=$1", user_id
        )
            
    async def is_banned(self, user_id: str) -> bool:
        banned_until = await self.db.fetchval("SELECT banned_until FROM users WHERE id=$1", user_id)
        if banned_until:
            current_time = int(time.time() * 1000)
            return current_time < banned_until
        return False

    # ========== Profile ==========

    async def get_profile_by_id(self, profile_id: str) -> PlayerProfile | None:
        row = await self.db.fetchrow(
            "SELECT id, user_id, name, texture_model, skin_hash, cape_hash FROM profiles WHERE id=$1",
            profile_id,
        )
        if row:
            return PlayerProfile(*row)
        return None

    async def get_profile_by_name(self, name: str) -> PlayerProfile | None:
        row = await self.db.fetchrow(
            "SELECT id, user_id, name, texture_model, skin_hash, cape_hash FROM profiles WHERE name=$1",
            name,
        )
        if row:
            return PlayerProfile(*row)
        return None

    async def get_profiles_by_user(self, user_id: str, limit: int = 100) -> list[PlayerProfile]:
        rows = await self.db.fetch(
            "SELECT id, user_id, name, texture_model, skin_hash, cape_hash FROM profiles WHERE user_id=$1 ORDER BY id LIMIT $2",
            user_id, limit,
        )
        return [PlayerProfile(*r) for r in rows]

    async def get_profiles_by_user_cursor(self, user_id: str, limit: int = 20, last_id: str | None = None) -> dict:
        """按ID游标分页获取用户角色列表"""
        actual_limit = limit + 1
        if last_id:
            rows = await self.db.fetch(
                "SELECT id, user_id, name, texture_model, skin_hash, cape_hash FROM profiles WHERE user_id=$1 AND id > $2 ORDER BY id LIMIT $3",
                user_id, last_id, actual_limit
            )
        else:
            rows = await self.db.fetch(
                "SELECT id, user_id, name, texture_model, skin_hash, cape_hash FROM profiles WHERE user_id=$1 ORDER BY id LIMIT $2",
                user_id, actual_limit
            )
        
        has_next = len(rows) > limit
        items = [PlayerProfile(*r) for r in rows[:limit]]
        
        next_cursor = None
        if has_next:
            next_cursor = CursorEncoder.encode({"last_id": rows[limit][0]})
        
        return {
            "items": items,
            "has_next": has_next,
            "next_cursor": next_cursor,
            "page_size": len(items),
        }

    async def create_profile(self, profile: PlayerProfile):
        await self.db.execute(
            "INSERT INTO profiles (id, user_id, name, texture_model) VALUES ($1, $2, $3, $4)",
            profile.id, profile.user_id, profile.name, profile.texture_model,
        )
            
    async def delete_profile(self, profile_id: str):
        await self.db.execute("DELETE FROM profiles WHERE id=$1", profile_id)

    async def verify_profile_ownership(self, user_id: str, profile_id: str) -> bool:
        val = await self.db.fetchval(
            "SELECT 1 FROM profiles WHERE id=$1 AND user_id=$2",
            profile_id, user_id,
        )
        return val is not None

    async def update_profile_skin(self, profile_id: str, skin_hash: str):
        await self.db.execute(
            "UPDATE profiles SET skin_hash=$1 WHERE id=$2", skin_hash, profile_id
        )

    async def update_profile_cape(self, profile_id: str, cape_hash: str):
        await self.db.execute(
            "UPDATE profiles SET cape_hash=$1 WHERE id=$2", cape_hash, profile_id
        )
            
    async def update_profile_texture_model(self, profile_id: str, texture_model: str):
        await self.db.execute(
            "UPDATE profiles SET texture_model=$1 WHERE id=$2",
            texture_model, profile_id,
        )

    async def update_profile_name(self, profile_id: str, name: str):
        await self.db.execute(
            "UPDATE profiles SET name=$1 WHERE id=$2",
            name, profile_id,
        )
            
    async def search_profiles_by_names(self, names: list[str], limit: int = 20) -> list[PlayerProfile]:
        # asyncpg handle array nicely with ANY
        rows = await self.db.fetch(
            "SELECT id, user_id, name, texture_model, skin_hash, cape_hash FROM profiles WHERE name = ANY($1) LIMIT $2",
            names, limit,
        )
        return [PlayerProfile(*r) for r in rows]
                
    async def count_profiles_by_user(self, user_id: str) -> int:
        return await self.db.fetchval("SELECT COUNT(*) FROM profiles WHERE user_id=$1", user_id) or 0

    async def get_display_names_by_ids(self, user_ids: list[str]) -> dict[str, str]:
        if not user_ids:
            return {}
        rows = await self.db.fetch(
            "SELECT id, display_name FROM users WHERE id = ANY($1)",
            user_ids,
        )
        return {r[0]: r[1] or "" for r in rows}

    # ========== Tokens ==========
    
    async def add_token(self, token: Token):
        await self.db.execute(
            "INSERT INTO tokens (access_token, client_token, user_id, profile_id, created_at) VALUES ($1, $2, $3, $4, $5)",
            token.access_token, token.client_token, token.user_id, token.profile_id, token.created_at,
        )

    async def get_token(self, access_token: str) -> Token | None:
        row = await self.db.fetchrow(
            "SELECT access_token, client_token, user_id, profile_id, created_at FROM tokens WHERE access_token=$1",
            access_token,
        )
        if row:
            return Token(*row)
        return None

    async def delete_token(self, access_token: str):
        await self.db.execute("DELETE FROM tokens WHERE access_token=$1", access_token)

    async def delete_tokens_by_user(self, user_id: str):
        await self.db.execute("DELETE FROM tokens WHERE user_id=$1", user_id)

    async def delete_expired_tokens(self, user_id: str, cutoff: int):
        await self.db.execute(
            "DELETE FROM tokens WHERE user_id=$1 AND created_at < $2",
            user_id, cutoff,
        )

    async def delete_surplus_tokens(self, user_id: str, keep: int = 5):
        # Using a subquery in DELETE is fine in PG
        await self.db.execute(
            """
            DELETE FROM tokens 
            WHERE user_id = $1 
            AND access_token NOT IN (
                SELECT access_token 
                FROM tokens 
                WHERE user_id = $1 
                ORDER BY created_at DESC 
                LIMIT $2
            )
            """,
            user_id, keep,
        )
            
    # ========== Sessions ==========
    
    async def add_session(self, session: Session):
        await self.db.execute(
            "INSERT INTO sessions (server_id, access_token, ip, created_at) VALUES ($1, $2, $3, $4)",
            session.server_id, session.access_token, session.ip, session.created_at,
        )

    async def delete_session(self, server_id: str):
        await self.db.execute("DELETE FROM sessions WHERE server_id=$1", server_id)

    async def get_session(self, server_id: str) -> Session | None:
        row = await self.db.fetchrow(
            "SELECT server_id, access_token, ip, created_at FROM sessions WHERE server_id=$1",
            server_id,
        )
        if row:
            return Session(*row)
        return None

    # ========== Invites ==========

    async def get_invite(self, code: str) -> InviteCode | None:
        row = await self.db.fetchrow(
            "SELECT code, created_at, used_by, total_uses, used_count, note FROM invites WHERE code=$1",
            code,
        )
        if row:
            return InviteCode(*row)
        return None

    async def create_invite(self, code: InviteCode):
        await self.db.execute(
            "INSERT INTO invites (code, created_at, total_uses, used_count, note) VALUES ($1, $2, $3, 0, $4)",
            code.code, code.created_at, code.total_uses, code.note,
        )

    async def use_invite(self, code: str, used_by: str = None):
        async with self.db.get_conn() as conn:
            async with conn.transaction():
                await conn.execute(
                    "UPDATE invites SET used_count = used_count + 1 WHERE code=$1", code
                )
                if used_by:
                    await conn.execute(
                        "UPDATE invites SET used_by=$1 WHERE code=$2 AND used_by IS NULL",
                        used_by, code
                    )

    async def count_invites(self) -> int:
        return await self.db.fetchval("SELECT COUNT(*) FROM invites") or 0

    async def list_invites_cursor(self, limit: int = 15, last_created_at: int | None = None, last_code: str | None = None) -> dict:
        """按created_at+code游标分页获取邀请码列表（时序复合游标）"""
        actual_limit = limit + 1
        
        if last_created_at is not None and last_code:
            rows = await self.db.fetch(
                """SELECT code, created_at, used_by, total_uses, used_count, note FROM invites
                   WHERE (created_at < $1) OR (created_at = $1 AND code < $2)
                   ORDER BY created_at DESC, code DESC LIMIT $3""",
                last_created_at, last_code, actual_limit
            )
        else:
            rows = await self.db.fetch(
                "SELECT code, created_at, used_by, total_uses, used_count, note FROM invites ORDER BY created_at DESC, code DESC LIMIT $1",
                actual_limit
            )
        
        has_next = len(rows) > limit
        items = [InviteCode(*r) for r in rows[:limit]]
        
        next_cursor = None
        if has_next:
            last_row = rows[limit]
            next_cursor = CursorEncoder.encode({
                "last_created_at": last_row[1],
                "last_code": last_row[0]
            })
        
        return {
            "items": items,
            "has_next": has_next,
            "next_cursor": next_cursor,
            "page_size": len(items),
        }

    async def delete_invite(self, code: str):
        await self.db.execute("DELETE FROM invites WHERE code=$1", code)
