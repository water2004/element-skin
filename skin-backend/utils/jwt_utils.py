"""JWT 令牌工具"""

import jwt
from datetime import datetime, timedelta, timezone
from typing import Dict, Optional
from config_loader import config


JWT_SECRET = config.get("jwt.secret", "dev-secret-default-key-at-least-32-chars-long")
JWT_ALGO = "HS256"


def get_cookie_settings() -> dict:
    """返回 JWT cookie 配置"""
    site_url = config.get("server.site_url", "http://localhost")
    secure = site_url.startswith("https://")
    expire_days = int(config.get("jwt.expire_days", "7"))
    return {
        "key": "jwt",
        "value": "",  # 由调用方设置
        "httponly": True,
        "secure": secure,
        "samesite": "lax",
        "max_age": expire_days * 24 * 3600,
        "path": "/",
    }


def create_jwt_token(user_id: str, is_admin: bool, expire_days: int) -> str:
    """
    创建 JWT 令牌

    Args:
        user_id: 用户 ID
        is_admin: 是否为管理员
        expire_days: 过期天数

    Returns:
        str: JWT 令牌
    """
    payload = {
        "sub": user_id,
        "is_admin": is_admin,
        "exp": datetime.now(timezone.utc) + timedelta(days=expire_days),
    }
    return jwt.encode(payload, JWT_SECRET, algorithm=JWT_ALGO)


def decode_jwt_token(token: str) -> Optional[Dict]:
    """
    解码 JWT 令牌

    Args:
        token: JWT 令牌字符串

    Returns:
        Optional[Dict]: 解码后的 payload，如果无效则返回 None
    """
    try:
        payload = jwt.decode(token, JWT_SECRET, algorithms=[JWT_ALGO])
        return payload
    except jwt.ExpiredSignatureError:
        return None
    except Exception:
        return None
