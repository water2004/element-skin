"""
微软账户登录模块
实现 OAuth 2.0 授权码模式和 Minecraft 正版验证
"""

import aiohttp
import asyncio
import logging
import urllib.parse
from typing import Optional, Dict, Tuple

from fastapi import HTTPException

from utils.profile_naming import generate_unique_profile_name
from utils.typing import PlayerProfile, normalize_texture_model
from utils.http import download_texture as download_texture
from services import assert_texture_size, resolve_max_texture_bytes

logger = logging.getLogger(__name__)

# 出站请求统一超时（与 utils/http.py、yggdrasil_client.py 对齐），避免外部端点挂死拖垮连接
_MS_TIMEOUT = aiohttp.ClientTimeout(total=15)


class MicrosoftAuthService:
    """微软账户认证服务（授权码模式）"""

    # OAuth 端点
    OAUTH_AUTHORITY = "https://login.microsoftonline.com/consumers"
    AUTHORIZE_ENDPOINT = f"{OAUTH_AUTHORITY}/oauth2/v2.0/authorize"
    TOKEN_ENDPOINT = f"{OAUTH_AUTHORITY}/oauth2/v2.0/token"

    # Xbox Live 端点
    XBL_AUTH_ENDPOINT = "https://user.auth.xboxlive.com/user/authenticate"
    XSTS_AUTH_ENDPOINT = "https://xsts.auth.xboxlive.com/xsts/authorize"

    # Minecraft 端点
    MC_LOGIN_ENDPOINT = (
        "https://api.minecraftservices.com/authentication/login_with_xbox"
    )
    MC_PROFILE_ENDPOINT = "https://api.minecraftservices.com/minecraft/profile"
    MC_ENTITLEMENTS_ENDPOINT = "https://api.minecraftservices.com/entitlements/mcstore"

    SCOPE = "XboxLive.signin offline_access"

    def __init__(self, client_id: str, client_secret: str, redirect_uri: str):
        self.client_id = client_id
        self.client_secret = client_secret
        self.redirect_uri = redirect_uri

    def get_authorization_url(self, state: str = None) -> str:
        """
        生成微软OAuth授权URL
        参数:
            state: 随机字符串，用于防止CSRF攻击
        返回: 授权URL
        """
        params = {
            "client_id": self.client_id,
            "response_type": "code",
            "redirect_uri": self.redirect_uri,
            "scope": self.SCOPE,
        }

        if state:
            params["state"] = state

        query_string = urllib.parse.urlencode(params)
        return f"{self.AUTHORIZE_ENDPOINT}?{query_string}"

    async def exchange_code_for_token(self, code: str) -> Dict:
        """
        使用授权码交换访问令牌
        参数:
            code: 从回调中获取的授权码
        返回: 令牌信息字典
        """
        async with aiohttp.ClientSession(timeout=_MS_TIMEOUT) as session:
            data = {
                "client_id": self.client_id,
                "client_secret": self.client_secret,
                "code": code,
                "redirect_uri": self.redirect_uri,
                "grant_type": "authorization_code",
            }

            async with session.post(
                self.TOKEN_ENDPOINT,
                data=data,
                headers={"Content-Type": "application/x-www-form-urlencoded"},
            ) as resp:
                if resp.status != 200:
                    error_text = await resp.text()
                    raise Exception(f"Failed to exchange code for token: {error_text}")

                return await resp.json()

    async def authenticate_xbl(self, ms_access_token: str) -> Tuple[str, str]:
        """
        Xbox Live 认证
        返回: (xbl_token, user_hash)
        """
        async with aiohttp.ClientSession(timeout=_MS_TIMEOUT) as session:
            payload = {
                "Properties": {
                    "AuthMethod": "RPS",
                    "SiteName": "user.auth.xboxlive.com",
                    "RpsTicket": f"d={ms_access_token}",
                },
                "RelyingParty": "http://auth.xboxlive.com",
                "TokenType": "JWT",
            }

            async with session.post(
                self.XBL_AUTH_ENDPOINT,
                json=payload,
                headers={
                    "Content-Type": "application/json",
                    "Accept": "application/json",
                },
            ) as resp:
                if resp.status != 200:
                    error_text = await resp.text()
                    raise Exception(f"XBL authentication failed: {error_text}")

                result = await resp.json()
                xbl_token = result["Token"]
                user_hash = result["DisplayClaims"]["xui"][0]["uhs"]

                return xbl_token, user_hash

    async def authenticate_xsts(self, xbl_token: str) -> Tuple[str, str]:
        """
        XSTS 认证
        返回: (xsts_token, user_hash)
        """
        async with aiohttp.ClientSession(timeout=_MS_TIMEOUT) as session:
            payload = {
                "Properties": {"SandboxId": "RETAIL", "UserTokens": [xbl_token]},
                "RelyingParty": "rp://api.minecraftservices.com/",
                "TokenType": "JWT",
            }

            async with session.post(
                self.XSTS_AUTH_ENDPOINT,
                json=payload,
                headers={"Content-Type": "application/json"},
            ) as resp:
                if resp.status != 200:
                    result = await resp.json()
                    xerr = result.get("XErr")

                    error_messages = {
                        2148916233: "This Microsoft account doesn't have an Xbox account. Please create one at xbox.com",
                        2148916238: "This account is a child account and needs to be added to a family",
                        2148916235: "Xbox Live is not available in your country/region",
                    }

                    if xerr in error_messages:
                        raise Exception(error_messages[xerr])

                    error_text = await resp.text()
                    raise Exception(f"XSTS authentication failed: {error_text}")

                result = await resp.json()
                xsts_token = result["Token"]
                user_hash = result["DisplayClaims"]["xui"][0]["uhs"]

                return xsts_token, user_hash

    async def authenticate_minecraft(self, user_hash: str, xsts_token: str) -> str:
        """
        Minecraft 认证
        返回: minecraft_access_token
        """
        async with aiohttp.ClientSession(timeout=_MS_TIMEOUT) as session:
            payload = {"identityToken": f"XBL3.0 x={user_hash};{xsts_token}"}

            async with session.post(
                self.MC_LOGIN_ENDPOINT,
                json=payload,
                headers={"Content-Type": "application/json"},
            ) as resp:
                if resp.status != 200:
                    error_text = await resp.text()
                    raise Exception(f"Minecraft authentication failed: {error_text}")

                result = await resp.json()
                return result["access_token"]

    async def check_game_ownership(self, mc_access_token: str) -> bool:
        """检查是否拥有 Minecraft Java 版"""
        async with aiohttp.ClientSession(timeout=_MS_TIMEOUT) as session:
            async with session.get(
                self.MC_ENTITLEMENTS_ENDPOINT,
                headers={"Authorization": f"Bearer {mc_access_token}"},
            ) as resp:
                if resp.status == 200:
                    result = await resp.json()
                    # 检查是否有游戏权限
                    items = result.get("items", [])
                    return len(items) > 0
                return False

    async def get_minecraft_profile(self, mc_access_token: str) -> Optional[Dict]:
        """
        获取 Minecraft 档案
        返回: {
            "id": "uuid_without_hyphens",
            "name": "PlayerName",
            "skins": [...],
            "capes": [...]
        }
        """
        async with aiohttp.ClientSession(timeout=_MS_TIMEOUT) as session:
            async with session.get(
                self.MC_PROFILE_ENDPOINT,
                headers={"Authorization": f"Bearer {mc_access_token}"},
            ) as resp:
                if resp.status == 404:
                    # 用户未创建游戏档案
                    return None

                if resp.status != 200:
                    error_text = await resp.text()
                    raise Exception(f"Failed to get profile: {error_text}")

                return await resp.json()

    async def complete_auth_flow(self, ms_access_token: str) -> Dict:
        """
        完成完整的认证流程
        返回: {
            "mc_access_token": "...",
            "profile": {...},
            "has_game": True/False
        }
        """
        # 1. Xbox Live 认证
        xbl_token, user_hash = await self.authenticate_xbl(ms_access_token)

        # 2. XSTS 认证
        xsts_token, user_hash = await self.authenticate_xsts(xbl_token)

        # 3. Minecraft 认证
        mc_access_token = await self.authenticate_minecraft(user_hash, xsts_token)

        # 4. 检查游戏所有权
        has_game = await self.check_game_ownership(mc_access_token)

        # 5. 获取档案
        profile = await self.get_minecraft_profile(mc_access_token)

        return {
            "mc_access_token": mc_access_token,
            "profile": profile,
            "has_game": has_game,
        }


class MicrosoftBackend:
    """微软登录编排：读取配置、构造认证服务、导入正版角色。"""

    def __init__(self, db, config, texture_storage):
        self.db = db
        self.config = config
        self.texture_storage = texture_storage

    async def _build_service(self) -> MicrosoftAuthService:
        client_id = await self.db.setting.get("microsoft_client_id")
        client_secret = await self.db.setting.get("microsoft_client_secret")
        if not client_id:
            raise HTTPException(
                status_code=500,
                detail="Microsoft OAuth not configured. Please contact administrator.",
            )
        if not client_secret:
            raise HTTPException(
                status_code=500,
                detail="Microsoft OAuth client_secret not configured. Please contact administrator.",
            )
        default_redirect = (
            self.config.get("server.site_url", "http://localhost:8000").rstrip("/")
            + "/microsoft/callback"
        )
        redirect_uri = await self.db.setting.get("microsoft_redirect_uri", default_redirect)
        return MicrosoftAuthService(client_id, client_secret, redirect_uri)

    async def get_authorization_url(self, state: str) -> str:
        service = await self._build_service()
        return service.get_authorization_url(state)

    async def complete_auth_flow(self, code: str) -> Dict:
        service = await self._build_service()
        token_data = await service.exchange_code_for_token(code)
        return await service.complete_auth_flow(token_data["access_token"])

    async def import_profile(
        self,
        user_id: str,
        profile_id: str,
        profile_name: str,
        skin_url: Optional[str],
        skin_variant: str,
        cape_url: Optional[str],
    ) -> Dict:
        if await self.db.user.get_profile_by_id(profile_id):
            raise HTTPException(status_code=400, detail="该角色 UUID 已在本地存在，无法导入")

        async def _name_exists(n: str) -> bool:
            return await self.db.user.get_profile_by_name(n) is not None

        try:
            profile_name = await generate_unique_profile_name(profile_name, _name_exists)
        except ValueError as e:
            raise HTTPException(status_code=400, detail=str(e))

        skin_hash = await self._import_texture(
            user_id, skin_url, "skin", f"From Microsoft account - {profile_name}"
        )
        cape_hash = await self._import_texture(
            user_id, cape_url, "cape", f"From Microsoft account - {profile_name}"
        )

        texture_model = normalize_texture_model(skin_variant)
        await self.db.user.create_profile(
            PlayerProfile(profile_id, user_id, profile_name, texture_model)
        )
        if skin_hash:
            await self.db.user.update_profile_skin(profile_id, skin_hash)
        if cape_hash:
            await self.db.user.update_profile_cape(profile_id, cape_hash)

        return {
            "ok": True,
            "profile": {
                "id": profile_id,
                "name": profile_name,
                "model": texture_model,
                "skin_hash": skin_hash,
                "cape_hash": cape_hash,
            },
        }

    async def _import_texture(
        self, user_id: str, url: Optional[str], texture_type: str, note: str
    ) -> Optional[str]:
        if not url:
            return None
        try:
            max_bytes = await resolve_max_texture_bytes(self.db)
            data = await download_texture(url, max_bytes=max_bytes)
            await assert_texture_size(self.db, data)
            texture_hash, created = await self.texture_storage.process_and_save_async_tracked(
                data, texture_type
            )
            try:
                await self.db.texture.add_to_library(user_id, texture_hash, texture_type, note)
            except Exception:
                if created:
                    try:
                        if not await self.db.texture.exists(texture_hash, texture_type):
                            await asyncio.to_thread(self.texture_storage.delete_file, texture_hash)
                    except Exception:
                        pass
                raise
            return texture_hash
        except Exception as e:
            logger.warning("Failed to download %s: %s", texture_type, e)
            return None