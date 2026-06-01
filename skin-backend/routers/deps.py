"""路由共享依赖：JWT 鉴权 + 账号状态校验"""

from fastapi import Request, HTTPException, Depends

from utils.jwt_utils import decode_access_token


# 由 routes_reference 在应用初始化时通过 bind_db(db) 注入。
# get_current_user 需要查库以使删号/降权对站点 API 即时生效（JWT 本身无状态）。
_db = None


def bind_db(db) -> None:
    """注入数据库实例，供 get_current_user 查询用户实时状态。"""
    global _db
    _db = db


async def get_current_user(request: Request) -> dict:
    """从 cookie 解析 access token 并校验账号实时状态，返回 payload。

    在解析 token 之外，额外查库校验：
    - 用户仍存在（删号后旧 token 立即失效）；
    - 以数据库的 is_admin 覆盖 token 中的旧值（降权/提权立即生效）。

    注意：封禁（banned_until）**不**在此拦截——封禁仅限制通过 Yggdrasil
    登录游戏（见 yggdrasil_backend），被封禁用户仍可正常访问主站。
    """
    token = request.cookies.get("access_token")
    if not token:
        raise HTTPException(status_code=401, detail="not authenticated")
    payload = decode_access_token(token)
    if not payload:
        raise HTTPException(status_code=401, detail="invalid or expired token")

    user_id = payload.get("sub")
    if not user_id:
        raise HTTPException(status_code=401, detail="invalid token payload")

    # 未绑定 db 属于部署错误（应在启动时 bind_db），明确报错而非静默放行。
    if _db is None:
        raise HTTPException(status_code=500, detail="auth backend not initialized")

    user = await _db.user.get_by_id(user_id)
    if not user:
        raise HTTPException(status_code=401, detail="user not found")

    # 以库内实时 is_admin 为准，避免旧 token 携带过期权限
    payload["is_admin"] = bool(user.is_admin)
    return payload


def admin_required(payload: dict = Depends(get_current_user)) -> dict:
    """在 get_current_user 基础上要求管理员权限。"""
    if not payload.get("is_admin"):
        raise HTTPException(status_code=403, detail="admin required")
    return payload
