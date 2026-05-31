"""路由共享依赖：JWT 鉴权"""

from fastapi import Request, HTTPException, Depends

from utils.jwt_utils import decode_jwt_token


async def get_current_user(request: Request) -> dict:
    """从 cookie 中解析 JWT，返回 payload。无效或缺失则抛 401。"""
    token = request.cookies.get("jwt")
    if not token:
        raise HTTPException(status_code=401, detail="not authenticated")
    payload = decode_jwt_token(token)
    if not payload:
        raise HTTPException(status_code=401, detail="invalid or expired token")
    return payload


def admin_required(payload: dict = Depends(get_current_user)) -> dict:
    """在 get_current_user 基础上要求管理员权限。"""
    if not payload.get("is_admin"):
        raise HTTPException(status_code=403, detail="admin required")
    return payload
