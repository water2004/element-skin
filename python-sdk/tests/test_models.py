from __future__ import annotations

from element_skin_sdk import UserInfo as ExportedUserInfo
from element_skin_sdk.models import DeviceAuthorization, PermissionDefinition, TokenSet, UserInfo


def test_token_set_accepts_space_separated_permissions_and_defaults() -> None:
    tokens = TokenSet.from_mapping(
        {
            "access_token": "access-token-1",
            "permissions": "account.read.self profile.read.owned",
        }
    )

    assert tokens.access_token == "access-token-1"
    assert tokens.token_type == "Bearer"
    assert tokens.expires_in == 0
    assert tokens.scope == ""
    assert tokens.refresh_token is None
    assert tokens.permissions == ("account.read.self", "profile.read.owned")
    assert tokens.id_token is None


def test_user_info_parses_protected_flag_and_permissions_exactly() -> None:
    user = UserInfo.from_mapping(
        {
            "id": "user-1",
            "email": "alice@example.test",
            "display_name": "Alice",
            "protected": True,
            "permissions": "account.read.self permission_protected.manage.any",
        }
    )

    assert user.id == "user-1"
    assert user.email == "alice@example.test"
    assert user.display_name == "Alice"
    assert user.protected is True
    assert user.permissions == (
        "account.read.self",
        "permission_protected.manage.any",
    )


def test_user_info_defaults_to_unprotected_and_uses_username_fallback() -> None:
    user = UserInfo.from_mapping(
        {
            "id": "user-2",
            "email": "bob@example.test",
            "username": "Bob",
            "permissions": ["account.read.self"],
        }
    )

    assert ExportedUserInfo is UserInfo
    assert user.id == "user-2"
    assert user.email == "bob@example.test"
    assert user.display_name == "Bob"
    assert user.protected is False
    assert user.permissions == ("account.read.self",)


def test_device_authorization_accepts_space_separated_permissions_and_defaults() -> None:
    device = DeviceAuthorization.from_mapping(
        {
            "device_code": "device-code-1",
            "user_code": "ABCD-EFGH",
            "verification_uri": "https://skin.example.test/oauth/device",
            "expires_in": 600,
            "permissions": "profile.read.owned",
        }
    )

    assert device.device_code == "device-code-1"
    assert device.user_code == "ABCD-EFGH"
    assert device.verification_uri == "https://skin.example.test/oauth/device"
    assert device.verification_uri_complete is None
    assert device.expires_in == 600
    assert device.interval == 5
    assert device.scope == ""
    assert device.permissions == ("profile.read.owned",)


def test_permission_definition_derives_segments_and_description_fallbacks() -> None:
    from_permission = PermissionDefinition.from_mapping(
        {"permission": "texture.read.public", "name": "Read public textures"}
    )
    empty = PermissionDefinition.from_mapping({})

    assert from_permission.code == "texture.read.public"
    assert from_permission.category == "texture"
    assert from_permission.action == "read"
    assert from_permission.scope == "public"
    assert from_permission.description == "Read public textures"
    assert empty.code == ""
    assert empty.category == ""
    assert empty.action == ""
    assert empty.scope == ""
    assert empty.description == ""
