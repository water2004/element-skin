"""远程 Yggdrasil 角色导入后端"""

import asyncio
import logging
from typing import Dict, List

from fastapi import HTTPException

from backends.yggdrasil_client import YggdrasilClient, download_texture
from utils.profile_naming import generate_unique_profile_name
from utils.typing import PlayerProfile, normalize_texture_model
from database_module import Database
from services import TextureStorage, assert_texture_size, resolve_max_texture_bytes

logger = logging.getLogger(__name__)


class ProfileImportBackend:
    def __init__(self, db: Database, texture_storage: TextureStorage):
        self.db = db
        self.texture_storage = texture_storage

    async def _import_texture(
        self, user_id: str, texture_bytes: bytes, texture_type: str, note: str, model: str = "default"
    ) -> str:
        await assert_texture_size(self.db, texture_bytes)
        texture_hash, created = await self.texture_storage.process_and_save_async_tracked(
            texture_bytes, texture_type
        )
        try:
            await self.db.texture.add_to_library(
                user_id, texture_hash, texture_type, note, is_public=False, model=model
            )
        except Exception:
            if created:
                try:
                    if not await self.db.texture.exists(texture_hash, texture_type):
                        await asyncio.to_thread(self.texture_storage.delete_file, texture_hash)
                except Exception:
                    pass
            raise
        return texture_hash

    async def get_ygg_profiles(self, api_url: str, username: str, password: str):
        client = YggdrasilClient(api_url)
        try:
            result = await client.authenticate(username, password)
            profiles = result.get("availableProfiles", [])
            return {"profiles": profiles}
        except HTTPException:
            raise  # 已是面向用户的业务错误，原样抛
        except Exception:
            # 不回显底层异常（含远端 URL/连接细节）；服务端记完整堆栈
            logger.warning("get_ygg_profiles failed", exc_info=True)
            raise HTTPException(status_code=400, detail="无法获取远端资料，请检查账号或稍后重试")

    async def _import_single_ygg_profile(
        self,
        user_id: str,
        api_url: str,
        profile_id: str,
        profile_name: str,
        client: YggdrasilClient,
    ):
        profile_data = await client.get_profile_with_textures(profile_id)

        if await self.db.user.get_profile_by_id(profile_id):
            raise HTTPException(status_code=400, detail="该角色 UUID 已在本地存在，无法导入")

        async def _name_exists(n: str) -> bool:
            return await self.db.user.get_profile_by_name(n) is not None

        target_name = await generate_unique_profile_name(profile_name, _name_exists)

        skin_hash = None
        skin_model = "default"
        max_bytes = await resolve_max_texture_bytes(self.db)
        if profile_data.get("skins"):
            skin_url = profile_data["skins"][0]["url"]
            skin_variant = profile_data["skins"][0].get("variant", "classic")
            skin_model = normalize_texture_model(skin_variant)
            try:
                skin_bytes = await download_texture(skin_url, max_bytes=max_bytes)
                skin_hash = await self._import_texture(
                    user_id, skin_bytes, "skin", f"Imported from {api_url}", model=skin_model
                )
            except Exception as e:
                logger.warning("Failed to download/upload skin: %s", e)

        cape_hash = None
        if profile_data.get("capes"):
            cape_url = profile_data["capes"][0]["url"]
            try:
                cape_bytes = await download_texture(cape_url, max_bytes=max_bytes)
                cape_hash = await self._import_texture(
                    user_id, cape_bytes, "cape", f"Imported from {api_url}"
                )
            except Exception as e:
                logger.warning("Failed to download/upload cape: %s", e)

        await self.db.user.create_profile(
            PlayerProfile(profile_id, user_id, target_name, skin_model)
        )

        if skin_hash:
            await self.db.user.update_profile_skin(profile_id, skin_hash)
        if cape_hash:
            await self.db.user.update_profile_cape(profile_id, cape_hash)

        return {"id": profile_id, "name": target_name}

    async def import_ygg_profile(self, user_id: str, api_url: str, profile_id: str, profile_name: str):
        client = YggdrasilClient(api_url)
        try:
            return await self._import_single_ygg_profile(user_id, api_url, profile_id, profile_name, client)
        except HTTPException:
            raise  # 已是面向用户的业务错误，原样抛
        except Exception:
            logger.warning("profile import failed for %s", profile_id, exc_info=True)
            raise HTTPException(status_code=400, detail="导入失败，请稍后重试")

    async def import_ygg_profiles(self, user_id: str, api_url: str, profiles: List[Dict[str, str]]):
        if not isinstance(profiles, list):
            raise HTTPException(status_code=400, detail="profiles must be a list")
        if not profiles:
            raise HTTPException(status_code=400, detail="profiles cannot be empty")

        client = YggdrasilClient(api_url)
        succeeded = []
        failed = []

        for profile in profiles:
            profile_id = str(profile.get("profile_id", "")).strip()
            profile_name = str(profile.get("profile_name", "")).strip()
            if not profile_id or not profile_name:
                failed.append({
                    "profile_id": profile_id,
                    "profile_name": profile_name,
                    "detail": "profile_id and profile_name are required",
                })
                continue

            try:
                result = await self._import_single_ygg_profile(user_id, api_url, profile_id, profile_name, client)
                succeeded.append(result)
            except HTTPException as exc:
                failed.append({
                    "profile_id": profile_id,
                    "profile_name": profile_name,
                    "detail": exc.detail,
                })
            except Exception:
                logger.warning("batch import item failed: %s", profile_id, exc_info=True)
                failed.append({
                    "profile_id": profile_id,
                    "profile_name": profile_name,
                    "detail": "导入失败",
                })

        return {
            "items": succeeded,
            "success_count": len(succeeded),
            "failure_count": len(failed),
            "failed": failed,
        }
