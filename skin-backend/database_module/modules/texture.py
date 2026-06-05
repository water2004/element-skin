from ..core import BaseDB
import time
from typing import Optional


class TextureModule:
    def __init__(self, db: BaseDB):
        self.db = db

    async def add_to_library(self, user_id: str, texture_hash: str, texture_type: str, note: str = "", is_public: bool = False, model: str = "default") -> bool:
        async with self.db.get_conn() as conn:
            async with conn.transaction():
                created_at = int(time.time() * 1000)
                is_public_val = 1 if is_public else 0
                
                # 记录用户材质
                await conn.execute(
                    "INSERT INTO user_textures (user_id, hash, texture_type, note, model, is_public, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT DO NOTHING",
                    user_id, texture_hash, texture_type, note, model, is_public_val, created_at,
                )
                
                # 记录到全局皮肤库
                await conn.execute(
                    "INSERT INTO skin_library (skin_hash, texture_type, is_public, uploader, model, name, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT DO NOTHING",
                    texture_hash, texture_type, is_public_val, user_id, model, note, created_at,
                )
                return True

    async def delete_from_library(self, user_id: str, texture_hash: str, texture_type: str) -> bool:
        async with self.db.get_conn() as conn:
            async with conn.transaction():
                val = await conn.fetchval(
                    "SELECT 1 FROM user_textures WHERE user_id=$1 AND hash=$2 AND texture_type=$3",
                    user_id, texture_hash, texture_type,
                )
                if val is None:
                    return False

                # Delete from user wardrobe
                await conn.execute(
                    "DELETE FROM user_textures WHERE user_id=$1 AND hash=$2 AND texture_type=$3",
                    user_id, texture_hash, texture_type,
                )
                
                # If this user was the uploader, also remove from global skin library
                await conn.execute(
                    "DELETE FROM skin_library WHERE uploader=$1 AND skin_hash=$2",
                    user_id, texture_hash,
                )
                return True

    async def get_for_user_cursor(self, user_id: str, texture_type: Optional[str] = None, limit: int = 20, last_created_at: int | None = None, last_hash: str | None = None) -> dict:
        """按created_at+hash游标分页获取用户材质列表"""
        actual_limit = limit + 1
        
        conditions = ["user_id=$1"]
        params = [user_id]
        p_idx = 2
        
        if texture_type:
            conditions.append(f"texture_type=${p_idx}")
            params.append(texture_type)
            p_idx += 1
        
        if last_created_at is not None and last_hash:
            conditions.append(f"(created_at < ${p_idx} OR (created_at = ${p_idx} AND hash < ${p_idx + 1}))")
            params.extend([last_created_at, last_hash])
            p_idx += 2
        
        query = "SELECT hash, texture_type, note, created_at, model, is_public FROM user_textures WHERE " + " AND ".join(conditions)
        query += f" ORDER BY created_at DESC, hash DESC LIMIT ${p_idx}"
        params.append(actual_limit)
        
        rows = await self.db.fetch(query, *params)
        
        has_next = len(rows) > limit
        items = [{"hash": r[0], "type": r[1], "note": r[2], "created_at": r[3], "model": r[4], "is_public": r[5]} for r in rows[:limit]]
        
        next_key = None
        if has_next:
            last_row = rows[limit - 1]
            next_key = {
                "last_created_at": last_row[3],
                "last_hash": last_row[0]
            }

        return {
            "items": items,
            "has_next": has_next,
            "next_key": next_key,
            "page_size": len(items),
        }

    async def count_for_user(self, user_id: str, texture_type: Optional[str] = None) -> int:
        params = [user_id]
        query = "SELECT COUNT(*) FROM user_textures WHERE user_id=$1"
        if texture_type:
            query += " AND texture_type=$2"
            params.append(texture_type)
        
        val = await self.db.fetchval(query, *params)
        return val or 0

    async def verify_ownership(self, user_id: str, texture_hash: str, texture_type: str) -> bool:
        val = await self.db.fetchval(
            "SELECT 1 FROM user_textures WHERE user_id=$1 AND hash=$2 AND texture_type=$3",
            user_id, texture_hash, texture_type,
        )
        return val is not None

    async def get_texture_info(self, user_id: str, texture_hash: str, texture_type: str) -> Optional[dict]:
        row = await self.db.fetchrow(
            "SELECT hash, texture_type, note, model, created_at, is_public FROM user_textures WHERE user_id=$1 AND hash=$2 AND texture_type=$3",
            user_id, texture_hash, texture_type,
        )
        if row:
            return {
                "hash": row[0],
                "type": row[1],
                "note": row[2],
                "model": row[3],
                "created_at": row[4],
                "is_public": row[5]
            }
        return None

    async def update_note(self, user_id: str, texture_hash: str, texture_type: str, note: str):
        async with self.db.get_conn() as conn:
            async with conn.transaction():
                await conn.execute(
                    "UPDATE user_textures SET note=$1 WHERE user_id=$2 AND hash=$3 AND texture_type=$4",
                    note, user_id, texture_hash, texture_type,
                )
                # 同时更新皮肤库中的名称 (如果是上传者)
                await conn.execute(
                    "UPDATE skin_library SET name=$1 WHERE skin_hash=$2 AND uploader=$3",
                    note, texture_hash, user_id,
                )

    async def update_model(self, user_id: str, texture_hash: str, texture_type: str, model: str):
        async with self.db.get_conn() as conn:
            async with conn.transaction():
                # Update user's wardrobe entry
                await conn.execute(
                    "UPDATE user_textures SET model=$1 WHERE user_id=$2 AND hash=$3 AND texture_type=$4",
                    model, user_id, texture_hash, texture_type,
                )
                # Update library entry if this user is the uploader
                await conn.execute(
                    "UPDATE skin_library SET model=$1 WHERE skin_hash=$2 AND uploader=$3",
                    model, texture_hash, user_id,
                )
                # If it's a skin, also update all profiles using this skin to match the new model
                if texture_type.lower() == "skin":
                    await conn.execute(
                        "UPDATE profiles SET texture_model=$1 WHERE skin_hash=$2 AND user_id=$3",
                        model, texture_hash, user_id,
                    )

    async def update_is_public(self, user_id: str, texture_hash: str, texture_type: str, is_public: bool):
        async with self.db.get_conn() as conn:
            async with conn.transaction():
                is_public_val = 1 if is_public else 0
                # 只有上传者才能修改公开状态 (is_public != 2)
                await conn.execute(
                    "UPDATE user_textures SET is_public=$1 WHERE user_id=$2 AND hash=$3 AND texture_type=$4 AND is_public != 2",
                    is_public_val, user_id, texture_hash, texture_type,
                )
                # 同时更新皮肤库
                await conn.execute(
                    "UPDATE skin_library SET is_public=$1 WHERE skin_hash=$2 AND uploader=$3",
                    is_public_val, texture_hash, user_id,
                )

    async def get_from_library_cursor(
        self,
        limit: int = 20,
        texture_type: Optional[str] = None,
        only_public: bool = True,
        last_created_at: int | None = None,
        last_skin_hash: str | None = None,
        query: str | None = None,
    ) -> dict:
        """按created_at+skin_hash游标分页获取公开皮肤库材质，支持按名称/hash/上传者搜索"""
        actual_limit = limit + 1
        conditions = []
        params = []
        p_idx = 1

        if only_public:
            conditions.append("sl.is_public = 1")

        if texture_type:
            conditions.append(f"sl.texture_type = ${p_idx}")
            params.append(texture_type)
            p_idx += 1

        if query:
            conditions.append(
                f"(sl.skin_hash ILIKE ${p_idx} OR sl.name ILIKE ${p_idx} OR u.display_name ILIKE ${p_idx})"
            )
            params.append(f"%{query}%")
            p_idx += 1

        if last_created_at is not None and last_skin_hash:
            conditions.append(
                f"(sl.created_at < ${p_idx} OR (sl.created_at = ${p_idx} AND sl.skin_hash < ${p_idx + 1}))"
            )
            params.extend([last_created_at, last_skin_hash])
            p_idx += 2

        query_sql = """
            SELECT sl.skin_hash, sl.texture_type, sl.is_public, sl.uploader,
                   sl.created_at, sl.model, sl.name,
                   u.display_name AS uploader_display_name
            FROM skin_library sl
            LEFT JOIN users u ON sl.uploader = u.id
        """
        if conditions:
            query_sql += " WHERE " + " AND ".join(conditions)

        query_sql += f" ORDER BY sl.created_at DESC, sl.skin_hash DESC LIMIT ${p_idx}"
        params.append(actual_limit)

        rows = await self.db.fetch(query_sql, *params)

        has_next = len(rows) > limit
        items = [
            {
                "hash": r[0],
                "type": r[1],
                "is_public": bool(r[2]),
                "uploader": r[3],
                "created_at": r[4],
                "model": r[5],
                "name": r[6],
                "uploader_display_name": r[7] or "",
            }
            for r in rows[:limit]
        ]

        next_key = None
        if has_next:
            last_row = rows[limit - 1]
            next_key = {
                "last_created_at": last_row[4],
                "last_skin_hash": last_row[0],
            }

        return {
            "items": items,
            "has_next": has_next,
            "next_key": next_key,
            "page_size": len(items),
        }

    async def list_all_textures_cursor(
        self,
        limit: int = 20,
        last_created_at: int | None = None,
        last_skin_hash: str | None = None,
        query: str | None = None,
        type_filter: str | None = None,
    ) -> dict:
        """全局管理员方法：列出所有材质（公共+私有），支持游标分页、搜索和类型过滤"""
        actual_limit = limit + 1

        conditions = []
        params = []
        p_idx = 1

        # 类型过滤
        if type_filter:
            conditions.append(f"sl.texture_type = ${p_idx}")
            params.append(type_filter)
            p_idx += 1

        # 搜索 (ILIKE on skin_hash and name)
        if query:
            conditions.append(f"(sl.skin_hash ILIKE ${p_idx} OR sl.name ILIKE ${p_idx} OR u.display_name ILIKE ${p_idx})")
            params.append(f"%{query}%")
            p_idx += 1

        # 游标条件 (created_at DESC, skin_hash DESC)
        if last_created_at is not None and last_skin_hash:
            conditions.append(
                f"(sl.created_at < ${p_idx} OR (sl.created_at = ${p_idx} AND sl.skin_hash < ${p_idx + 1}))"
            )
            params.extend([last_created_at, last_skin_hash])
            p_idx += 2

        query_sql = """
            SELECT sl.skin_hash, sl.texture_type, sl.is_public, sl.uploader,
                   sl.created_at, sl.model, sl.name,
                   u.email AS uploader_email, u.display_name AS uploader_display_name
            FROM skin_library sl
            LEFT JOIN users u ON sl.uploader = u.id
        """
        if conditions:
            query_sql += " WHERE " + " AND ".join(conditions)

        query_sql += f" ORDER BY sl.created_at DESC, sl.skin_hash DESC LIMIT ${p_idx}"
        params.append(actual_limit)

        rows = await self.db.fetch(query_sql, *params)

        has_next = len(rows) > limit
        items = [
            {
                "hash": r[0],
                "type": r[1],
                "is_public": bool(r[2]),
                "uploader_user_id": r[3],
                "created_at": r[4],
                "model": r[5],
                "name": r[6],
                "uploader_email": r[7],
                "uploader_display_name": r[8],
            }
            for r in rows[:limit]
        ]

        next_key = None
        if has_next:
            last_row = rows[limit - 1]
            next_key = {
                "last_created_at": last_row[4],
                "last_skin_hash": last_row[0],
            }

        return {
            "items": items,
            "has_next": has_next,
            "next_key": next_key,
            "page_size": len(items),
        }

    async def count_library(self, texture_type: Optional[str] = None, only_public: bool = True) -> int:
        query = "SELECT COUNT(*) FROM skin_library"
        conditions = []
        params = []
        p_idx = 1
        if only_public:
            conditions.append("is_public = 1")
        if texture_type:
            conditions.append(f"texture_type = ${p_idx}")
            params.append(texture_type)
            p_idx += 1
        
        if conditions:
            query += " WHERE " + " AND ".join(conditions)
        
        val = await self.db.fetchval(query, *params)
        return val or 0

    async def add_to_user_wardrobe(self, user_id: str, texture_hash: str) -> bool:
        """
        从公共库添加材质到用户衣柜
        """
        async with self.db.get_conn() as conn:
            async with conn.transaction():
                # 获取材质信息
                row = await conn.fetchrow(
                    "SELECT texture_type, model, uploader, name, is_public FROM skin_library WHERE skin_hash = $1", texture_hash
                )
                if not row:
                    return False
                texture_type = row[0]
                model = row[1]
                uploader = row[2]
                name = row[3] or ""
                src_is_public = row[4]

                # 仅允许：公开材质（任何人可收藏）或自己上传的材质（可找回）。
                # 拒绝他人的私有材质，避免越权读取。
                if src_is_public != 1 and uploader != user_id:
                    return False

                created_at = int(time.time() * 1000)

                # 如果用户是上传者，则恢复为公开状态(1)，否则为收藏状态(2)
                is_public = 1 if uploader == user_id else 2
                
                await conn.execute(
                    "INSERT INTO user_textures (user_id, hash, texture_type, note, model, is_public, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT DO NOTHING",
                    user_id, texture_hash, texture_type, name, model, is_public, created_at,
                )
                return True

    # ========== Admin-facing Base Methods (pure CRUD) ==========

    async def get_texture_from_library(self, texture_hash: str) -> dict | None:
        """获取皮肤库中的材质记录"""
        row = await self.db.fetchrow(
            "SELECT skin_hash, uploader FROM skin_library WHERE skin_hash=$1", texture_hash
        )
        return dict(row) if row else None

    async def delete_texture(self, texture_hash: str, texture_type: str, user_id: str | None = None, force: bool = False):
        """删除材质记录（事务安全）。
        
        force=True: 删除所有用户引用 + 皮肤库记录
        force=False + user_id: 删除单个用户引用，若剩余为0则物理删除皮肤库记录
        """
        if not force and not user_id:
            raise ValueError("per-user deletion requires user_id")
        
        async with self.db.get_conn() as conn:
            async with conn.transaction():
                if force:
                    await conn.execute(
                        "DELETE FROM user_textures WHERE hash=$1 AND texture_type=$2",
                        texture_hash, texture_type,
                    )
                    await conn.execute(
                        "DELETE FROM skin_library WHERE skin_hash=$1",
                        texture_hash,
                    )
                else:
                    await conn.execute(
                        "DELETE FROM user_textures WHERE user_id=$1 AND hash=$2 AND texture_type=$3",
                        user_id, texture_hash, texture_type,
                    )
                    remaining = await conn.fetchval(
                        "SELECT COUNT(*) FROM user_textures WHERE hash=$1 AND texture_type=$2",
                        texture_hash, texture_type,
                    )
                    if remaining == 0:
                        await conn.execute(
                            "DELETE FROM skin_library WHERE skin_hash=$1",
                            texture_hash,
                        )

