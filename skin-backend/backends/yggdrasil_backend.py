import time
import json
import base64
from typing import Dict, Optional, Tuple

from utils.crypto import CryptoUtils
from utils.typing import User, PlayerProfile, Token, Session, normalize_texture_model
from utils.uuid_utils import generate_random_uuid
from utils.password_utils import hash_password, verify_password
from database_module import Database
from services import TextureStorage


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
    def __init__(self, db: Database, crypto: CryptoUtils, texture_storage: TextureStorage, config=None):
        self.db = db
        self.crypto = crypto
        self.texture_storage = texture_storage
        self.config = config
        self.TOKEN_TTL = 15 * 24 * 3600 * 1000  # 15天 (毫秒)
        self.SESSION_TTL = 30 * 1000  # 30秒 (用于join验证)

    def _site_url(self) -> str:
        return self.config.get("server.site_url", "").rstrip("/") if self.config else ""

    def build_profile_json(self, profile: PlayerProfile, sign: bool = False) -> Dict:
        """构建角色 JSON，包含 textures 和签名。"""
        textures_payload = {
            "timestamp": int(time.time() * 1000),
            "profileId": profile.id,
            "profileName": profile.name,
            "textures": {},
        }
        base_texture_url = f"{self._site_url()}/static/textures/"

        if profile.skin_hash:
            textures_payload["textures"]["SKIN"] = {
                "url": base_texture_url + profile.skin_hash + ".png"
            }
            if profile.texture_model == "slim":
                textures_payload["textures"]["SKIN"]["metadata"] = {"model": "slim"}

        if profile.cape_hash:
            textures_payload["textures"]["CAPE"] = {
                "url": base_texture_url + profile.cape_hash + ".png"
            }

        textures_base64 = base64.b64encode(
            json.dumps(textures_payload).encode("utf-8")
        ).decode("utf-8")

        prop = {"name": "textures", "value": textures_base64}
        if sign:
            prop["signature"] = self.crypto.sign_data(textures_base64)

        return {
            "id": profile.id,
            "name": profile.name,
            "properties": [
                prop,
                {"name": "uploadableTextures", "value": "skin,cape"},
            ],
        }

    async def build_authenticate_response(
        self, username, password, client_token, request_user: bool
    ) -> Dict:
        access_token, avail_players, selected_profile, user_id = await self.authenticate(
            username, password, client_token
        )
        resp = {
            "accessToken": access_token,
            "clientToken": client_token or access_token,
            "availableProfiles": [{"id": p.id, "name": p.name} for p in avail_players],
        }
        if selected_profile:
            resp["selectedProfile"] = {
                "id": selected_profile.id,
                "name": selected_profile.name,
            }
        if request_user:
            user_obj = await self.db.user.get_by_id(user_id)
            if user_obj:
                resp["user"] = {
                    "id": user_id,
                    "properties": [
                        {"name": "preferredLanguage", "value": user_obj.preferred_language}
                    ],
                }
        return resp

    async def lookup_profile_by_name(self, player_name: str) -> Optional[Dict]:
        p = await self.db.user.get_profile_by_name(player_name)
        if p:
            return {"id": p.id, "name": p.name}
        return None

    async def build_metadata(self, default_site_url: str) -> Dict:
        site_name = await self.db.setting.get("site_name", "Yggdrasil 皮肤站")
        site_url = (self.config.get("server.site_url", default_site_url) if self.config else default_site_url).rstrip("/")
        host = (
            site_url.replace("https://", "").replace("http://", "").split("/")[0]
            if site_url
            else "localhost"
        )
        return {
            "meta": {
                "serverName": site_name,
                "implementationName": "element-skin",
                "implementationVersion": "1.0.0",
                "links": {
                    "homepage": f"{site_url}/" if site_url else None,
                    "register": f"{site_url}/register/" if site_url else None,
                },
                "feature.non_email_login": True,
            },
            "skinDomains": await self.db.fallback.collect_skin_domains() + [host],
            "signaturePublickey": self.crypto.get_public_key_pem(),
        }

    async def _cleanup_tokens(self, user_id: str):
        cutoff = int(time.time() * 1000) - self.TOKEN_TTL
        await self.db.user.delete_expired_tokens(user_id, cutoff)
        await self.db.user.delete_surplus_tokens(user_id, keep=5)

    async def _verify_credentials(self, username, password) -> Tuple[Optional[User], Optional[PlayerProfile]]:
        user = await self.db.user.get_by_email(username)
        login_profile = None
        if not user:
            login_profile = await self.db.user.get_profile_by_name(username)
            if login_profile:
                user = await self.db.user.get_by_id(login_profile.user_id)

        if not user:
            return None, None

        if verify_password(password, user.password):
            if not user.password.startswith("$2"):
                new_hash = hash_password(password)
                await self.db.user.update_password(user.id, new_hash)
            return user, login_profile

        return None, None

    async def authenticate(
        self, username, password, clientToken
    ) -> Tuple[str, list, Optional[PlayerProfile], str]:
        user, login_profile = await self._verify_credentials(username, password)
        if not user:
            raise ForbiddenOperationException(
                "Invalid credentials. Invalid username or password."
            )

        user_id = user.id
        access_token = generate_random_uuid()
        client_token = clientToken if clientToken else generate_random_uuid()

        selected_profile = None
        if login_profile:
            # 如果是通过角色名登录，availableProfiles 仅包含该角色，且必须被选中
            avail_players = [login_profile]
            selected_profile = login_profile
        else:
            # 如果是通过邮箱登录，返回该用户下的所有角色
            avail_players = await self.db.user.get_profiles_by_user(user_id)
            # 如果只有一个角色，则默认选中
            if len(avail_players) == 1:
                selected_profile = avail_players[0]

        pid_to_bind = selected_profile.id if selected_profile else None
        created_at = int(time.time() * 1000)
        await self.db.user.add_token(
            Token(access_token, client_token, user_id, pid_to_bind, created_at)
        )
        await self._cleanup_tokens(user_id)

        return access_token, avail_players, selected_profile, user_id

    async def refresh(
        self, accessToken, clientToken, selectedProfile_uuid, requestUser=False
    ) -> Dict:
        selectedProfile_uuid = (
            selectedProfile_uuid.replace("-", "") if selectedProfile_uuid else None
        )
        token_data = await self.db.user.get_token(accessToken)
        if not token_data:
            raise ForbiddenOperationException("Invalid token.")
        if clientToken and clientToken != token_data.client_token:
            raise ForbiddenOperationException("Invalid token.")

        await self.db.user.delete_token(accessToken)

        new_profile_id = token_data.profile_id
        selected_profile_resp = None

        if selectedProfile_uuid:
            if token_data.profile_id:
                raise IllegalArgumentException(
                    "Access token already has a profile assigned."
                )
            p_check = await self.db.user.verify_profile_ownership(
                token_data.user_id, selectedProfile_uuid
            )
            if not p_check:
                raise ForbiddenOperationException("Invalid profile.")
            new_profile_id = selectedProfile_uuid
            p_obj = await self.db.user.get_profile_by_id(selectedProfile_uuid)
            if p_obj:
                selected_profile_resp = {"id": p_obj.id, "name": p_obj.name}
        elif token_data.profile_id:
            p_obj = await self.db.user.get_profile_by_id(token_data.profile_id)
            if p_obj:
                selected_profile_resp = {"id": p_obj.id, "name": p_obj.name}

        new_access_token = generate_random_uuid()
        created_at = int(time.time() * 1000)
        await self.db.user.add_token(
            Token(
                new_access_token,
                token_data.client_token,
                token_data.user_id,
                new_profile_id,
                created_at,
            )
        )
        await self._cleanup_tokens(token_data.user_id)

        resp = {"accessToken": new_access_token, "clientToken": token_data.client_token}
        if selected_profile_resp:
            resp["selectedProfile"] = selected_profile_resp
        if requestUser:
            user = await self.db.user.get_by_id(token_data.user_id)
            if user:
                resp["user"] = {
                    "id": user.id,
                    "properties": [
                        {"name": "preferredLanguage", "value": user.preferred_language}
                    ],
                }
        return resp

    async def validate(self, req: Dict):
        token_data = await self.db.user.get_token(req.get("accessToken"))
        if not token_data:
            raise ForbiddenOperationException("Invalid token.")
        if req.get("clientToken") and req["clientToken"] != token_data.client_token:
            raise ForbiddenOperationException("Invalid token.")
        if (int(time.time() * 1000) - token_data.created_at) > self.TOKEN_TTL:
            raise ForbiddenOperationException("Invalid token.")

    async def invalidate(self, access_token: str):
        await self.db.user.delete_token(access_token)

    async def signout(self, username, password):
        user, _ = await self._verify_credentials(username, password)
        if not user:
            raise ForbiddenOperationException(
                "Invalid credentials. Invalid username or password."
            )
        await self.db.user.delete_tokens_by_user(user.id)

    async def join_server(self, access_token, selected_profile_id, server_id, ip: str):
        token_data = await self.db.user.get_token(access_token)
        if not token_data:
            raise ForbiddenOperationException("Invalid token.")
        if token_data.profile_id != selected_profile_id:
            raise ForbiddenOperationException("Invalid token.")
        await self.db.user.delete_session(server_id)
        await self.db.user.add_session(
            Session(server_id, access_token, ip, int(time.time() * 1000))
        )

    async def has_joined(
        self, username: str, server_id: str
    ) -> Optional[PlayerProfile]:
        session = await self.db.user.get_session(server_id)
        if not session:
            return None
        if (int(time.time() * 1000) - session.created_at) > self.SESSION_TTL:
            return None

        token_data = await self.db.user.get_token(session.access_token)
        if not token_data or not token_data.profile_id:
            return None

        profile = await self.db.user.get_profile_by_id(token_data.profile_id)
        if not profile or profile.name != username:
            return None

        if await self.db.user.is_banned(profile.user_id):
            raise ForbiddenOperationException(
                "Account is banned. Please contact administrator."
            )

        return profile

    async def get_profile(self, uuid: str) -> Optional[PlayerProfile]:
        uuid = uuid.replace("-", "")
        profile = await self.db.user.get_profile_by_id(uuid)
        if not profile:
            return None
        return profile

    async def get_profiles_by_names(self, names: list) -> list[Dict]:
        if not names:
            return []
        profiles = await self.db.user.search_profiles_by_names(names[:100], limit=100)
        return [{"id": p.id, "name": p.name} for p in profiles]

    async def _authorize_profile_owner(self, access_token: str, uuid: str) -> Token:
        token_data = await self.db.user.get_token(access_token)
        if not token_data:
            raise ForbiddenOperationException("Unauthorized")
        if not await self.db.user.verify_profile_ownership(token_data.user_id, uuid):
            raise ForbiddenOperationException("Unauthorized")
        return token_data

    async def upload_texture(
        self,
        access_token: str,
        uuid: str,
        texture_type: str,
        file_bytes: bytes,
        model: str = "",
    ):
        uuid = uuid.replace("-", "")
        token_data = await self._authorize_profile_owner(access_token, uuid)

        max_size_kb_str = await self.db.setting.get("max_texture_size", "1024")
        if len(file_bytes) > int(max_size_kb_str) * 1024:
            raise IllegalArgumentException(f"Texture file too large.")

        try:
            texture_hash = self.texture_storage.process_and_save(file_bytes, texture_type)
            await self.db.texture.add_to_library(token_data.user_id, texture_hash, texture_type)
            if texture_type.lower() == "skin":
                m_val = normalize_texture_model(model)
                await self.db.user.update_profile_skin(uuid, texture_hash)
                await self.db.user.update_profile_texture_model(uuid, m_val)
            else:
                await self.db.user.update_profile_cape(uuid, texture_hash)
        except ValueError as e:
            raise IllegalArgumentException(str(e))
        except Exception as e:
            if isinstance(e, YggdrasilError):
                raise
            print(f"Texture processing error: {e}")
            raise IllegalArgumentException("Failed to process texture")

    async def delete_texture(self, access_token: str, uuid: str, texture_type: str):
        uuid = uuid.replace("-", "")
        await self._authorize_profile_owner(access_token, uuid)

        if texture_type.lower() == "skin":
            await self.db.user.update_profile_skin(uuid, None)
        else:
            await self.db.user.update_profile_cape(uuid, None)
