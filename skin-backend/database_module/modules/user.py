from ..core import BaseDB
from utils.typing import User, PlayerProfile, InviteCode, Token, Session, Texture
import time
import uuid
import re

class UserModule:
    def __init__(self, db: BaseDB):
        self.db = db

    # ========== User ==========
    async def get_by_email(self, email: str) -> User | None:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT id, email, password, is_admin, preferred_language, display_name, banned_until FROM users WHERE email=?",
                (email,),
            ) as cur:
                usr = await cur.fetchone()
                if usr:
                    return User(*usr)
                return None

    async def get_by_id(self, user_id: str) -> User | None:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT id, email, password, is_admin, preferred_language, display_name, banned_until FROM users WHERE id=?",
                (user_id,),
            ) as cur:
                usr = await cur.fetchone()
                if usr:
                    return User(*usr)
                return None

    async def create(self, user: User):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "INSERT INTO users (id, email, password, is_admin, display_name) VALUES (?, ?, ?, ?, ?)",
                (user.id, user.email, user.password, user.is_admin, user.display_name),
            )
            await conn.commit()

    async def update_password(self, user_id: str, new_password_hash: str):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "UPDATE users SET password=? WHERE id=?", (new_password_hash, user_id)
            )
            await conn.commit()

    async def update_email(self, user_id: str, new_email: str):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "UPDATE users SET email=? WHERE id=?", (new_email, user_id)
            )
            await conn.commit()
            
    async def update_display_name(self, user_id: str, new_display_name: str):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "UPDATE users SET display_name=? WHERE id=?",
                (new_display_name, user_id),
            )
            await conn.commit()

    async def delete(self, user_id: str):
        async with self.db.get_conn() as conn:
            await conn.execute("DELETE FROM profiles WHERE user_id=?", (user_id,))
            await conn.execute("DELETE FROM tokens WHERE user_id=?", (user_id,))
            # await conn.execute("DELETE FROM sessions WHERE id=?", (user_id,))
            # Assuming session logic might need this, though session table structure is different in legacy
            # ↑ Agent 魅力时刻
            await conn.execute("DELETE FROM user_textures WHERE user_id=?", (user_id,))
            await conn.execute("DELETE FROM users WHERE id=?", (user_id,))
            await conn.commit()

    async def count(self) -> int:
        async with self.db.get_conn() as conn:
            async with conn.execute("SELECT COUNT(*) FROM users") as cur:
                row = await cur.fetchone()
                return row[0] if row else 0

    async def list_users(self, limit: int = 20, offset: int = 0) -> list[User]:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT id, email, display_name, is_admin, banned_until, preferred_language FROM users ORDER BY email LIMIT ? OFFSET ?",
                (limit, offset),
            ) as cur:
                rows = await cur.fetchall()
                # SELECT Order: id(0), email(1), display_name(2), is_admin(3), banned_until(4), preferred_language(5)
                # User Init: id, email, password, is_admin, preferred_language, display_name, banned_until
                return [User(r[0], r[1], "", r[3], r[5], r[2], r[4]) for r in rows]

    async def toggle_admin(self, user_id: str) -> int:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT is_admin FROM users WHERE id=?", (user_id,)
            ) as cur:
                row = await cur.fetchone()
                if not row:
                    return -1
                new_status = 0 if row[0] else 1

            await conn.execute(
                "UPDATE users SET is_admin=? WHERE id=?", (new_status, user_id)
            )
            await conn.commit()
            return new_status
            
    async def ban(self, user_id: str, banned_until: int):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "UPDATE users SET banned_until=? WHERE id=?", (banned_until, user_id)
            )
            await conn.commit()

    async def unban(self, user_id: str):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "UPDATE users SET banned_until=NULL WHERE id=?", (user_id,)
            )
            await conn.commit()
            
    async def is_banned(self, user_id: str) -> bool:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT banned_until FROM users WHERE id=?", (user_id,)
            ) as cur:
                row = await cur.fetchone()
                if row and row[0]:
                    current_time = int(time.time() * 1000)
                    return current_time < row[0]
                return False

    # ========== Profile ==========

    async def get_profile_by_id(self, profile_id: str) -> PlayerProfile | None:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT id, user_id, name, texture_model, skin_hash, cape_hash FROM profiles WHERE id=?",
                (profile_id,),
            ) as cur:
                p = await cur.fetchone()
                if p:
                    return PlayerProfile(*p)
                return None

    async def get_profile_by_name(self, name: str) -> PlayerProfile | None:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT id, user_id, name, texture_model, skin_hash, cape_hash FROM profiles WHERE name=?",
                (name,),
            ) as cur:
                p = await cur.fetchone()
                if p:
                    return PlayerProfile(*p)
                return None

    async def get_profiles_by_user(self, user_id: str) -> list[PlayerProfile]:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT id, user_id, name, texture_model, skin_hash, cape_hash FROM profiles WHERE user_id=?",
                (user_id,),
            ) as cur:
                rows = await cur.fetchall()
                return [PlayerProfile(*r) for r in rows]

    async def create_profile(self, profile: PlayerProfile):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "INSERT INTO profiles (id, user_id, name, texture_model) VALUES (?, ?, ?, ?)",
                (
                    profile.id,
                    profile.user_id,
                    profile.name,
                    profile.texture_model,
                ),
            )
            await conn.commit()
            
    async def delete_profile(self, profile_id: str):
        async with self.db.get_conn() as conn:
            await conn.execute("DELETE FROM profiles WHERE id=?", (profile_id,))
            await conn.commit()

    async def verify_profile_ownership(self, user_id: str, profile_id: str) -> bool:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT 1 FROM profiles WHERE id=? AND user_id=?",
                (profile_id, user_id),
            ) as cur:
                row = await cur.fetchone()
                return row is not None

    async def update_profile_skin(self, profile_id: str, skin_hash: str):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "UPDATE profiles SET skin_hash=? WHERE id=?", (skin_hash, profile_id)
            )
            await conn.commit()

    async def update_profile_cape(self, profile_id: str, cape_hash: str):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "UPDATE profiles SET cape_hash=? WHERE id=?", (cape_hash, profile_id)
            )
            await conn.commit()
            
    async def update_profile_texture_model(self, profile_id: str, texture_model: str):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "UPDATE profiles SET texture_model=? WHERE id=?",
                (texture_model, profile_id),
            )
            await conn.commit()
            
    async def search_profiles_by_names(self, names: list[str], limit: int = 20, offset: int = 0) -> list[PlayerProfile]:
         async with self.db.get_conn() as conn:
            placeholders = ",".join("?" * len(names))
            query = f"SELECT id, user_id, name, texture_model, skin_hash, cape_hash FROM profiles WHERE name IN ({placeholders}) LIMIT ? OFFSET ?"
            async with conn.execute(query, (*names, limit, offset)) as cur:
                rows = await cur.fetchall()
                return [PlayerProfile(*r) for r in rows]
                
    async def count_profiles_by_user(self, user_id: str) -> int:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT COUNT(*) FROM profiles WHERE user_id=?", (user_id,)
            ) as cur:
                row = await cur.fetchone()
                return row[0] if row else 0

    async def get_display_names_by_ids(self, user_ids: list[str]) -> dict[str, str]:
        if not user_ids:
            return {}
        async with self.db.get_conn() as conn:
            placeholders = ",".join("?" * len(user_ids))
            query = f"SELECT id, display_name FROM users WHERE id IN ({placeholders})"
            async with conn.execute(query, user_ids) as cur:
                rows = await cur.fetchall()
                return {r[0]: r[1] or "" for r in rows}

    # ========== Tokens ==========
    
    async def add_token(self, token: Token):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "INSERT INTO tokens (access_token, client_token, user_id, profile_id, created_at) VALUES (?, ?, ?, ?, ?)",
                (
                    token.access_token,
                    token.client_token,
                    token.user_id,
                    token.profile_id,
                    token.created_at,
                ),
            )
            await conn.commit()

    async def get_token(self, access_token: str) -> Token | None:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT access_token, client_token, user_id, profile_id, created_at FROM tokens WHERE access_token=?",
                (access_token,),
            ) as cur:
                row = await cur.fetchone()
                if row:
                    return Token(*row)
                return None

    async def delete_token(self, access_token: str):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "DELETE FROM tokens WHERE access_token=?", (access_token,)
            )
            await conn.commit()

    async def delete_expired_tokens(self, user_id: str, cutoff: int):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "DELETE FROM tokens WHERE user_id=? AND created_at < ?",
                (user_id, cutoff),
            )
            await conn.commit()

    async def delete_surplus_tokens(self, user_id: str, keep: int = 5):
        async with self.db.get_conn() as conn:
            await conn.execute(
                """
                DELETE FROM tokens 
                WHERE user_id = ? 
                AND access_token NOT IN (
                    SELECT access_token 
                    FROM tokens 
                    WHERE user_id = ? 
                    ORDER BY created_at DESC 
                    LIMIT ?
                )
                """,
                (user_id, user_id, keep),
            )
            await conn.commit()
            
    # ========== Sessions ==========
    
    async def add_session(self, session: Session):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "INSERT INTO sessions (server_id, access_token, ip, created_at) VALUES (?, ?, ?, ?)",
                (
                    session.server_id,
                    session.access_token,
                    session.ip,
                    session.created_at,
                ),
            )
            await conn.commit()

    async def delete_session(self, server_id: str):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "DELETE FROM sessions WHERE server_id=?", (server_id,)
            )
            await conn.commit()

    async def get_session(self, server_id: str) -> Session | None:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT server_id, access_token, ip, created_at FROM sessions WHERE server_id=?",
                (server_id,),
            ) as cur:
                row = await cur.fetchone()
                if row:
                    return Session(*row)
                return None

    # ========== Invites ==========

    async def get_invite(self, code: str) -> InviteCode | None:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT code, created_at, used_by, total_uses, used_count, note FROM invites WHERE code=?",
                (code,),
            ) as cur:
                invite = await cur.fetchone()
                if invite:
                    return InviteCode(*invite)
                return None

    async def create_invite(self, code: InviteCode):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "INSERT INTO invites (code, created_at, total_uses, used_count, note) VALUES (?, ?, ?, 0, ?)",
                (code.code, code.created_at, code.total_uses, code.note),
            )
            await conn.commit()

    async def use_invite(self, code: str, used_by: str = None):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "UPDATE invites SET used_count = used_count + 1 WHERE code=?", (code,)
            )
            if used_by:
                await conn.execute(
                    "UPDATE invites SET used_by=? WHERE code=? AND used_by IS NULL",
                    (used_by, code),
                )
            await conn.commit()

    async def list_invites(self) -> list[InviteCode]:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT code, created_at, used_by, total_uses, used_count, note FROM invites ORDER BY created_at DESC"
            ) as cur:
                rows = await cur.fetchall()
                return [InviteCode(*r) for r in rows]

    async def delete_invite(self, code: str):
        async with self.db.get_conn() as conn:
            await conn.execute("DELETE FROM invites WHERE code=?", (code,))
            await conn.commit()

    # ========== Official (Mojang) Account Whitelist ==========

    async def add_official_whitelist_user(self, username: str):
        created_at = int(time.time() * 1000)
        async with self.db.get_conn() as conn:
            await conn.execute(
                "INSERT OR IGNORE INTO official_whitelist (username, created_at) VALUES (?, ?)",
                (username, created_at),
            )
            await conn.commit()

    async def remove_official_whitelist_user(self, username: str):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "DELETE FROM official_whitelist WHERE username=?", (username,)
            )
            await conn.commit()

    async def is_user_in_official_whitelist(self, username: str) -> bool:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT 1 FROM official_whitelist WHERE username=? COLLATE NOCASE",
                (username,),
            ) as cur:
                row = await cur.fetchone()
                return row is not None

    async def list_official_whitelist_users(self) -> list[dict]:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT username, created_at FROM official_whitelist ORDER BY created_at DESC"
            ) as cur:
                rows = await cur.fetchall()
                return [{"username": r[0], "created_at": r[1]} for r in rows]
