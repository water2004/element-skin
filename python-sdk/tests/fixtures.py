"""Shared exact fixtures for SDK tests."""

TOKEN_RESPONSE = {
    "access_token": "access-token-1",
    "token_type": "Bearer",
    "expires_in": 3600,
    "scope": "account.read.self profile.read.owned",
    "refresh_token": "refresh-token-1",
    "permissions": ["account.read.self", "profile.read.owned"],
    "id_token": "id-token-1",
}

CLIENT_CREDENTIALS_TOKEN_RESPONSE = {
    "access_token": "server-token-1",
    "token_type": "Bearer",
    "expires_in": 1800,
    "scope": "minecraft_session.hasjoined.server",
    "permissions": ["minecraft_session.hasjoined.server"],
}

DEVICE_CODE_RESPONSE = {
    "device_code": "device-code-1",
    "user_code": "ABCD-EFGH",
    "verification_uri": "https://skin.example.test/oauth/device",
    "verification_uri_complete": "https://skin.example.test/oauth/device?user_code=ABCD-EFGH",
    "expires_in": 600,
    "interval": 5,
    "scope": "profile.read.owned",
    "permissions": ["profile.read.owned"],
}

ME_RESPONSE = {
    "id": "user-1",
    "email": "alice@example.test",
    "display_name": "Alice",
    "protected": False,
    "permissions": ["account.read.self"],
}

PROFILE_PAGE_RESPONSE = {
    "items": [{"id": "profile-1", "name": "Alice", "model": "slim"}],
    "has_next": False,
    "next_cursor": None,
    "page_size": 1,
}

MINECRAFT_HAS_JOINED_RESPONSE = {
    "id": "profile-uuid",
    "name": "Alice",
    "properties": [{"name": "textures", "value": "base64-textures"}],
}

PERMISSION_CATALOG_RESPONSE = {
    "permissions": [
        {
            "code": "account.read.self",
            "category": "account",
            "action": "read",
            "scope": "self",
            "description": "Read own account",
        },
        {
            "code": "minecraft_session.hasjoined.server",
            "category": "minecraft_session",
            "action": "hasjoined",
            "scope": "server",
            "description": "Query joined Minecraft session",
        },
    ]
}

WEBHOOK_SIGNING_SECRET = "whsec_test_signing_secret"
WEBHOOK_TIMESTAMP = 1_786_118_400_123
WEBHOOK_PAYLOAD = {
    "id": "evt_profile_updated_1",
    "type": "profile.updated",
    "created_at": 1_786_118_399_000,
    "data": {"profile_id": "profile-uuid", "user_id": "user-uuid"},
}
