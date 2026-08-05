"""Local validation for OAuth permission scopes."""

from __future__ import annotations

from collections.abc import Iterable

from ..exceptions import InvalidScope, PermissionDenied
from .catalog import PermissionCatalog


class PermissionValidator:
    DELEGATED_FORBIDDEN_SCOPES = frozenset({"server", "system"})
    CLIENT_CREDENTIALS_ALLOWED_SCOPES = frozenset({"any", "public", "server"})
    OIDC_SCOPES = frozenset({"openid", "profile", "email", "offline_access"})

    def __init__(self, catalog: PermissionCatalog | None = None):
        self.catalog = catalog

    def normalize(self, scopes: Iterable[str] | str | None) -> tuple[str, ...]:
        if scopes is None:
            return ()
        if isinstance(scopes, str):
            raw = scopes.split()
        else:
            raw = list(scopes)

        normalized = tuple(scope.strip() for scope in raw if scope and scope.strip())
        invalid_protocol_scopes = [
            scope
            for scope in normalized
            if "." not in scope and scope not in self.OIDC_SCOPES
        ]
        if invalid_protocol_scopes:
            raise InvalidScope("unknown OIDC scope", invalid_scopes=invalid_protocol_scopes)
        if self.catalog is not None:
            self.catalog.require_known(
                scope for scope in normalized if scope not in self.OIDC_SCOPES
            )
        return normalized

    def validate_delegated(self, scopes: Iterable[str] | str | None) -> tuple[str, ...]:
        normalized = self.normalize(scopes)
        invalid = [
            scope
            for scope in normalized
            if _scope_segment(scope) in self.DELEGATED_FORBIDDEN_SCOPES
        ]
        if invalid:
            raise InvalidScope(
                "authorization code and device flows cannot request server or system scopes",
                invalid_scopes=invalid,
            )
        oidc_scopes = [scope for scope in normalized if scope in self.OIDC_SCOPES]
        if oidc_scopes and "openid" not in oidc_scopes:
            raise InvalidScope("OIDC scopes require openid", invalid_scopes=oidc_scopes)
        return normalized

    def validate_client_credentials(self, scopes: Iterable[str] | str | None) -> tuple[str, ...]:
        normalized = self.normalize(scopes)
        invalid = [
            scope
            for scope in normalized
            if _scope_segment(scope) not in self.CLIENT_CREDENTIALS_ALLOWED_SCOPES
        ]
        if invalid:
            raise InvalidScope(
                "client credentials flow can only request public, server, or any scopes",
                invalid_scopes=invalid,
            )
        return normalized

    @staticmethod
    def require_token_permissions(
        available_permissions: Iterable[str] | None,
        required_permissions: Iterable[str],
    ) -> None:
        if available_permissions is None:
            return

        available = set(available_permissions)
        missing = sorted(permission for permission in required_permissions if permission not in available)
        if missing:
            raise PermissionDenied(
                403,
                f"missing required permission: {', '.join(missing)}",
                response_body={"detail": "missing required permission", "missing_permissions": missing},
            )


def _scope_segment(scope: str) -> str:
    parts = scope.split(".")
    return parts[2] if len(parts) >= 3 else ""
