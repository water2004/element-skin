import base64
import json
import time


class Texture:
    skinHash: str
    capeHash: str
    model: str

    def __init__(
        self, skinHash: str = None, capeHash: str = None, model: str = "default"
    ):
        self.skinHash = skinHash
        self.capeHash = capeHash
        self.model = model

    def to_json(
        self, timeStamp: int, profileId: str, profileName: str, baseUrl: str
    ) -> dict:
        base_texture_url = "/static/textures/"
        if baseUrl:
            base_texture_url = baseUrl.rstrip("/") + "/static/textures/"

        textures_payload = {
            "timestamp": timeStamp,
            "profileId": profileId,
            "profileName": profileName,
            "textures": {},
        }
        if self.skinHash:
            textures_payload["textures"]["SKIN"] = {
                "url": base_texture_url + self.skinHash + ".png"
            }
            if self.model == "slim":
                textures_payload["textures"]["SKIN"]["metadata"] = {"model": "slim"}

        if self.capeHash:
            textures_payload["textures"]["CAPE"] = {
                "url": base_texture_url + self.capeHash + ".png"
            }


class User:
    id: str
    email: str
    password: str
    is_admin: int
    display_name: str
    banned_until: int
    preferredLanguage: str

    def __init__(
        self,
        id: str,
        email: str,
        password: str,
        is_admin: int = 0,
        preferred_language: str = "zh_CN",
        display_name: str = "",
        banned_until: int = None,
    ):
        self.id = id
        self.email = email
        self.password = password
        self.is_admin = is_admin
        self.preferredLanguage = preferred_language
        self.display_name = display_name
        self.banned_until = banned_until

    def to_json(self) -> dict:
        return {
            "id": self.id,
            "preferredLanguage": self.preferredLanguage,
        }


class PlayerProfile:
    id: str
    user_id: str
    name: str
    texture_model: str
    skin_hash: str
    cape_hash: str

    def __init__(
        self,
        id: str,
        user_id: str,
        name: str,
        texture_model: str = "default",
        skin_hash: str = None,
        cape_hash: str = None,
    ):
        self.id = id
        self.user_id = user_id
        self.name = name
        self.texture_model = texture_model
        self.skin_hash = skin_hash
        self.cape_hash = cape_hash

    def to_json(self, texture: Texture) -> dict:
        textures_json = json.dumps(
            texture.to_json(int(time.time() * 1000), self.id, self.name, None)
        )
        textures_base64 = base64.b64encode(textures_json.encode("utf-8")).decode(
            "utf-8"
        )
        # 后续再支持签名和 uploadableTextures
        return {
            "id": self.id,
            "name": self.name,
            "properties": [
                {"name": "textures", "value": textures_base64},
                {"name": "uploadableTextures", "value": "skin,cape"},
            ],
        }


class InviteCode:
    code: str
    created_at: int
    used_by: str
    total_uses: int
    used_count: int
    note: str

    def __init__(
        self,
        code: str,
        created_at: int,
        used_by: str = None,
        total_uses: int = 1,
        used_count: int = 0,
        note: str = "",
    ):
        self.code = code
        self.created_at = created_at
        self.used_by = used_by
        self.total_uses = total_uses
        self.used_count = used_count
        self.note = note


class Token:
    access_token: str
    client_token: str
    user_id: str
    profile_id: str
    created_at: int

    def __init__(
        self,
        access_token: str,
        client_token: str,
        user_id: str,
        profile_id: str = None,
        created_at: int = None,
    ):
        self.access_token = access_token
        self.client_token = client_token
        self.user_id = user_id
        self.profile_id = profile_id
        self.created_at = created_at


class Session:
    server_id: str
    access_token: str
    ip: str
    created_at: int

    def __init__(
        self,
        server_id: str,
        access_token: str,
        ip: str = None,
        created_at: int = None,
    ):
        self.server_id = server_id
        self.access_token = access_token
        self.ip = ip
        self.created_at = created_at
