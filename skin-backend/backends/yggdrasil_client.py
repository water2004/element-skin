import aiohttp
import asyncio
import json
import base64
from typing import Optional, Dict, List, Any, Tuple

class YggdrasilClient:
    """Yggdrasil 协议客户端，用于从远程皮肤站获取信息"""

    def __init__(self, api_base_url: str):
        # 确保 api_base_url 以 / 结尾
        self.api_base_url = api_base_url.rstrip("/") + "/"
        self.auth_url = self.api_base_url + "authserver/authenticate"
        self.profile_url = self.api_base_url + "sessionserver/session/minecraft/profile/"

    async def authenticate(self, username: str, password: str) -> Dict[str, Any]:
        """
        在远程皮肤站进行身份验证
        返回: { "accessToken": "...", "availableProfiles": [...], "user": {...} }
        """
        payload = {
            "username": username,
            "password": password,
            "agent": {
                "name": "Minecraft",
                "version": 1
            },
            "requestUser": True
        }

        async with aiohttp.ClientSession() as session:
            async with session.post(self.auth_url, json=payload, timeout=10) as resp:
                if resp.status == 200:
                    return await resp.json()
                else:
                    try:
                        error_data = await resp.json()
                        error_msg = error_data.get("errorMessage", f"HTTP {resp.status}")
                    except:
                        error_msg = f"HTTP {resp.status}"
                    raise Exception(f"Authentication failed: {error_msg}")

    async def get_profile_with_textures(self, uuid: str) -> Dict[str, Any]:
        """
        获取带有材质信息的角色档案
        """
        url = self.profile_url + uuid.replace("-", "")
        async with aiohttp.ClientSession() as session:
            async with session.get(url, timeout=10) as resp:
                if resp.status == 200:
                    data = await resp.json()
                    return self._parse_textures(data)
                elif resp.status == 204:
                    raise Exception("Profile not found")
                else:
                    raise Exception(f"Failed to fetch profile: HTTP {resp.status}")

    def _parse_textures(self, profile_data: Dict[str, Any]) -> Dict[str, Any]:
        """
        从角色属性中解析材质信息
        """
        properties = profile_data.get("properties", [])
        textures_base64 = None
        for prop in properties:
            if prop.get("name") == "textures":
                textures_base64 = prop.get("value")
                break
        
        if not textures_base64:
            return {
                "id": profile_data.get("id"),
                "name": profile_data.get("name"),
                "skins": [],
                "capes": []
            }

        try:
            textures_json = json.loads(base64.b64decode(textures_base64).decode("utf-8"))
            textures = textures_json.get("textures", {})
            
            skins = []
            if "SKIN" in textures:
                skin_data = textures["SKIN"]
                skins.append({
                    "url": skin_data.get("url"),
                    "variant": skin_data.get("metadata", {}).get("model", "classic")
                })
            
            capes = []
            if "CAPE" in textures:
                capes.append({
                    "url": textures["CAPE"].get("url")
                })
                
            return {
                "id": profile_data.get("id"),
                "name": profile_data.get("name"),
                "skins": skins,
                "capes": capes
            }
        except Exception as e:
            print(f"Error parsing textures: {e}")
            return {
                "id": profile_data.get("id"),
                "name": profile_data.get("name"),
                "skins": [],
                "capes": []
            }

async def download_texture(url: str) -> bytes:
    """下载皮肤或披风纹理"""
    timeout = aiohttp.ClientTimeout(total=15)
    async with aiohttp.ClientSession(timeout=timeout) as session:
        async with session.get(url) as resp:
            if resp.status == 200:
                return await resp.read()
            raise Exception(f"Failed to download texture from {url}")
