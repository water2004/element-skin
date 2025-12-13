"""
速率限制中间件
防止暴力破解和 API 滥用
配置从数据库动态读取
"""

import time
from collections import defaultdict
from typing import Dict, Tuple
from fastapi import Request, HTTPException
import aiosqlite


class RateLimiter:
    def __init__(self, db_path: str = "yggdrasil.db"):
        # 存储格式: {(ip, endpoint): [(timestamp, count)]}
        self._attempts: Dict[Tuple[str, str], list] = defaultdict(list)
        self.db_path = db_path

    async def _get_setting(self, key: str, default: str) -> str:
        """从数据库获取设置"""
        try:
            async with aiosqlite.connect(self.db_path) as conn:
                cur = await conn.execute(
                    "SELECT value FROM settings WHERE key=?", (key,)
                )
                row = await cur.fetchone()
                return row[0] if row else default
        except Exception:
            return default

    async def is_enabled(self) -> bool:
        """检查速率限制是否启用"""
        enabled = await self._get_setting("rate_limit_enabled", "true")
        return enabled.lower() == "true"

    def _clean_old_attempts(self, ip: str, endpoint: str, window_seconds: int):
        """清理过期的请求记录"""
        current_time = time.time()
        key = (ip, endpoint)
        self._attempts[key] = [
            (ts, count)
            for ts, count in self._attempts[key]
            if current_time - ts < window_seconds
        ]

    async def check_auth_limit(self, ip: str, endpoint: str) -> bool:
        """
        检查认证接口速率限制
        返回 True 表示允许，False 表示超限
        """
        if not await self.is_enabled():
            return True

        max_attempts = int(await self._get_setting("rate_limit_auth_attempts", "5"))
        window_minutes = int(await self._get_setting("rate_limit_auth_window", "15"))
        window_seconds = window_minutes * 60

        self._clean_old_attempts(ip, endpoint, window_seconds)

        key = (ip, endpoint)
        current_attempts = sum(count for _, count in self._attempts[key])

        if current_attempts >= max_attempts:
            return False

        # 记录本次尝试
        self._attempts[key].append((time.time(), 1))
        return True

    async def check_general_limit(self, ip: str, endpoint: str) -> bool:
        """
        检查通用接口速率限制
        返回 True 表示允许，False 表示超限
        """
        if not await self.is_enabled():
            return True

        max_requests = 100  # 通用限制可以保持固定
        window_seconds = 60

        self._clean_old_attempts(ip, endpoint, window_seconds)

        key = (ip, endpoint)
        current_requests = sum(count for _, count in self._attempts[key])

        if current_requests >= max_requests:
            return False

        # 记录本次请求
        self._attempts[key].append((time.time(), 1))
        return True

    def reset(self, ip: str, endpoint: str):
        """重置指定 IP 和端点的限制（登录成功后调用）"""
        key = (ip, endpoint)
        if key in self._attempts:
            del self._attempts[key]


# 全局速率限制器
rate_limiter = RateLimiter()


async def check_rate_limit(request: Request, is_auth_endpoint: bool = False):
    """
    速率限制检查函数
    用于路由装饰器或依赖注入
    """
    if not await rate_limiter.is_enabled():
        return

    ip = request.client.host
    endpoint = request.url.path

    if is_auth_endpoint:
        if not await rate_limiter.check_auth_limit(ip, endpoint):
            raise HTTPException(
                status_code=429, detail=f"Too many attempts. Please try again later."
            )
    else:
        if not await rate_limiter.check_general_limit(ip, endpoint):
            raise HTTPException(
                status_code=429, detail=f"Rate limit exceeded. Please slow down."
            )
