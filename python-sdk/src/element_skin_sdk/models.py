"""Shared SDK data models."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any


@dataclass(frozen=True)
class UserInfo:
    id: str
    email: str
    display_name: str = ""
    protected: bool = False
    permissions: tuple[str, ...] = ()

    @classmethod
    def from_mapping(cls, data: dict[str, Any]) -> "UserInfo":
        permissions = data.get("permissions") or []
        if isinstance(permissions, str):
            permissions = permissions.split()

        return cls(
            id=str(data["id"]),
            email=str(data["email"]),
            display_name=str(data.get("display_name") or data.get("username") or ""),
            protected=bool(data.get("protected", False)),
            permissions=tuple(str(permission) for permission in permissions),
        )


@dataclass(frozen=True)
class TokenSet:
    access_token: str
    token_type: str
    expires_in: int
    scope: str = ""
    refresh_token: str | None = None
    permissions: tuple[str, ...] = ()
    id_token: str | None = None

    @classmethod
    def from_mapping(cls, data: dict[str, Any]) -> "TokenSet":
        permissions = data.get("permissions") or []
        if isinstance(permissions, str):
            permissions = permissions.split()

        return cls(
            access_token=str(data["access_token"]),
            token_type=str(data.get("token_type", "Bearer")),
            expires_in=int(data.get("expires_in", 0)),
            scope=str(data.get("scope", "")),
            refresh_token=data.get("refresh_token"),
            permissions=tuple(str(permission) for permission in permissions),
            id_token=data.get("id_token"),
        )


@dataclass(frozen=True)
class AuthorizationSession:
    authorization_url: str
    code_verifier: str
    code_challenge: str
    state: str
    scopes: tuple[str, ...]
    nonce: str | None = None


@dataclass(frozen=True)
class DeviceAuthorization:
    device_code: str
    user_code: str
    verification_uri: str
    verification_uri_complete: str | None
    expires_in: int
    interval: int
    scope: str = ""
    permissions: tuple[str, ...] = ()

    @classmethod
    def from_mapping(cls, data: dict[str, Any]) -> "DeviceAuthorization":
        permissions = data.get("permissions") or []
        if isinstance(permissions, str):
            permissions = permissions.split()

        return cls(
            device_code=str(data["device_code"]),
            user_code=str(data["user_code"]),
            verification_uri=str(data["verification_uri"]),
            verification_uri_complete=data.get("verification_uri_complete"),
            expires_in=int(data["expires_in"]),
            interval=int(data.get("interval", 5)),
            scope=str(data.get("scope", "")),
            permissions=tuple(str(permission) for permission in permissions),
        )


@dataclass(frozen=True)
class PermissionDefinition:
    code: str
    category: str
    action: str
    scope: str
    description: str = ""

    @classmethod
    def from_mapping(cls, data: dict[str, Any]) -> "PermissionDefinition":
        code = str(data.get("code") or data.get("permission") or "")
        segments = code.split(".")
        category = str(data.get("category") or (segments[0] if len(segments) >= 1 else ""))
        action = str(data.get("action") or (segments[1] if len(segments) >= 2 else ""))
        scope = str(data.get("scope") or (segments[2] if len(segments) >= 3 else ""))

        return cls(
            code=code,
            category=category,
            action=action,
            scope=scope,
            description=str(data.get("description") or data.get("name") or ""),
        )
