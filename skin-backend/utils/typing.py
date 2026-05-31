class User:
    id: str
    email: str
    password: str
    is_admin: int
    display_name: str
    banned_until: int
    preferred_language: str

    def __init__(
        self,
        id: str,
        email: str,
        password: str,
        is_admin: int = 0,
        preferred_language: str = "zh_CN",
        display_name: str = "",
        banned_until: int = None,
        avatar_hash: str = None,
    ):
        self.id = id
        self.email = email
        self.password = password
        self.is_admin = is_admin
        self.preferred_language = preferred_language
        self.display_name = display_name
        self.banned_until = banned_until
        self.avatar_hash = avatar_hash


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


def normalize_texture_model(value: str) -> str:
    """把皮肤模型/变体归一化为 'slim' 或 'default'。"""
    return "slim" if value == "slim" else "default"


def serialize_profile_summary(profile: PlayerProfile) -> dict:
    """角色列表项的统一序列化。"""
    return {
        "id": profile.id,
        "name": profile.name,
        "model": profile.texture_model,
        "skin_hash": profile.skin_hash,
        "cape_hash": profile.cape_hash,
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
