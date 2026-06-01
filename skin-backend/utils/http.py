"""HTTP 工具：纹理下载"""

import aiohttp

from utils.url_guard import validate_outbound_url


async def download_texture(url: str) -> bytes:
    """下载皮肤或披风纹理，失败抛 Exception。

    下载前对 URL 做 SSRF 校验（仅 http/https、禁止解析到内网/保留地址），
    并禁止跟随重定向，避免「先返回公网 URL 再 302 跳内网」的绕过。
    """
    await validate_outbound_url(url)
    timeout = aiohttp.ClientTimeout(total=15)
    async with aiohttp.ClientSession(timeout=timeout) as session:
        async with session.get(url, allow_redirects=False) as resp:
            if resp.status == 200:
                return await resp.read()
            raise Exception(f"Failed to download texture from {url}")
