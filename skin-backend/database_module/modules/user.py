from ..core import BaseDB
from utils.typing import User, PlayerProfile, InviteCode, Token, Session
import time
import asyncpg


class InviteExhaustedError(Exception):
    """注册事务内邀请码已无剩余次数，用于回滚整笔建号事务。"""
    pass


class DisplayNameConflictError(Exception):
    """display_name 唯一性争用：在持有 advisory 锁的事务内仍发现冲突。"""
    pass


# 与 go 实现保持一致的常量种子，保证不同进程对同一 display_name
# 取出同一把 advisory 锁。
_DISPLAY_NAME_LOCK_SEED = 0x444953504C4159


class UserModule:
    def __init__(self, db: BaseDB):
        self.db = db

    # ========== User ==========
    async def get_by_email(self, email: str) -> User | None:
        row = await self.db.fetchrow(
            "SELECT id, email, password, is_admin, preferred_language, display_name, banned_until, avatar_hash FROM users WHERE email=$1",
            email,
        )
        if row:
            return User(*row)
        return None

    async def get_by_id(self, user_id: str) -> User | None:
        row = await self.db.fetchrow(
            "SELECT id, email, password, is_admin, preferred_language, display_name, banned_until, avatar_hash FROM users WHERE id=$1",
            user_id,
        )
        if row:
            return User(*row)
        return None

    async def create(self, user: User):
        await self.db.execute(
            "INSERT INTO users (id, email, password, is_admin, display_name, avatar_hash) VALUES ($1, $2, $3, $4, $5, $6)",
            user.id, user.email, user.password, user.is_admin, user.display_name, user.avatar_hash,
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

    async def update_display_name_safely(self, user_id: str, new_display_name: str) -> bool:
        """事务内通过 advisory 锁串行同名 display_name 检查与写入，避免 TOCTOU 重名。

        同名争用抛 DisplayNameConflictError；用户不存在返回 False。
        """
        async with self.db.get_conn() as conn:
            async with conn.transaction():
                # 取出 user 行的写锁，避免与 delete/update 互相竞争
                exists = await conn.fetchval(
                    "SELECT 1 FROM users WHERE id=$1 FOR UPDATE", user_id
                )
                if not exists:
                    return False
                if new_display_name:
                    # advisory 锁仅在事务期间有效，覆盖所有同 display_name 的并发写入
                    await conn.execute(
                        "SELECT pg_advisory_xact_lock(hashtextextended($1, $2))",
                        new_display_name, _DISPLAY_NAME_LOCK_SEED,
                    )
                    conflict = await conn.fetchval(
                        "SELECT 1 FROM users WHERE display_name=$1 AND id<>$2",
                        new_display_name, user_id,
                    )
                    if conflict:
                        raise DisplayNameConflictError()
                await conn.execute(
                    "UPDATE users SET display_name=$1 WHERE id=$2",
                    new_display_name, user_id,
                )
        return True

    async def update_preferred_language(self, user_id: str, preferred_language: str):
        await self.db.execute(
            "UPDATE users SET preferred_language=$1 WHERE id=$2",
            preferred_language, user_id,
        )
            
    async def update_avatar_hash(self, user_id: str, avatar_hash: str | None):
        await self.db.execute(
            "UPDATE users SET avatar_hash=$1 WHERE id=$2",
            avatar_hash, user_id,
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
                # Lock the user row first; bail out cleanly if it has already been deleted.
                exists = await conn.fetchval(
                    "SELECT 1 FROM users WHERE id=$1 FOR UPDATE", user_id
                )
                if not exists:
                    return False
                await conn.execute("DELETE FROM profiles WHERE user_id=$1", user_id)
                await conn.execute("DELETE FROM tokens WHERE user_id=$1", user_id)
                await conn.execute("DELETE FROM site_refresh_tokens WHERE user_id=$1", user_id)
                # 删除该用户上传的库材质所对应的所有用户衣柜引用
                await conn.execute(
                    """
                    DELETE FROM user_textures
                    WHERE (hash, texture_type) IN (
                        SELECT skin_hash, texture_type FROM skin_library WHERE uploader=$1
                    )
                    """,
                    user_id,
                )
                # 再删该用户上传到全局库的记录
                await conn.execute(
                    "DELETE FROM skin_library WHERE uploader=$1", user_id
                )
                # 最后删除该用户自己的衣柜行（如有剩余）
                await conn.execute("DELETE FROM user_textures WHERE user_id=$1", user_id)
                await conn.execute("DELETE FROM users WHERE id=$1", user_id)
                return True

    async def count(self) -> int:
        return await self.db.fetchval("SELECT COUNT(*) FROM users") or 0

    async def list_users_cursor(self, limit: int = 20, last_id: str | None = None) -> dict:
        """按ID游标分页获取用户列表"""
        actual_limit = limit + 1
        if last_id:
            rows = await self.db.fetch(
                "SELECT id, email, display_name, is_admin, banned_until, preferred_language, avatar_hash FROM users WHERE id > $1 ORDER BY id LIMIT $2",
                last_id, actual_limit
            )
        else:
            rows = await self.db.fetch(
                "SELECT id, email, display_name, is_admin, banned_until, preferred_language, avatar_hash FROM users ORDER BY id LIMIT $1",
                actual_limit
            )
        
        has_next = len(rows) > limit
        items = [
            User(
                id=r[0],
                email=r[1],
                password="",
                is_admin=r[3],
                preferred_language=r[5],
                display_name=r[2],
                banned_until=r[4],
                avatar_hash=r[6],
            )
            for r in rows[:limit]
        ]

        next_key = None
        if has_next:
            next_key = {"last_id": rows[limit - 1][0]}

        return {
            "items": items,
            "has_next": has_next,
            "next_key": next_key,
            "page_size": len(items),
        }

    async def search_users_cursor(self, query: str, limit: int = 20, last_id: str | None = None) -> dict:
        """按用户名/邮箱/角色名模糊搜索用户（游标分页）
        
        使用 EXISTS 子查询而非 LEFT JOIN，在找到第一个匹配 profile 后即短路，
        避免 JOIN 膨胀和 DISTINCT 去重的性能开销。
        """
        actual_limit = limit + 1
        like_pattern = f"%{query}%"
        
        search_condition = """
            (display_name ILIKE $1 OR email ILIKE $1
             OR EXISTS (SELECT 1 FROM profiles WHERE profiles.user_id = users.id AND profiles.name ILIKE $1))
        """
        
        if last_id:
            sql = f"""
                SELECT id, email, display_name, is_admin, banned_until, preferred_language, avatar_hash
                FROM users
                WHERE {search_condition} AND id > $2
                ORDER BY id LIMIT $3
            """
            rows = await self.db.fetch(sql, like_pattern, last_id, actual_limit)
        else:
            sql = f"""
                SELECT id, email, display_name, is_admin, banned_until, preferred_language, avatar_hash
                FROM users
                WHERE {search_condition}
                ORDER BY id LIMIT $2
            """
            rows = await self.db.fetch(sql, like_pattern, actual_limit)
        
        has_next = len(rows) > limit
        items = [
            User(
                id=r[0],
                email=r[1],
                password="",
                is_admin=r[3],
                preferred_language=r[5],
                display_name=r[2],
                banned_until=r[4],
                avatar_hash=r[6],
            )
            for r in rows[:limit]
        ]

        next_key = None
        if has_next:
            next_key = {"last_id": rows[limit - 1][0]}

        return {
            "items": items,
            "has_next": has_next,
            "next_key": next_key,
            "page_size": len(items),
        }

    async def toggle_admin(self, user_id: str) -> int:
        # 单条原子语句，避免读改写之间被并发请求抢占。
        row = await self.db.fetchrow(
            """
            UPDATE users
            SET is_admin = NOT is_admin
            WHERE id = $1
            RETURNING is_admin
            """,
            user_id,
        )
        if row is None:
            return -1
        return 1 if row[0] else 0
            
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
        
        next_key = None
        if has_next:
            next_key = {"last_id": rows[limit - 1][0]}
        
        return {
            "items": items,
            "has_next": has_next,
            "next_key": next_key,
            "page_size": len(items),
        }

    async def list_all_profiles_cursor(self, limit: int = 20, after_id: str | None = None, query: str | None = None) -> dict:
        """按ID游标分页获取所有游戏角色（含所属用户信息），支持按角色名/邮箱搜索"""
        actual_limit = limit + 1

        if query:
            like_pattern = f"%{query}%"
            if after_id:
                rows = await self.db.fetch(
                    """SELECT p.id, p.user_id, p.name, p.texture_model, p.skin_hash, p.cape_hash,
                              u.email AS owner_email, u.display_name AS owner_display_name
                       FROM profiles p JOIN users u ON p.user_id = u.id
                       WHERE (p.name ILIKE $1 OR u.email ILIKE $1 OR u.display_name ILIKE $1) AND p.id > $2
                       ORDER BY p.id LIMIT $3""",
                    like_pattern, after_id, actual_limit
                )
            else:
                rows = await self.db.fetch(
                    """SELECT p.id, p.user_id, p.name, p.texture_model, p.skin_hash, p.cape_hash,
                              u.email AS owner_email, u.display_name AS owner_display_name
                       FROM profiles p JOIN users u ON p.user_id = u.id
                       WHERE (p.name ILIKE $1 OR u.email ILIKE $1 OR u.display_name ILIKE $1)
                       ORDER BY p.id LIMIT $2""",
                    like_pattern, actual_limit
                )
        else:
            if after_id:
                rows = await self.db.fetch(
                    """SELECT p.id, p.user_id, p.name, p.texture_model, p.skin_hash, p.cape_hash,
                              u.email AS owner_email, u.display_name AS owner_display_name
                       FROM profiles p JOIN users u ON p.user_id = u.id
                       WHERE p.id > $1
                       ORDER BY p.id LIMIT $2""",
                    after_id, actual_limit
                )
            else:
                rows = await self.db.fetch(
                    """SELECT p.id, p.user_id, p.name, p.texture_model, p.skin_hash, p.cape_hash,
                              u.email AS owner_email, u.display_name AS owner_display_name
                       FROM profiles p JOIN users u ON p.user_id = u.id
                       ORDER BY p.id LIMIT $1""",
                    actual_limit
                )

        has_next = len(rows) > limit
        items = [
            {
                "id": r[0],
                "user_id": r[1],
                "name": r[2],
                "texture_model": r[3],
                "skin_hash": r[4],
                "cape_hash": r[5],
                "owner_email": r[6],
                "owner_display_name": r[7],
            }
            for r in rows[:limit]
        ]

        next_key = None
        if has_next:
            next_key = {"last_id": rows[limit - 1][0]}

        return {
            "items": items,
            "has_next": has_next,
            "next_key": next_key,
            "page_size": len(items),
        }

    async def create_profile(self, profile: PlayerProfile):
        await self.db.execute(
            "INSERT INTO profiles (id, user_id, name, texture_model, skin_hash, cape_hash) VALUES ($1, $2, $3, $4, $5, $6)",
            profile.id, profile.user_id, profile.name, profile.texture_model, profile.skin_hash, profile.cape_hash,
        )

    async def create_user_with_profile(
        self,
        user: User,
        profile: PlayerProfile,
        invite_code: str | None = None,
        used_by: str | None = None,
    ) -> bool:
        """事务内创建 user + profile（可选核销邀请），任一步失败整体回滚。

        - 邮箱/角色名唯一冲突抛 asyncpg.UniqueViolationError，由上层转 400。
        - display_name 通过事务级 advisory 锁串行化重名校验，避免并发注册产生重复昵称。
        - invite_code 给定时，在同一事务内条件核销；无剩余次数抛
          InviteExhaustedError 触发回滚，杜绝「建号成功但邀请超额」。
        返回 True。
        """
        async with self.db.get_conn() as conn:
            async with conn.transaction():
                if user.display_name:
                    await conn.execute(
                        "SELECT pg_advisory_xact_lock(hashtextextended($1, $2))",
                        user.display_name, _DISPLAY_NAME_LOCK_SEED,
                    )
                    conflict = await conn.fetchval(
                        "SELECT 1 FROM users WHERE display_name=$1",
                        user.display_name,
                    )
                    if conflict:
                        raise DisplayNameConflictError()
                await conn.execute(
                    "INSERT INTO users (id, email, password, is_admin, display_name, avatar_hash) VALUES ($1, $2, $3, $4, $5, $6)",
                    user.id, user.email, user.password, user.is_admin, user.display_name, user.avatar_hash,
                )
                await conn.execute(
                    "INSERT INTO profiles (id, user_id, name, texture_model, skin_hash, cape_hash) VALUES ($1, $2, $3, $4, $5, $6)",
                    profile.id, profile.user_id, profile.name, profile.texture_model, profile.skin_hash, profile.cape_hash,
                )
                if invite_code:
                    updated = await conn.execute(
                        "UPDATE invites SET used_count = used_count + 1 "
                        "WHERE code=$1 AND (total_uses IS NULL OR used_count < total_uses)",
                        invite_code,
                    )
                    if updated.split()[-1] == "0":
                        raise InviteExhaustedError(invite_code)
                    if used_by:
                        await conn.execute(
                            "UPDATE invites SET used_by=$1 WHERE code=$2 AND used_by IS NULL",
                            used_by, invite_code,
                        )
        return True

    async def delete_profile_cascade(self, profile_id: str) -> bool:
        """事务内删除角色及其 Yggdrasil 游戏 token，避免孤儿 token。

        返回是否真的删到角色行（token 可能本就为 0 条）。
        """
        async with self.db.get_conn() as conn:
            async with conn.transaction():
                await conn.execute("DELETE FROM tokens WHERE profile_id=$1", profile_id)
                result = await conn.execute("DELETE FROM profiles WHERE id=$1", profile_id)
        return result.split()[-1] != "0"

    async def verify_profile_ownership(self, user_id: str, profile_id: str) -> bool:
        val = await self.db.fetchval(
            "SELECT 1 FROM profiles WHERE id=$1 AND user_id=$2",
            profile_id, user_id,
        )
        return val is not None

    async def update_profile_skin(self, profile_id: str, skin_hash: str | None = None):
        await self.db.execute(
            "UPDATE profiles SET skin_hash=$1 WHERE id=$2", skin_hash, profile_id
        )

    async def update_profile_skin_and_model(
        self, profile_id: str, skin_hash: str | None, texture_model: str
    ):
        """同事务内同时更新 skin_hash 与 texture_model，避免两步写之间出现错配。"""
        await self.db.execute(
            "UPDATE profiles SET skin_hash=$1, texture_model=$2 WHERE id=$3",
            skin_hash, texture_model, profile_id,
        )

    async def update_profile_cape(self, profile_id: str, cape_hash: str | None = None):
        await self.db.execute(
            "UPDATE profiles SET cape_hash=$1 WHERE id=$2", cape_hash, profile_id
        )
            
    async def update_profile_texture_model(self, profile_id: str, texture_model: str):
        await self.db.execute(
            "UPDATE profiles SET texture_model=$1 WHERE id=$2",
            texture_model, profile_id,
        )

    async def update_profile_name(self, profile_id: str, name: str) -> bool:
        """更新角色名，返回是否真的更新到行（不处理验证）。

        0 行更新（profile_id 不存在）返回 False；唯一冲突返回 False。
        """
        try:
            result = await self.db.execute("UPDATE profiles SET name=$1 WHERE id=$2", name, profile_id)
        except asyncpg.exceptions.UniqueViolationError:
            return False
        return result.split()[-1] != "0"

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

    async def rotate_token(self, old_access: str, new_token: Token) -> bool:
        """事务内删除旧 yggdrasil token 并写入新 token；旧 token 不存在或被并发消费返回 False。

        将删除与写入合并为单事务，避免任意单步失败导致用户的 yggdrasil 会话被强制销毁。
        """
        async with self.db.get_conn() as conn:
            async with conn.transaction():
                tag = await conn.execute(
                    "DELETE FROM tokens WHERE access_token=$1", old_access
                )
                deleted = int(tag.split()[-1] or 0)
                if deleted == 0:
                    return False
                await conn.execute(
                    "INSERT INTO tokens (access_token, client_token, user_id, profile_id, created_at) VALUES ($1, $2, $3, $4, $5)",
                    new_token.access_token, new_token.client_token, new_token.user_id, new_token.profile_id, new_token.created_at,
                )
                return True

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
            
    # ========== Site Refresh Tokens ==========
    # 站点会话的 refresh token（不透明随机串，库中仅存 SHA-256 哈希）。
    # 与 Yggdrasil 游戏令牌的 tokens 表完全无关。

    async def add_refresh_token(self, token_hash: str, user_id: str, expires_at: int, created_at: int):
        await self.db.execute(
            "INSERT INTO site_refresh_tokens (token_hash, user_id, expires_at, created_at) VALUES ($1, $2, $3, $4)",
            token_hash, user_id, expires_at, created_at,
        )

    async def get_refresh_token(self, token_hash: str):
        return await self.db.fetchrow(
            "SELECT token_hash, user_id, expires_at, created_at FROM site_refresh_tokens WHERE token_hash=$1",
            token_hash,
        )

    async def delete_refresh_token(self, token_hash: str):
        await self.db.execute("DELETE FROM site_refresh_tokens WHERE token_hash=$1", token_hash)

    async def consume_refresh_token(self, token_hash: str):
        """原子地删除并返回该 refresh token 行（DELETE ... RETURNING）。

        返回被删行（含 user_id, expires_at, created_at）；若 token 不存在或已被
        并发请求消费，返回 None。Postgres 对同一行的并发 DELETE 串行化，只有一个
        事务能 RETURNING 出该行——「拿到行」即「唯一赢家」，轮换的单赢者语义即建于此。
        """
        return await self.db.fetchrow(
            "DELETE FROM site_refresh_tokens WHERE token_hash=$1 "
            "RETURNING token_hash, user_id, expires_at, created_at",
            token_hash,
        )

    async def rotate_refresh_token(
        self,
        old_hash: str,
        new_hash: str,
        user_id: str,
        expires_at: int,
        created_at: int,
    ) -> bool:
        """事务内原子轮换 refresh：删除旧 + 插入新一并提交，避免单步失败导致用户登出。

        返回 True 表示旧行被本次调用删除并已写入新行；False 表示旧行不存在/已被并发
        请求消费——调用方应据此返回 401。期间任意 SQL 错误抛出。
        """
        async with self.db.get_conn() as conn:
            async with conn.transaction():
                tag = await conn.execute(
                    "DELETE FROM site_refresh_tokens WHERE token_hash=$1 AND user_id=$2",
                    old_hash, user_id,
                )
                deleted = int(tag.split()[-1] or 0)
                if deleted == 0:
                    return False
                await conn.execute(
                    "INSERT INTO site_refresh_tokens (token_hash, user_id, expires_at, created_at) VALUES ($1, $2, $3, $4)",
                    new_hash, user_id, expires_at, created_at,
                )
                return True

    async def delete_refresh_tokens_by_user(self, user_id: str):
        await self.db.execute("DELETE FROM site_refresh_tokens WHERE user_id=$1", user_id)

    async def delete_expired_refresh_tokens(self, cutoff: int):
        await self.db.execute("DELETE FROM site_refresh_tokens WHERE expires_at < $1", cutoff)

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
        
        next_key = None
        if has_next:
            last_row = rows[limit - 1]
            next_key = {
                "last_created_at": last_row[1],
                "last_code": last_row[0]
            }
        
        return {
            "items": items,
            "has_next": has_next,
            "next_key": next_key,
            "page_size": len(items),
        }

    async def delete_invite(self, code: str):
        await self.db.execute("DELETE FROM invites WHERE code=$1", code)
