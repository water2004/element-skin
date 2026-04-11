from ..core import BaseDB
import asyncpg
import time
from typing import Optional, Tuple
import os
from PIL import Image
from io import BytesIO
from utils.pagination import CursorEncoder

# Import from utils, this assumes correct python path
from utils.image_utils import (
    validate_texture_dimensions,
    compute_texture_hash_from_image,
    normalize_png,
)
from config_loader import config

class TextureModule:
    def __init__(self, db: BaseDB):
        self.db = db
        self.textures_dir = config.get("textures.directory", "textures")
        os.makedirs(self.textures_dir, exist_ok=True)

    async def upload(
        self, user_id: str, file_bytes: bytes, texture_type: str, note: str = "", is_public: bool = False, model: str = "default"
    ) -> Tuple[str, str]:
        """
        验证、保存并记录材质
        """
        # 规范化图像
        normalized_bytes, img = normalize_png(file_bytes)

        # 验证尺寸
        is_cape = texture_type.lower() == "cape"
        if not validate_texture_dimensions(img, is_cape):
            raise ValueError("Invalid texture dimensions")

        # 计算哈希
        texture_hash = compute_texture_hash_from_image(img)

        # 保存文件
        file_path = os.path.join(self.textures_dir, f"{texture_hash}.png")
        with open(file_path, "wb") as f:
            f.write(normalized_bytes)

        await self.add_to_library(user_id, texture_hash, texture_type, note, is_public, model)

        return texture_hash, texture_type

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
        
        next_cursor = None
        if has_next:
            last_row = rows[limit]
            next_cursor = CursorEncoder.encode({
                "last_created_at": last_row[3],
                "last_hash": last_row[0]
            })
        
        return {
            "items": items,
            "has_next": has_next,
            "next_cursor": next_cursor,
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
    ) -> dict:
        """按created_at+skin_hash游标分页获取公开皮肤库材质"""
        actual_limit = limit + 1
        conditions = []
        params = []
        p_idx = 1

        if only_public:
            conditions.append("is_public = 1")

        if texture_type:
            conditions.append(f"texture_type = ${p_idx}")
            params.append(texture_type)
            p_idx += 1

        if last_created_at is not None and last_skin_hash:
            conditions.append(f"(created_at < ${p_idx} OR (created_at = ${p_idx} AND skin_hash < ${p_idx + 1}))")
            params.extend([last_created_at, last_skin_hash])
            p_idx += 2

        query = "SELECT skin_hash, texture_type, is_public, uploader, created_at, model, name FROM skin_library"
        if conditions:
            query += " WHERE " + " AND ".join(conditions)
        
        query += f" ORDER BY created_at DESC, skin_hash DESC LIMIT ${p_idx}"
        params.append(actual_limit)

        rows = await self.db.fetch(query, *params)
        
        has_next = len(rows) > limit
        items = [
            {
                "hash": r[0],
                "type": r[1],
                "is_public": bool(r[2]),
                "uploader": r[3],
                "created_at": r[4],
                "model": r[5],
                "name": r[6]
            }
            for r in rows[:limit]
        ]
        
        next_cursor = None
        if has_next:
            last_row = rows[limit]
            next_cursor = CursorEncoder.encode({
                "last_created_at": last_row[4],
                "last_skin_hash": last_row[0]
            })
        
        return {
            "items": items,
            "has_next": has_next,
            "next_cursor": next_cursor,
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
                    "SELECT texture_type, model, uploader FROM skin_library WHERE skin_hash = $1", texture_hash
                )
                if not row:
                    return False
                texture_type = row[0]
                model = row[1]
                uploader = row[2]
            
                created_at = int(time.time() * 1000)
                
                # 如果用户是上传者，则恢复为公开状态(1)，否则为收藏状态(2)
                is_public = 1 if uploader == user_id else 2
                
                await conn.execute(
                    "INSERT INTO user_textures (user_id, hash, texture_type, model, is_public, created_at) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING",
                    user_id, texture_hash, texture_type, model, is_public, created_at,
                )
                return True
