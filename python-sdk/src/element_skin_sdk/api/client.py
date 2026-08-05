"""Synchronous Element Skin `/v2` API client."""

from __future__ import annotations

from collections.abc import Iterable
from typing import Any

import httpx

from ..http import HTTPClient
from ..models import TokenSet
from ..permissions import AccountScopes, MinecraftScopes, ProfileScopes, TextureScopes, WardrobeScopes
from ..permissions.validator import PermissionValidator


class ElementSkinAPI:
    def __init__(
        self,
        base_url: str,
        *,
        access_token: str | None = None,
        token: TokenSet | None = None,
        permissions: Iterable[str] | None = None,
        http_client: HTTPClient | None = None,
        transport: httpx.BaseTransport | None = None,
        timeout: float = 10.0,
    ):
        token_permissions = tuple(token.permissions) if token is not None else None
        self.permissions = tuple(permissions) if permissions is not None else token_permissions
        self._http = http_client or HTTPClient(
            base_url,
            access_token=access_token or (token.access_token if token else None),
            transport=transport,
            timeout=timeout,
        )

    def close(self) -> None:
        self._http.close()

    def __enter__(self) -> "ElementSkinAPI":
        return self

    def __exit__(self, exc_type: object, exc: object, tb: object) -> None:
        self.close()

    def me(self) -> dict[str, Any]:
        self._require(AccountScopes.READ_SELF)
        return self._http.get("/v2/users/me")

    def request_email_change_code(self, email: str) -> dict[str, Any]:
        self._require(AccountScopes.UPDATE_SELF)
        return self._http.post(
            "/v2/users/me/email/verification-code",
            json={"email": email},
        )

    def change_email(self, email: str, code: str) -> None:
        self._require(AccountScopes.UPDATE_SELF)
        self._http.put(
            "/v2/users/me/email",
            json={"email": email, "code": code},
        )

    def list_profiles(self, *, cursor: str | None = None, page_size: int | None = None) -> dict[str, Any]:
        self._require(ProfileScopes.READ_OWNED)
        return self._http.get(
            "/v2/users/me/profiles",
            params=_clean_params({"cursor": cursor, "limit": page_size}),
        )

    def create_profile(self, name: str, *, model: str = "default") -> dict[str, Any]:
        self._require(ProfileScopes.CREATE_OWNED)
        return self._http.post("/v2/users/me/profiles", json={"name": name, "model": model})

    def update_profile(self, profile_id: str, **fields: Any) -> None:
        self._require(ProfileScopes.UPDATE_OWNED)
        self._http.patch(f"/v2/users/me/profiles/{profile_id}", json=fields)

    def delete_profile(self, profile_id: str) -> None:
        self._require(ProfileScopes.DELETE_OWNED)
        self._http.delete(f"/v2/users/me/profiles/{profile_id}")

    def list_textures(
        self,
        *,
        texture_type: str | None = None,
        cursor: str | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        self._require(TextureScopes.READ_OWNED)
        return self._http.get(
            "/v2/users/me/textures",
            params=_clean_params(
                {"texture_type": texture_type, "cursor": cursor, "limit": page_size},
            ),
        )

    def get_texture(self, texture_hash: str, texture_type: str) -> dict[str, Any]:
        self._require(TextureScopes.READ_OWNED)
        return self._http.get(f"/v2/users/me/textures/{texture_hash}/{texture_type}")

    def update_texture(self, texture_hash: str, texture_type: str, **fields: Any) -> dict[str, Any]:
        required: list[str] = []
        if any(field in fields for field in ("note", "model")):
            required.append(TextureScopes.UPDATE_METADATA_OWNED)
        if "is_public" in fields:
            required.append(TextureScopes.UPDATE_VISIBILITY_OWNED)
        self._require(*(required or [TextureScopes.UPDATE_METADATA_OWNED]))
        return self._http.patch(f"/v2/users/me/textures/{texture_hash}/{texture_type}", json=fields)

    def delete_texture(self, texture_hash: str, texture_type: str) -> None:
        self._require(TextureScopes.DELETE_OWNED)
        self._http.delete(f"/v2/users/me/textures/{texture_hash}/{texture_type}")

    def add_texture_to_wardrobe(self, texture_hash: str, *, texture_type: str) -> None:
        self._require(WardrobeScopes.ENTRY_ADD_OWNED)
        self._http.post(
            f"/v2/users/me/textures/{texture_hash}/wardrobe",
            params={"texture_type": texture_type},
        )

    def apply_texture(self, texture_hash: str, *, profile_id: str, texture_type: str) -> None:
        self._require(WardrobeScopes.ENTRY_APPLY_OWNED)
        self._http.post(
            f"/v2/users/me/textures/{texture_hash}/apply",
            json={"profile_id": profile_id, "texture_type": texture_type},
        )

    def minecraft_profile(self, username: str) -> dict[str, Any]:
        self._require(MinecraftScopes.PROFILE_READ_PUBLIC)
        return self._http.get(f"/v2/minecraft/profiles/by-name/{username}")

    def minecraft_profiles(self, usernames: list[str]) -> dict[str, Any]:
        self._require(MinecraftScopes.PROFILE_READ_PUBLIC)
        return self._http.post("/v2/minecraft/profiles/by-names", json={"names": usernames})

    def minecraft_has_joined(
        self,
        *,
        username: str,
        server_id: str,
        ip: str | None = None,
    ) -> dict[str, Any]:
        self._require(MinecraftScopes.SESSION_HASJOINED_SERVER)
        return self._http.post(
            "/v2/minecraft/session/has-joined",
            json=_clean_params({"username": username, "server_id": server_id, "ip": ip}),
        )

    def _require(self, *permissions: str) -> None:
        PermissionValidator.require_token_permissions(self.permissions, permissions)


def _clean_params(params: dict[str, Any]) -> dict[str, Any]:
    return {key: value for key, value in params.items() if value is not None}
