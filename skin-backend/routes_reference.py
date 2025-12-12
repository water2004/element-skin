# 假设上述文件已保存
from fastapi import FastAPI, Request, HTTPException, Depends, status
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
import jwt
from datetime import datetime, timedelta
from contextlib import asynccontextmanager
from fastapi.responses import JSONResponse, Response
from fastapi import UploadFile, File, Form, Header
from fastapi.staticfiles import StaticFiles
from fastapi.middleware.cors import CORSMiddleware
import os
import time
import uuid
from backend import YggdrasilBackend, YggdrasilError
from database import Database
from models import AuthRequest, RefreshRequest, JoinRequest, CryptoUtils

# 初始化
db = Database()
crypto = CryptoUtils("private.pem")
backend = YggdrasilBackend(db, crypto)

# JWT 配置
JWT_SECRET = os.environ.get("JWT_SECRET", "dev-secret")
JWT_ALGO = "HS256"
JWT_EXPIRE_DAYS = int(os.environ.get("JWT_EXPIRE_DAYS", "7"))
security = HTTPBearer()


@asynccontextmanager
async def lifespan(app: FastAPI):
    # 在应用启动时初始化数据库
    await db.init()
    try:
        yield
    finally:
        # Database has no explicit close method; cleanup can be added here if needed
        pass


app = FastAPI(lifespan=lifespan)


# 全局请求/响应日志中间件：打印每个收到的请求摘要与响应状态，便于诊断为什么游戏没有请求到 PNG
@app.middleware("http")
async def log_all_requests(request: Request, call_next):
    try:
        body = await request.body()
    except Exception:
        body = b""

    # 打印基础请求信息（限制 body 输出长度）
    print("--- HTTP REQUEST ---")
    print(f"{request.method} {request.url}")
    # 打印部分头部信息（避免泄露大块敏感数据）
    hdrs = {k: v for k, v in request.headers.items()}
    print("Headers:", {k: hdrs[k] for k in list(hdrs)[:20]})
    if body:
        try:
            preview = body.decode("utf-8", errors="replace")
        except Exception:
            preview = str(body[:200])
        print("Body preview:", preview[:1000])
    else:
        print("Body preview: <empty>")

    # Recreate request stream for downstream
    async def receive():
        return {"type": "http.request", "body": body}

    response = await call_next(Request(request.scope, receive))

    print(
        f"--- HTTP RESPONSE --- {response.status_code} for {request.method} {request.url}\n"
    )
    return response


# 允许跨域开发请求（开发环境使用），生产请按需限制
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# 挂载静态材质目录
textures_path = os.path.join(os.path.dirname(__file__), "textures")
os.makedirs(textures_path, exist_ok=True)
# 挂载为 /static/textures，避免与 /textures/upload 路由冲突（StaticFiles 会拦截相同前缀的请求）
app.mount("/static/textures", StaticFiles(directory=textures_path), name="textures")


# 异常处理器
@app.exception_handler(YggdrasilError)
async def ygg_exception_handler(request: Request, exc: YggdrasilError):
    return JSONResponse(
        status_code=exc.status_code,
        content={"error": exc.error, "errorMessage": exc.message},
    )


# --- 路由 ---


@app.post("/authserver/authenticate")
async def authenticate(req: AuthRequest):
    resp = await backend.authenticate(req)
    # 如果请求了用户信息，则签发 JWT（包含 is_admin）
    if resp.get("user"):
        user_id = resp["user"]["id"]
        # 查询是否为管理员
        async with db.get_conn() as conn:
            cur = await conn.execute(
                "SELECT is_admin FROM users WHERE id=?", (user_id,)
            )
            row = await cur.fetchone()
            is_admin = bool(row[0]) if row else False

        payload = {
            "sub": user_id,
            "is_admin": is_admin,
            "exp": datetime.utcnow() + timedelta(days=JWT_EXPIRE_DAYS),
        }
        token = jwt.encode(payload, JWT_SECRET, algorithm=JWT_ALGO)
        resp["token"] = token

    return resp


@app.post("/authserver/refresh")
async def refresh(req: RefreshRequest):
    return await backend.refresh(req)


@app.post("/authserver/validate")
async def validate(req: dict):
    # validate API 若成功返回 204
    await backend.validate(req)
    return Response(status_code=204)


@app.post("/authserver/invalidate")
async def invalidate(req: dict):
    token = req.get("accessToken")
    if token:
        await backend.invalidate(token)
    return Response(status_code=204)


@app.post("/sessionserver/session/minecraft/join")
async def join_server(req: JoinRequest, request: Request):
    # 获取真实 IP
    ip = request.client.host
    await backend.join_server(req, ip)
    return Response(status_code=204)


@app.get("/sessionserver/session/minecraft/hasJoined")
async def has_joined(request: Request, username: str, serverId: str, ip: str = None):
    profile = await backend.has_joined(
        username, serverId, ip, base_url=str(request.base_url)
    )
    if profile:
        return profile
    else:
        return Response(status_code=204)


@app.get("/sessionserver/session/minecraft/profile/{uuid}")
async def get_profile(request: Request, uuid: str, unsigned: bool = True):
    profile = await backend.get_profile(uuid, unsigned, base_url=str(request.base_url))
    if profile:
        return profile
    return Response(status_code=204)


async def get_current_user(creds: HTTPAuthorizationCredentials = Depends(security)):
    token = creds.credentials
    try:
        payload = jwt.decode(token, JWT_SECRET, algorithms=[JWT_ALGO])
        return payload
    except jwt.ExpiredSignatureError:
        raise HTTPException(status_code=401, detail="token expired")
    except Exception:
        raise HTTPException(status_code=401, detail="invalid token")


def admin_required(payload: dict = Depends(get_current_user)):
    if not payload.get("is_admin"):
        raise HTTPException(status_code=403, detail="admin required")
    return payload


@app.get("/me")
async def me(payload: dict = Depends(get_current_user)):
    user_id = payload.get("sub")
    async with db.get_conn() as conn:
        cur = await conn.execute(
            "SELECT id, email, preferred_language, is_admin FROM users WHERE id=?",
            (user_id,),
        )
        row = await cur.fetchone()
        if not row:
            raise HTTPException(status_code=404, detail="user not found")

        cur2 = await conn.execute(
            "SELECT id, name, texture_model, skin_hash, cape_hash FROM profiles WHERE user_id=?",
            (user_id,),
        )
        profiles = await cur2.fetchall()
        profiles_list = [
            {
                "id": p[0],
                "name": p[1],
                "model": p[2],
                "skin_hash": p[3],
                "cape_hash": p[4],
            }
            for p in profiles
        ]

        return {
            "id": row[0],
            "email": row[1],
            "lang": row[2],
            "is_admin": bool(row[3]),
            "profiles": profiles_list,
        }


# 注册接口（支持邀请码，若 settings 中设置 invite_required=1 则必须提供有效邀请码）
@app.post("/register")
async def register(req: dict):
    email = req.get("email")
    password = req.get("password")
    invite = req.get("invite")
    if not email or not password:
        raise HTTPException(status_code=400, detail="email and password required")

    async with db.get_conn() as conn:
        # 检查是否需要邀请码
        cur = await conn.execute(
            "SELECT value FROM settings WHERE key=?", ("invite_required",)
        )
        row = await cur.fetchone()
        invite_required = row and row[0] == "1"

        if invite_required:
            if not invite:
                raise HTTPException(status_code=400, detail="invite required")
            c = await conn.execute(
                "SELECT code, used_by FROM invites WHERE code=?", (invite,)
            )
            crow = await c.fetchone()
            if not crow:
                raise HTTPException(status_code=400, detail="invalid invite")
            if crow[1]:
                raise HTTPException(status_code=400, detail="invite already used")

        # 创建用户
        uid = uuid.uuid4().hex
        await conn.execute(
            "INSERT INTO users (id, email, password) VALUES (?, ?, ?)",
            (uid, email, password),
        )
        # 创建默认 profile
        pid = uuid.uuid4().hex
        await conn.execute(
            "INSERT INTO profiles (id, user_id, name) VALUES (?, ?, ?)",
            (pid, uid, email.split("@")[0]),
        )

        if invite_required and invite:
            await conn.execute(
                "UPDATE invites SET used_by=? WHERE code=?", (uid, invite)
            )

        await conn.commit()

    return {"id": uid}


# 管理接口：简单的 invite 与 用户列表示例
@app.get("/admin/users")
async def admin_list_users():
    raise HTTPException(
        status_code=401, detail="use paginated endpoint /admin/users/list"
    )


@app.post("/admin/invite/generate")
async def admin_generate_invite():
    code = uuid.uuid4().hex[:8]
    created_at = int(time.time() * 1000)
    async with db.get_conn() as conn:
        await conn.execute(
            "INSERT INTO invites (code, created_by, created_at) VALUES (?, ?, ?)",
            (code, None, created_at),
        )
        await conn.commit()
    return {"code": code}


@app.get("/admin/invites")
async def admin_list_invites():
    async with db.get_conn() as conn:
        cur = await conn.execute(
            "SELECT code, created_by, used_by, created_at FROM invites"
        )
        rows = await cur.fetchall()
        return [
            {"code": r[0], "created_by": r[1], "used_by": r[2], "created_at": r[3]}
            for r in rows
        ]


@app.get("/admin/users/list")
async def admin_users_list(
    page: int = 1, per_page: int = 20, _=Depends(admin_required)
):
    offset = (page - 1) * per_page
    async with db.get_conn() as conn:
        cur = await conn.execute(
            "SELECT id, email, preferred_language, is_admin FROM users LIMIT ? OFFSET ?",
            (per_page, offset),
        )
        rows = await cur.fetchall()
        cur2 = await conn.execute("SELECT COUNT(1) FROM users")
        total = (await cur2.fetchone())[0]
        return {
            "items": [
                {"id": r[0], "email": r[1], "lang": r[2], "is_admin": bool(r[3])}
                for r in rows
            ],
            "total": total,
        }


@app.post("/admin/users/reset-password")
async def admin_reset_password(payload: dict, _=Depends(admin_required)):
    user_id = payload.get("user_id")
    new_password = payload.get("new_password")
    if not user_id or not new_password:
        raise HTTPException(status_code=400, detail="user_id and new_password required")
    async with db.get_conn() as conn:
        await conn.execute(
            "UPDATE users SET password=? WHERE id=?", (new_password, user_id)
        )
        await conn.commit()
    return {"ok": True}


@app.post("/admin/users/delete")
async def admin_delete_user(payload: dict, _=Depends(admin_required)):
    user_id = payload.get("user_id")
    if not user_id:
        raise HTTPException(status_code=400, detail="user_id required")
    async with db.get_conn() as conn:
        await conn.execute("DELETE FROM users WHERE id=?", (user_id,))
        await conn.execute("DELETE FROM profiles WHERE user_id=?", (user_id,))
        await conn.commit()
    return {"ok": True}


# 材质上传路由（使用 multipart/form-data）
@app.post("/textures/upload")
async def textures_upload(
    file: UploadFile = File(...),
    accessToken: str = Form(None),
    uuid: str = Form(...),
    texture_type: str = Form(...),
    model: str = Form(""),
    authorization: str = Header(None),
):
    # 兼容两种传递 access token 的方式：1) Authorization: Bearer <token> 2) form field accessToken
    content = await file.read()
    token = accessToken
    if authorization and authorization.startswith("Bearer "):
        token = authorization.split(" ", 1)[1]
    if not token:
        raise HTTPException(status_code=401, detail="access token required")

    await backend.upload_texture(token, uuid, texture_type, content, model)
    return {"ok": True}


@app.put("/api/user/profile/{uuid}/{textureType}")
async def api_put_profile(
    uuid: str,
    textureType: str,
    file: UploadFile = File(...),
    model: str = Form(""),
    authorization: str = Header(None),
):
    # 接受 Authorization: Bearer <accessToken>
    token = None
    if authorization and authorization.startswith("Bearer "):
        token = authorization.split(" ", 1)[1]
    if not token:
        raise HTTPException(status_code=401, detail="access token required")
    content = await file.read()
    await backend.upload_texture(token, uuid, textureType, content, model)
    return Response(status_code=204)


@app.delete("/api/user/profile/{uuid}/{textureType}")
async def api_delete_profile(
    uuid: str, textureType: str, authorization: str = Header(None)
):
    token = None
    if authorization and authorization.startswith("Bearer "):
        token = authorization.split(" ", 1)[1]
    if not token:
        raise HTTPException(status_code=401, detail="access token required")
    await backend.delete_texture(token, uuid, textureType)
    return Response(status_code=204)


# 扩展 API: 元数据
@app.get("/")
async def get_meta(request: Request):
    with open("public.pem", "r") as f:
        pub_key = f.read()

    # 动态返回 skinDomains，以包含当前请求的主机，避免客户端因域名白名单不匹配而拒绝材质
    host = request.url.hostname or ""
    skin_domains = [host] if host else ["example.com"]
    # 若为多级域名，则同时添加根域名的通配符（例如 sub.example.com -> .example.com）
    if host and "." in host:
        parts = host.split(".", 1)
        if len(parts) == 2:
            skin_domains.append("." + parts[1])

    return {
        "meta": {
            "serverName": "My Yggdrasil Server",
            "implementationName": "Python-UV-Yggdrasil",
            "implementationVersion": "1.0.0",
        },
        "skinDomains": skin_domains,
        "signaturePublickey": pub_key,
    }


if __name__ == "__main__":
    import uvicorn

    # "routes_reference:app" 对应 文件名:变量名
    # host="0.0.0.0" 表示允许局域网访问，127.0.0.1 仅限本机
    # port=8000 是端口号
    uvicorn.run("routes_reference:app", host="127.0.0.1", port=8000, reload=True)
