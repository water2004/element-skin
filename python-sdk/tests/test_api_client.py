from __future__ import annotations

import httpx
import pytest

from element_skin_sdk import ElementSkinAPI
from element_skin_sdk.exceptions import APIError, PermissionDenied
from element_skin_sdk.models import TokenSet
from element_skin_sdk.permissions import (
    AccountScopes,
    MinecraftScopes,
    ProfileScopes,
    TextureScopes,
    WardrobeScopes,
)

from .conftest import RequestRecorder
from .fixtures import ME_RESPONSE, MINECRAFT_HAS_JOINED_RESPONSE, PROFILE_PAGE_RESPONSE


def test_me_sends_bearer_token_and_returns_exact_body(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(ME_RESPONSE))
    api = ElementSkinAPI(
        "https://skin.example.test",
        token=TokenSet(
            access_token="access-token-1",
            token_type="Bearer",
            expires_in=3600,
            permissions=(AccountScopes.READ_SELF,),
        ),
        transport=recorder.transport(),
    )

    body = api.me()

    assert body == ME_RESPONSE
    assert recorder.requests[0].method == "GET"
    assert recorder.requests[0].path == "/v2/users/me"
    assert recorder.requests[0].headers["authorization"] == "Bearer access-token-1"


def test_local_permission_check_blocks_request_before_transport(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(ME_RESPONSE))
    api = ElementSkinAPI(
        "https://skin.example.test",
        access_token="access-token-1",
        permissions=(ProfileScopes.READ_OWNED,),
        transport=recorder.transport(),
    )

    with pytest.raises(PermissionDenied) as exc:
        api.me()

    assert exc.value.status_code == 403
    assert exc.value.detail == "missing required permission: account.read.self"
    assert exc.value.response_body == {
        "detail": "missing required permission",
        "missing_permissions": ["account.read.self"],
    }
    assert recorder.requests == []


def test_email_change_uses_exact_permission_paths_and_bodies(response_json) -> None:
    responses = [
        response_json({"ttl": 300}),
        httpx.Response(204),
    ]
    recorder = RequestRecorder(lambda request: responses.pop(0))
    api = ElementSkinAPI(
        "https://skin.example.test",
        access_token="access-token-1",
        permissions=(AccountScopes.UPDATE_SELF,),
        transport=recorder.transport(),
    )

    sent = api.request_email_change_code("new@example.com")
    changed = api.change_email("new@example.com", "EMAIL123")

    assert sent == {"ttl": 300}
    assert changed is None
    assert [(request.method, request.path, request.json_body) for request in recorder.requests] == [
        ("POST", "/v2/users/me/email/verification-code", {"email": "new@example.com"}),
        ("PUT", "/v2/users/me/email", {"email": "new@example.com", "code": "EMAIL123"}),
    ]


def test_email_change_permission_check_blocks_both_requests(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json({"ok": True}))
    api = ElementSkinAPI(
        "https://skin.example.test",
        access_token="access-token-1",
        permissions=(AccountScopes.READ_SELF,),
        transport=recorder.transport(),
    )

    for call in (
        lambda: api.request_email_change_code("new@example.com"),
        lambda: api.change_email("new@example.com", "EMAIL123"),
    ):
        with pytest.raises(PermissionDenied) as exc:
            call()
        assert exc.value.status_code == 403
        assert exc.value.detail == "missing required permission: account.update.self"
    assert recorder.requests == []


def test_list_profiles_uses_exact_cursor_params(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(PROFILE_PAGE_RESPONSE))
    api = ElementSkinAPI(
        "https://skin.example.test",
        access_token="access-token-1",
        permissions=(ProfileScopes.READ_OWNED,),
        transport=recorder.transport(),
    )

    body = api.list_profiles(cursor="cursor-1", page_size=20)

    assert body == PROFILE_PAGE_RESPONSE
    assert recorder.requests[0].path == "/v2/users/me/profiles"
    assert recorder.requests[0].query == {"cursor": ["cursor-1"], "limit": ["20"]}


def test_profile_mutations_use_exact_methods_paths_and_bodies(response_json) -> None:
    responses = [
        {"id": "profile-1", "name": "Created", "model": "default"},
        None,
        None,
    ]

    def handler(request):
        payload = responses.pop(0)
        if payload is None:
            return response_json({}, 204)
        return response_json(payload)

    recorder = RequestRecorder(handler)
    api = ElementSkinAPI(
        "https://skin.example.test",
        access_token="access-token-1",
        permissions=(
            ProfileScopes.CREATE_OWNED,
            ProfileScopes.UPDATE_OWNED,
            ProfileScopes.DELETE_OWNED,
        ),
        transport=recorder.transport(),
    )

    created = api.create_profile("Created")
    updated = api.update_profile("profile-1", name="Renamed", model="slim")
    deleted = api.delete_profile("profile-1")

    assert created == {"id": "profile-1", "name": "Created", "model": "default"}
    assert updated is None
    assert deleted is None
    assert [(request.method, request.path, request.json_body) for request in recorder.requests] == [
        ("POST", "/v2/users/me/profiles", {"name": "Created", "model": "default"}),
        ("PATCH", "/v2/users/me/profiles/profile-1", {"name": "Renamed", "model": "slim"}),
        ("DELETE", "/v2/users/me/profiles/profile-1", None),
    ]


def test_list_textures_uses_backend_texture_type_param(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json({"items": [], "has_next": False}))
    api = ElementSkinAPI(
        "https://skin.example.test",
        access_token="access-token-1",
        permissions=(TextureScopes.READ_OWNED,),
        transport=recorder.transport(),
    )

    body = api.list_textures(texture_type="skin", cursor="texture-cursor", page_size=10)

    assert body == {"items": [], "has_next": False}
    assert recorder.requests[0].path == "/v2/users/me/textures"
    assert recorder.requests[0].query == {
        "texture_type": ["skin"],
        "cursor": ["texture-cursor"],
        "limit": ["10"],
    }


def test_update_texture_uses_hash_and_type_path_with_exact_body(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json({"hash": "hash-1", "type": "skin"}))
    api = ElementSkinAPI(
        "https://skin.example.test",
        access_token="access-token-1",
        permissions=(TextureScopes.UPDATE_OWNED,),
        transport=recorder.transport(),
    )

    body = api.update_texture("hash-1", "skin", note="A note", model="slim")

    assert body == {"hash": "hash-1", "type": "skin"}
    assert recorder.requests[0].method == "PATCH"
    assert recorder.requests[0].path == "/v2/users/me/textures/hash-1/skin"
    assert recorder.requests[0].json_body == {"note": "A note", "model": "slim"}


def test_texture_delete_wardrobe_and_minecraft_wrappers_use_exact_shapes(response_json) -> None:
    responses = [
        None,
        None,
        None,
        {"id": "profile-1", "name": "Alice"},
        {"profiles": [{"id": "profile-1", "name": "Alice"}]},
    ]

    def handler(request):
        payload = responses.pop(0)
        if payload is None:
            return response_json({}, 204)
        return response_json(payload)

    recorder = RequestRecorder(handler)
    api = ElementSkinAPI(
        "https://skin.example.test",
        access_token="access-token-1",
        permissions=(
            TextureScopes.DELETE_OWNED,
            WardrobeScopes.ENTRY_ADD_OWNED,
            WardrobeScopes.ENTRY_APPLY_OWNED,
            MinecraftScopes.PROFILE_READ_PUBLIC,
        ),
        transport=recorder.transport(),
    )

    assert api.delete_texture("hash-1", "skin") is None
    assert api.add_texture_to_wardrobe("hash-1", texture_type="skin") is None
    assert api.apply_texture("hash-1", profile_id="profile-1", texture_type="skin") is None
    assert api.minecraft_profile("Alice") == {"id": "profile-1", "name": "Alice"}
    assert api.minecraft_profiles(["Alice"]) == {"profiles": [{"id": "profile-1", "name": "Alice"}]}
    assert [
        (request.method, request.path, request.query, request.json_body)
        for request in recorder.requests
    ] == [
        ("DELETE", "/v2/users/me/textures/hash-1/skin", {}, None),
        ("POST", "/v2/users/me/textures/hash-1/wardrobe", {"texture_type": ["skin"]}, None),
        (
            "POST",
            "/v2/users/me/textures/hash-1/apply",
            {},
            {"profile_id": "profile-1", "texture_type": "skin"},
        ),
        ("GET", "/v2/minecraft/profiles/by-name/Alice", {}, None),
        ("POST", "/v2/minecraft/profiles/by-names", {}, {"names": ["Alice"]}),
    ]


def test_minecraft_has_joined_posts_exact_json(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(MINECRAFT_HAS_JOINED_RESPONSE))
    api = ElementSkinAPI(
        "https://skin.example.test",
        token=TokenSet(
            access_token="server-token-1",
            token_type="Bearer",
            expires_in=1800,
            permissions=(MinecraftScopes.SESSION_HASJOINED_SERVER,),
        ),
        transport=recorder.transport(),
    )

    body = api.minecraft_has_joined(username="Alice", server_id="server-hash", ip="127.0.0.1")

    assert body == MINECRAFT_HAS_JOINED_RESPONSE
    assert recorder.requests[0].method == "POST"
    assert recorder.requests[0].path == "/v2/minecraft/session/has-joined"
    assert recorder.requests[0].json_body == {
        "username": "Alice",
        "server_id": "server-hash",
        "ip": "127.0.0.1",
    }


def test_http_error_maps_detail_exactly(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json({"detail": "texture not found"}, 404))
    api = ElementSkinAPI(
        "https://skin.example.test",
        access_token="access-token-1",
        permissions=(TextureScopes.READ_OWNED,),
        transport=recorder.transport(),
    )

    with pytest.raises(APIError) as exc:
        api.get_texture("missing-hash", "skin")

    assert exc.value.status_code == 404
    assert exc.value.detail == "texture not found"
    assert exc.value.response_body == {"detail": "texture not found"}


def test_api_context_manager_closes_owned_client(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(ME_RESPONSE))

    with ElementSkinAPI(
        "https://skin.example.test",
        access_token="access-token-1",
        permissions=(AccountScopes.READ_SELF,),
        transport=recorder.transport(),
    ) as api:
        assert api.me() == ME_RESPONSE

    assert len(recorder.requests) == 1


def test_missing_permission_metadata_allows_request_when_token_permissions_unknown(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(ME_RESPONSE))
    api = ElementSkinAPI(
        "https://skin.example.test",
        access_token="access-token-1",
        transport=recorder.transport(),
    )

    assert api.me() == ME_RESPONSE
    assert recorder.requests[0].path == "/v2/users/me"
