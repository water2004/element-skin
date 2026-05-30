"""HTTP 工具：纹理下载"""

import aiohttp


async def download_texture(url: str) -> bytes:
    """下载皮肤或披风纹理，失败抛 Exception。"""
    timeout = aiohttp.ClientTimeout(total=15)
    async with aiohttp.ClientSession(timeout=timeout) as session:
        async with session.get(url) as resp:
            if resp.status == 200:
                return await resp.read()
            raise Exception(f"Failed to download texture from {url}")
