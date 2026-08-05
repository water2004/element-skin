from __future__ import annotations

import pytest

from element_skin_sdk.exceptions import InvalidScope
from element_skin_sdk.permissions import (
    AccountScopes,
    ExternalIdentityScopes,
    IdentityProviderScopes,
    InviteScopes,
    MinecraftScopes,
    OIDCScopes,
    OfficialProfileScopes,
    PermissionCatalog,
    PermissionValidator,
    TextureScopes,
)

from .fixtures import PERMISSION_CATALOG_RESPONSE


def test_permission_catalog_loads_exact_definitions() -> None:
    catalog = PermissionCatalog.from_api_payload(PERMISSION_CATALOG_RESPONSE)

    assert catalog.codes == frozenset(
        {"account.read.self", "minecraft_session.hasjoined.server"}
    )
    definition = catalog.get("minecraft_session.hasjoined.server")
    assert definition is not None
    assert definition.category == "minecraft_session"
    assert definition.action == "hasjoined"
    assert definition.scope == "server"
    assert definition.description == "Query joined Minecraft session"


def test_permission_catalog_rejects_unknown_scope_with_exact_list() -> None:
    catalog = PermissionCatalog.from_api_payload(PERMISSION_CATALOG_RESPONSE)

    with pytest.raises(InvalidScope) as exc:
        catalog.require_known(["unknown.scope.self", "account.read.self"])

    assert exc.value.invalid_scopes == ["unknown.scope.self"]
    assert str(exc.value) == "unknown permission scope: unknown.scope.self"


def test_permission_catalog_rejects_missing_permissions_list() -> None:
    with pytest.raises(InvalidScope) as exc:
        PermissionCatalog.from_api_payload({"items": []})

    assert exc.value.invalid_scopes == []
    assert str(exc.value) == "permission metadata must contain a permissions list"


def test_permission_catalog_accepts_all_known_scopes_in_original_order() -> None:
    catalog = PermissionCatalog.from_api_payload(PERMISSION_CATALOG_RESPONSE)

    assert catalog.require_known(
        ["minecraft_session.hasjoined.server", "account.read.self"]
    ) == ("minecraft_session.hasjoined.server", "account.read.self")


def test_validator_accepts_delegated_user_scope_and_rejects_server_scope() -> None:
    validator = PermissionValidator()

    assert validator.validate_delegated(AccountScopes.READ_SELF) == ("account.read.self",)
    with pytest.raises(InvalidScope) as exc:
        validator.validate_delegated([MinecraftScopes.SESSION_HASJOINED_SERVER])

    assert exc.value.invalid_scopes == ["minecraft_session.hasjoined.server"]


def test_validator_accepts_client_credentials_app_only_scopes_and_rejects_self_scope() -> None:
    validator = PermissionValidator()

    assert validator.validate_client_credentials(MinecraftScopes.SESSION_HASJOINED_SERVER) == (
        "minecraft_session.hasjoined.server",
    )
    assert validator.validate_client_credentials([InviteScopes.READ_ANY]) == ("invite.read.any",)
    with pytest.raises(InvalidScope) as exc:
        validator.validate_client_credentials([AccountScopes.READ_SELF])

    assert exc.value.invalid_scopes == ["account.read.self"]


def test_validator_normalize_handles_none_blank_values_and_catalog() -> None:
    catalog = PermissionCatalog.from_api_payload(PERMISSION_CATALOG_RESPONSE)
    validator = PermissionValidator(catalog)

    assert validator.normalize(None) == ()
    assert validator.normalize([" account.read.self ", "", "  "]) == ("account.read.self",)
    with pytest.raises(InvalidScope) as exc:
        validator.normalize(["missing.scope.self"])

    assert exc.value.invalid_scopes == ["missing.scope.self"]


def test_validator_accepts_oidc_scopes_with_catalog_and_rejects_invalid_combinations() -> None:
    catalog = PermissionCatalog.from_api_payload(PERMISSION_CATALOG_RESPONSE)
    validator = PermissionValidator(catalog)

    assert validator.validate_delegated(
        [OIDCScopes.OPENID, OIDCScopes.PROFILE, AccountScopes.READ_SELF]
    ) == ("openid", "profile", "account.read.self")

    with pytest.raises(InvalidScope) as exc:
        validator.validate_delegated([OIDCScopes.EMAIL])
    assert exc.value.invalid_scopes == ["email"]
    assert str(exc.value) == "OIDC scopes require openid"

    with pytest.raises(InvalidScope) as exc:
        validator.validate_delegated(["unknown_protocol_scope"])
    assert exc.value.invalid_scopes == ["unknown_protocol_scope"]
    assert str(exc.value) == "unknown OIDC scope"


def test_v2_permission_constants_match_current_protocol_codes() -> None:
    assert TextureScopes.CREATE_OWNED == "texture.create.owned"
    assert TextureScopes.UPDATE_METADATA_OWNED == "texture.update_metadata.owned"
    assert TextureScopes.UPDATE_VISIBILITY_OWNED == "texture.update_visibility.owned"
    assert ExternalIdentityScopes.CREATE_OWNED == "external_identity.create.owned"
    assert IdentityProviderScopes.UPDATE_ANY == "identity_provider.update.any"
    assert OfficialProfileScopes.REFRESH_OWNED == "official_profile.refresh.owned"
