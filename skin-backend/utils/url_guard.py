"""出站 URL 安全校验（SSRF 防护）。

用于所有以用户可控 URL 发起出站请求的路径（远程 Yggdrasil 导入、
Microsoft 导入材质下载等）。基线策略：

- 仅允许 http / https。
- 解析主机名得到的所有 IP 必须是公网地址；任一落入私有/回环/链路本地/
  保留段即拒绝（挡住 127.0.0.1、10/172/192.168 内网、169.254.169.254 云元数据等）。

注意：这是基础防护，无法消除 DNS rebinding（解析后到连接之间地址可能变化）。
对本项目的部署形态（容器经 Nginx 出网）已足够；如需更强保证，应在网络层
限制出站到可信目标。
"""

import asyncio
import ipaddress
import socket
from urllib.parse import urlsplit


class UnsafeURLError(ValueError):
    """URL 未通过出站安全校验。继承 ValueError，便于沿用现有 except ValueError 处理。"""


_ALLOWED_SCHEMES = {"http", "https"}


def _ip_is_blocked(ip: ipaddress._BaseAddress) -> bool:
    """判断单个 IP 是否属于禁止访问的范围。"""
    return (
        ip.is_private
        or ip.is_loopback
        or ip.is_link_local
        or ip.is_reserved
        or ip.is_multicast
        or ip.is_unspecified
    )


async def validate_outbound_url(url: str) -> None:
    """校验出站 URL；不安全则抛 UnsafeURLError。

    通过 getaddrinfo 解析主机的全部地址并逐一检查，避免"一个名字解析出多个
    IP，只检查第一个"的绕过。getaddrinfo 是阻塞调用，放进线程池执行。
    """
    if not url or not isinstance(url, str):
        raise UnsafeURLError("Empty or invalid URL")

    parts = urlsplit(url)
    if parts.scheme.lower() not in _ALLOWED_SCHEMES:
        raise UnsafeURLError(f"Disallowed URL scheme: {parts.scheme!r}")

    host = parts.hostname
    if not host:
        raise UnsafeURLError("URL has no host")

    # 主机本身就是 IP 字面量时，直接判定（getaddrinfo 也能处理，但显式更快更清晰）
    try:
        ip = ipaddress.ip_address(host)
        if _ip_is_blocked(ip):
            raise UnsafeURLError(f"URL resolves to a blocked address: {host}")
        return
    except ValueError:
        pass  # 不是 IP 字面量，按域名解析

    loop = asyncio.get_running_loop()
    try:
        infos = await loop.getaddrinfo(host, parts.port or None, proto=socket.IPPROTO_TCP)
    except socket.gaierror as e:
        raise UnsafeURLError(f"Failed to resolve host {host!r}: {e}") from e

    if not infos:
        raise UnsafeURLError(f"Host {host!r} did not resolve to any address")

    for info in infos:
        sockaddr = info[4]
        ip_str = sockaddr[0]
        try:
            ip = ipaddress.ip_address(ip_str)
        except ValueError:
            raise UnsafeURLError(f"Host {host!r} resolved to invalid address {ip_str!r}")
        if _ip_is_blocked(ip):
            raise UnsafeURLError(
                f"Host {host!r} resolves to a blocked address: {ip_str}"
            )
