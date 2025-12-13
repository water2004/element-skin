import time
import uuid
import json
import base64
import os
from typing import Dict, Any, Optional
from PIL import Image
from io import BytesIO

from models import CryptoUtils, AuthRequest, RefreshRequest
from database import Database


# 异常定义
class YggdrasilError(Exception):
    def __init__(self, error: str, message: str, status_code: int = 400):
        self.error = error
        self.message = message
        self.status_code = status_code


class ForbiddenOperationException(YggdrasilError):
    def __init__(self, message: str):
        super().__init__("ForbiddenOperationException", message, 403)


class IllegalArgumentException(YggdrasilError):
    def __init__(self, message: str):
        super().__init__("IllegalArgumentException", message, 400)


class YggdrasilBackend:
    def __init__(self, db: Database, crypto: CryptoUtils):
        self.db = db
        self.crypto = crypto
        self.TOKEN_TTL = 15 * 24 * 3600 * 1000  # 15天 (毫秒)
        self.SESSION_TTL = 30 * 1000  # 30秒 (用于join验证)

    # =========================
    # 辅助方法
    # =========================

    async def _cleanup_tokens(self, user_id: str):
        # 删除过期令牌并仅保留最近 5 个令牌
        cutoff = int(time.time() * 1000) - self.TOKEN_TTL
        async with self.db.get_conn() as conn:
            # 移除过期令牌
            await conn.execute(
                "DELETE FROM tokens WHERE user_id=? AND created_at < ?",
                (user_id, cutoff),
            )

            # 查询该用户按时间倒序的所有令牌
            cur = await conn.execute(
                "SELECT access_token FROM tokens WHERE user_id=? ORDER BY created_at DESC",
                (user_id,),
            )
            rows = await cur.fetchall()

            # 保留最新 5 个，其余删除
            surplus = rows[5:]
            if surplus:
                await conn.executemany(
                    "DELETE FROM tokens WHERE access_token=?",
                    [(r[0],) for r in surplus],
                )

            await conn.commit()

    async def _get_profile_json(
        self, profile_row, sign: bool = False, base_url: str = None
    ) -> Dict:
        """构建角色 JSON，包含 textures 和签名"""
        pid, uid, name, model, skin_hash, cape_hash = profile_row

        profile_data = {"id": pid, "name": name, "properties": []}

        # 构建 Textures 属性
        textures_payload = {
            "timestamp": int(time.time() * 1000),
            "profileId": pid,
            "profileName": name,
            "textures": {},
        }

        # 材质基础 URL：默认使用相对路径 `/static/textures/`。
        # 若提供了 base_url（例如 Request.base_url），则生成绝对 URL：{base_url.rstrip('/')}/static/textures/{hash}.png
        base_texture_url = "/static/textures/"
        if base_url:
            # ensure no trailing slash on base_url
            base_texture_url = base_url.rstrip("/") + "/static/textures/"

        if skin_hash:
            textures_payload["textures"]["SKIN"] = {
                "url": base_texture_url + skin_hash + ".png"
            }
            if model == "slim":
                textures_payload["textures"]["SKIN"]["metadata"] = {"model": "slim"}

        if cape_hash:
            textures_payload["textures"]["CAPE"] = {
                "url": base_texture_url + cape_hash + ".png"
            }

        # 序列化 Textures
        textures_json = json.dumps(textures_payload)
        textures_base64 = base64.b64encode(textures_json.encode("utf-8")).decode(
            "utf-8"
        )

        prop = {"name": "textures", "value": textures_base64}
        if sign:
            prop["signature"] = self.crypto.sign_data(textures_base64)

        profile_data["properties"].append(prop)

        # 添加 uploadableTextures 扩展属性
        profile_data["properties"].append(
            {"name": "uploadableTextures", "value": "skin,cape"}
        )

        return profile_data

    # =========================
    # 用户/认证部分 API 实现
    # =========================

    async def authenticate(self, req: AuthRequest) -> Dict:
        import bcrypt

        async with self.db.get_conn() as conn:
            # 1. 查找用户
            cursor = await conn.execute(
                "SELECT id, email, preferred_language, password FROM users WHERE email = ?",
                (req.username,),
            )
            user = await cursor.fetchone()

            # 验证密码
            if user and len(user) >= 4:
                user_id, email, lang, password_hash = user
                # 检查是否是旧的明文密码（向后兼容）
                if password_hash.startswith("$2"):
                    # bcrypt 哈希
                    if not bcrypt.checkpw(
                        req.password.encode("utf-8"), password_hash.encode("utf-8")
                    ):
                        user = None
                else:
                    # 旧的明文密码，验证后升级为哈希
                    if password_hash == req.password:
                        # 升级为 bcrypt
                        new_hash = bcrypt.hashpw(
                            req.password.encode("utf-8"), bcrypt.gensalt()
                        ).decode("utf-8")
                        await conn.execute(
                            "UPDATE users SET password=? WHERE id=?",
                            (new_hash, user_id),
                        )
                        await conn.commit()
                    else:
                        user = None

            if not user:
                # 尝试角色名登录
                cursor = await conn.execute(
                    "SELECT u.id, u.email, u.preferred_language, u.password, p.id FROM users u "
                    "JOIN profiles p ON p.user_id = u.id WHERE p.name = ?",
                    (req.username,),
                )
                user_via_profile = await cursor.fetchone()
                if user_via_profile and len(user_via_profile) >= 4:
                    user_id, email, lang, password_hash, _ = user_via_profile
                    if password_hash.startswith("$2"):
                        if not bcrypt.checkpw(
                            req.password.encode("utf-8"), password_hash.encode("utf-8")
                        ):
                            user_via_profile = None
                    else:
                        if password_hash != req.password:
                            user_via_profile = None
                        else:
                            # 升级密码
                            new_hash = bcrypt.hashpw(
                                req.password.encode("utf-8"), bcrypt.gensalt()
                            ).decode("utf-8")
                            await conn.execute(
                                "UPDATE users SET password=? WHERE id=?",
                                (new_hash, user_id),
                            )
                            await conn.commit()

                if not user_via_profile:
                    raise ForbiddenOperationException(
                        "Invalid credentials. Invalid username or password."
                    )
                user = user_via_profile[:3]
            else:
                user = user[:3]

            user_id, email, lang = user

            # 2. 生成令牌
            access_token = uuid.uuid4().hex
            client_token = req.clientToken if req.clientToken else uuid.uuid4().hex

            # 3. 获取角色列表
            cursor = await conn.execute(
                "SELECT id, name FROM profiles WHERE user_id = ?", (user_id,)
            )
            profiles = await cursor.fetchall()
            avail_profiles = [{"id": p[0], "name": p[1]} for p in profiles]

            # 4. 确定选中的角色
            selected_profile = None
            # 如果只有一个角色，或者通过角色名登录，自动选择
            # 这里简化逻辑：如果只有一个角色就自动选
            if len(avail_profiles) == 1:
                selected_profile = avail_profiles[0]

            pid_to_bind = selected_profile["id"] if selected_profile else None

            # 5. 存入数据库
            created_at = int(time.time() * 1000)
            await conn.execute(
                "INSERT INTO tokens (access_token, client_token, user_id, profile_id, created_at) VALUES (?, ?, ?, ?, ?)",
                (access_token, client_token, user_id, pid_to_bind, created_at),
            )
            await conn.commit()

            # 清理旧令牌，避免无限膨胀
            await self._cleanup_tokens(user_id)

            resp = {
                "accessToken": access_token,
                "clientToken": client_token,
                "availableProfiles": avail_profiles,
            }
            if selected_profile:
                resp["selectedProfile"] = selected_profile

            if req.requestUser:
                resp["user"] = {
                    "id": user_id,
                    "properties": [{"name": "preferredLanguage", "value": lang}],
                }
            return resp

    async def refresh(self, req: RefreshRequest) -> Dict:
        async with self.db.get_conn() as conn:
            # 1. 查找旧令牌
            cursor = await conn.execute(
                "SELECT client_token, user_id, profile_id FROM tokens WHERE access_token = ?",
                (req.accessToken,),
            )
            token_data = await cursor.fetchone()

            if not token_data:
                raise ForbiddenOperationException("Invalid token.")

            stored_client_token, user_id, stored_profile_id = token_data

            # 校验 clientToken
            if req.clientToken and req.clientToken != stored_client_token:
                raise ForbiddenOperationException("Invalid token.")

            # 2. 吊销旧令牌
            await conn.execute(
                "DELETE FROM tokens WHERE access_token = ?", (req.accessToken,)
            )

            # 3. 处理角色选择
            new_profile_id = stored_profile_id
            selected_profile_resp = None

            if req.selectedProfile:
                if stored_profile_id:
                    raise IllegalArgumentException(
                        "Access token already has a profile assigned."
                    )
                # 验证该角色属于该用户
                p_check = await conn.execute(
                    "SELECT id, name FROM profiles WHERE id = ? AND user_id = ?",
                    (req.selectedProfile["id"], user_id),
                )
                p_row = await p_check.fetchone()
                if not p_row:
                    raise ForbiddenOperationException("Invalid profile.")
                new_profile_id = p_row[0]
                selected_profile_resp = {"id": p_row[0], "name": p_row[1]}
            elif stored_profile_id:
                # 保持原有角色
                p_check = await conn.execute(
                    "SELECT id, name FROM profiles WHERE id = ?", (stored_profile_id,)
                )
                p_row = await p_check.fetchone()
                if p_row:
                    selected_profile_resp = {"id": p_row[0], "name": p_row[1]}

            # 4. 颁发新令牌
            new_access_token = uuid.uuid4().hex
            created_at = int(time.time() * 1000)
            await conn.execute(
                "INSERT INTO tokens (access_token, client_token, user_id, profile_id, created_at) VALUES (?, ?, ?, ?, ?)",
                (
                    new_access_token,
                    stored_client_token,
                    user_id,
                    new_profile_id,
                    created_at,
                ),
            )
            await conn.commit()

            # 清理旧令牌
            await self._cleanup_tokens(user_id)

            resp = {
                "accessToken": new_access_token,
                "clientToken": stored_client_token,
            }
            if selected_profile_resp:
                resp["selectedProfile"] = selected_profile_resp

            if req.requestUser:
                # 获取用户信息
                u_row = await (
                    await conn.execute(
                        "SELECT preferred_language FROM users WHERE id=?", (user_id,)
                    )
                ).fetchone()
                resp["user"] = {
                    "id": user_id,
                    "properties": (
                        [{"name": "preferredLanguage", "value": u_row[0]}]
                        if u_row
                        else []
                    ),
                }
            return resp

    async def validate(self, req: Dict):
        async with self.db.get_conn() as conn:
            cursor = await conn.execute(
                "SELECT client_token, created_at FROM tokens WHERE access_token = ?",
                (req.get("accessToken"),),
            )
            row = await cursor.fetchone()
            if not row:
                raise ForbiddenOperationException("Invalid token.")

            client_token, created_at = row
            if req.get("clientToken") and req["clientToken"] != client_token:
                raise ForbiddenOperationException("Invalid token.")

            # 检查过期
            if (int(time.time() * 1000) - created_at) > self.TOKEN_TTL:
                # 可以在这里删除过期令牌
                raise ForbiddenOperationException("Invalid token.")

            # 验证通过，无返回值（HTTP 204）
            return

    async def invalidate(self, access_token: str):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "DELETE FROM tokens WHERE access_token = ?", (access_token,)
            )
            await conn.commit()

    # =========================
    # 会话部分 API 实现
    # =========================

    async def join_server(self, req: AuthRequest, ip: str):
        async with self.db.get_conn() as conn:
            # 1. 验证 Token
            cursor = await conn.execute(
                "SELECT profile_id FROM tokens WHERE access_token = ?",
                (req.accessToken,),
            )
            row = await cursor.fetchone()
            if not row:
                raise ForbiddenOperationException("Invalid token.")

            profile_id = row[0]
            if profile_id != req.selectedProfile:
                raise ForbiddenOperationException("Invalid token.")

            # 2. 记录 Session (ServerId)
            # 应该先清理旧的同名 serverId
            await conn.execute(
                "DELETE FROM sessions WHERE server_id = ?", (req.serverId,)
            )
            await conn.execute(
                "INSERT INTO sessions (server_id, access_token, ip, created_at) VALUES (?, ?, ?, ?)",
                (req.serverId, req.accessToken, ip, int(time.time() * 1000)),
            )
            await conn.commit()

    async def has_joined(
        self,
        username: str,
        server_id: str,
        ip: Optional[str] = None,
        base_url: str = None,
    ) -> Optional[Dict]:
        async with self.db.get_conn() as conn:
            # 1. 查找 Session
            cursor = await conn.execute(
                "SELECT access_token, ip, created_at FROM sessions WHERE server_id = ?",
                (server_id,),
            )
            session = await cursor.fetchone()
            if not session:
                return None  # No content

            access_token, stored_ip, created_at = session

            # 检查 Session 是否过期 (30秒)
            if (int(time.time() * 1000) - created_at) > self.SESSION_TTL:
                return None

            # 若开启了 prevent-proxy-connections，可以检查 IP
            # if ip and ip != stored_ip: return None

            # 2. 获取 Token 对应的 Profile
            t_cursor = await conn.execute(
                "SELECT profile_id FROM tokens WHERE access_token = ?", (access_token,)
            )
            t_row = await t_cursor.fetchone()
            if not t_row:
                return None

            profile_id = t_row[0]

            # 3. 获取 Profile 详情
            p_cursor = await conn.execute(
                "SELECT * FROM profiles WHERE id = ?", (profile_id,)
            )
            p_row = await p_cursor.fetchone()
            if not p_row or p_row[2] != username:  # 检查用户名是否匹配
                return None

            # 返回带签名的完整信息
            return await self._get_profile_json(p_row, sign=True, base_url=base_url)

    # =========================
    # 角色/材质部分 API 实现
    # =========================

    async def get_profile(
        self, uuid: str, unsigned: bool = True, base_url: str = None
    ) -> Optional[Dict]:
        async with self.db.get_conn() as conn:
            cursor = await conn.execute("SELECT * FROM profiles WHERE id = ?", (uuid,))
            row = await cursor.fetchone()
            if not row:
                return None
            return await self._get_profile_json(
                row, sign=not unsigned, base_url=base_url
            )

    async def get_profiles_by_names(
        self, names: list, base_url: str = None
    ) -> list[Dict]:
        """
        按名称批量查询角色（用于 POST /api/profiles/minecraft）
        """
        if not names or len(names) == 0:
            return []
        # 最多查询100个
        names = names[:100]
        async with self.db.get_conn() as conn:
            placeholders = ",".join("?" * len(names))
            query = f"SELECT * FROM profiles WHERE name IN ({placeholders})"
            cursor = await conn.execute(query, names)
            rows = await cursor.fetchall()
            results = []
            for row in rows:
                # 不包含 properties（简化版）
                pid, uid, name, model, skin_hash, cape_hash = row
                results.append({"id": pid, "name": name})
            return results

    async def upload_texture(
        self,
        access_token: str,
        uuid: str,
        texture_type: str,
        file_bytes: bytes,
        model: str = "",
    ):
        # 1. 鉴权
        async with self.db.get_conn() as conn:
            # SELECT token's user and bound profile
            cursor = await conn.execute(
                "SELECT user_id, profile_id FROM tokens WHERE access_token = ?",
                (access_token,),
            )
            row = await cursor.fetchone()
            if not row:
                raise ForbiddenOperationException("Unauthorized")

            token_user_id, token_profile_id = row

            # 验证目标 profile 属于该 token 所在的 user
            p_cursor = await conn.execute(
                "SELECT user_id FROM profiles WHERE id = ?", (uuid,)
            )
            p_row = await p_cursor.fetchone()
            if not p_row or p_row[0] != token_user_id:
                raise ForbiddenOperationException("Unauthorized")

            # 检查文件大小限制（从 settings 读取）
            size_cur = await conn.execute(
                "SELECT value FROM settings WHERE key='max_texture_size'"
            )
            size_row = await size_cur.fetchone()
            max_size_kb = int(size_row[0]) if size_row else 1024
            max_size_bytes = max_size_kb * 1024

            if len(file_bytes) > max_size_bytes:
                raise IllegalArgumentException(
                    f"Texture file too large. Maximum size: {max_size_kb}KB"
                )

            # 2. 检查图片安全性与规范
            try:
                # 打开并验证为 PNG
                img = Image.open(BytesIO(file_bytes))
                if img.format != "PNG":
                    raise IllegalArgumentException("Texture must be PNG")

                # 检查尺寸 (64x32, 64x64 等)
                is_cape = texture_type.lower() == "cape"
                if not self.crypto.validate_texture_dimensions(img, is_cape):
                    raise IllegalArgumentException("Invalid image dimensions")

                # 规范化图片：转换为 RGBA 并重新保存为干净的 PNG（去除不必要 chunk）
                img = img.convert("RGBA")
                normalized_io = BytesIO()
                img.save(normalized_io, format="PNG")
                normalized_bytes = normalized_io.getvalue()

                # 3. 计算 Hash（基于像素数据，符合规范）
                # 修复：应传入 Image 对象而非 PNG 字节
                texture_hash = self.crypto.compute_texture_hash_from_image(img)

                # 4. 保存文件 (保存到本地 textures/ 目录)，文件名使用 {hash}.png
                textures_dir = os.path.join(os.path.dirname(__file__), "textures")
                os.makedirs(textures_dir, exist_ok=True)
                file_name = f"{texture_hash}.png"
                file_path = os.path.join(textures_dir, file_name)
                with open(file_path, "wb") as wf:
                    wf.write(normalized_bytes)

                print(
                    f"DEBUG: Saved texture {texture_hash} -> {file_path} (size: {len(normalized_bytes)} bytes)"
                )

                # 5. 更新数据库
                field = "skin_hash" if texture_type.lower() == "skin" else "cape_hash"
                if texture_type.lower() == "skin":
                    # 更新 model
                    m_val = "slim" if model == "slim" else "default"
                    sql = f"UPDATE profiles SET skin_hash = ?, texture_model = ? WHERE id = ?"
                    params = [texture_hash, m_val, uuid]
                else:
                    sql = f"UPDATE profiles SET {field} = ? WHERE id = ?"
                    params = [texture_hash, uuid]

                await conn.execute(sql, tuple(params))
                await conn.commit()

            except YggdrasilError:
                # 已知的业务异常直接抛出
                raise
            except Exception as e:
                print("Texture processing error:", e)
                raise IllegalArgumentException("Failed to process texture")

    async def delete_texture(self, access_token: str, uuid: str, texture_type: str):
        async with self.db.get_conn() as conn:
            # 鉴权
            cursor = await conn.execute(
                "SELECT user_id, profile_id FROM tokens WHERE access_token = ?",
                (access_token,),
            )
            row = await cursor.fetchone()
            if not row:
                raise ForbiddenOperationException("Unauthorized")

            token_user_id, token_profile_id = row

            p_cursor = await conn.execute(
                "SELECT user_id FROM profiles WHERE id = ?", (uuid,)
            )
            p_row = await p_cursor.fetchone()
            if not p_row or p_row[0] != token_user_id:
                raise ForbiddenOperationException("Unauthorized")

            field = "skin_hash" if texture_type.lower() == "skin" else "cape_hash"
            await conn.execute(
                f"UPDATE profiles SET {field} = NULL WHERE id = ?", (uuid,)
            )
            await conn.commit()
