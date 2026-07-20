from __future__ import annotations

from urllib.parse import parse_qs, urlparse

import httpx
import pytest

from element_skin_sdk import MemoryTokenStore, OAuthClient
from element_skin_sdk.exceptions import InvalidScope, OAuthError
from element_skin_sdk.permissions import AccountScopes, InviteScopes, MinecraftScopes, ProfileScopes

from .conftest import RequestRecorder
from .fixtures import CLIENT_CREDENTIALS_TOKEN_RESPONSE, DEVICE_CODE_RESPONSE, TOKEN_RESPONSE


def test_authorization_url_builds_pkce_request_with_exact_query() -> None:
    oauth = OAuthClient(
        "https://skin.example.test/",
        "client-1",
        redirect_uri="https://app.example.test/callback",
    )

    session = oauth.authorization_url(
        [AccountScopes.READ_SELF, ProfileScopes.READ_OWNED],
        state="state-1",
        code_verifier="a" * 64,
    )

    parsed = urlparse(session.authorization_url)
    query = parse_qs(parsed.query)
    assert parsed.scheme == "https"
    assert parsed.netloc == "skin.example.test"
    assert parsed.path == "/oauth/authorize"
    assert query == {
        "response_type": ["code"],
        "client_id": ["client-1"],
        "redirect_uri": ["https://app.example.test/callback"],
        "scope": ["account.read.self profile.read.owned"],
        "state": ["state-1"],
        "code_challenge": [session.code_challenge],
        "code_challenge_method": ["S256"],
    }
    assert session.code_verifier == "a" * 64
    assert session.scopes == ("account.read.self", "profile.read.owned")


def test_authorization_url_rejects_server_scope_for_delegated_flow() -> None:
    oauth = OAuthClient("https://skin.example.test", "client-1", redirect_uri="https://app/cb")

    with pytest.raises(InvalidScope) as exc:
        oauth.authorization_url([MinecraftScopes.SESSION_HASJOINED_SERVER], state="state-1")

    assert exc.value.invalid_scopes == ["minecraft_session.hasjoined.server"]
    assert str(exc.value) == "authorization code and device flows cannot request server or system scopes"


def test_authorization_url_requires_redirect_uri() -> None:
    oauth = OAuthClient("https://skin.example.test", "client-1")

    with pytest.raises(ValueError) as exc:
        oauth.authorization_url([AccountScopes.READ_SELF], state="state-1")

    assert str(exc.value) == "redirect_uri is required for authorization code flow"


def test_authorization_info_and_approval_use_exact_authenticated_routes(response_json) -> None:
    request_params = {
        "response_type": "code",
        "client_id": "client-1",
        "redirect_uri": "https://app.example.test/callback",
        "scope": AccountScopes.READ_SELF,
        "state": "state-1",
        "code_challenge": "challenge-1",
        "code_challenge_method": "S256",
    }
    responses = [
        {
            "client": {"client_id": "client-1"},
            "scopes": [{"code": "account.read.self"}],
            "redirect_uri": "https://app.example.test/callback",
            "state": "state-1",
        },
        {
            "code": "code-1",
            "redirect_url": "https://app.example.test/callback?code=code-1&state=state-1",
            "state": "state-1",
        },
    ]

    def handler(request):
        return response_json(responses.pop(0))

    recorder = RequestRecorder(handler)
    oauth = OAuthClient(
        "https://skin.example.test",
        "client-1",
        access_token="user-access-token",
        transport=recorder.transport(),
    )

    info = oauth.authorization_info(request_params)
    approval = oauth.approve_authorization(request_params)

    assert info == {
        "client": {"client_id": "client-1"},
        "scopes": [{"code": "account.read.self"}],
        "redirect_uri": "https://app.example.test/callback",
        "state": "state-1",
    }
    assert approval == {
        "code": "code-1",
        "redirect_url": "https://app.example.test/callback?code=code-1&state=state-1",
        "state": "state-1",
    }
    assert [(request.method, request.path) for request in recorder.requests] == [
        ("GET", "/oauth/authorize"),
        ("POST", "/oauth/authorize"),
    ]
    assert recorder.requests[0].headers["authorization"] == "Bearer user-access-token"
    assert recorder.requests[0].query == {
        "client_id": ["client-1"],
        "redirect_uri": ["https://app.example.test/callback"],
        "scope": ["account.read.self"],
        "state": ["state-1"],
        "response_type": ["code"],
        "code_challenge": ["challenge-1"],
        "code_challenge_method": ["S256"],
    }
    assert recorder.requests[1].json_body == request_params


def test_exchange_code_posts_exact_form_and_saves_tokens(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(TOKEN_RESPONSE))
    store = MemoryTokenStore()
    oauth = OAuthClient(
        "https://skin.example.test",
        "client-1",
        redirect_uri="https://app.example.test/callback",
        client_secret="secret-1",
        token_store=store,
        transport=recorder.transport(),
    )

    tokens = oauth.exchange_code(code="auth-code-1", code_verifier="verifier-1")

    assert tokens.access_token == "access-token-1"
    assert tokens.refresh_token == "refresh-token-1"
    assert tokens.permissions == ("account.read.self", "profile.read.owned")
    assert store.load() == tokens
    assert len(recorder.requests) == 1
    request = recorder.requests[0]
    assert request.method == "POST"
    assert request.path == "/oauth/token"
    assert request.form == {
        "grant_type": ["authorization_code"],
        "client_id": ["client-1"],
        "code": ["auth-code-1"],
        "code_verifier": ["verifier-1"],
        "redirect_uri": ["https://app.example.test/callback"],
        "client_secret": ["secret-1"],
    }


def test_exchange_code_requires_redirect_uri_before_request() -> None:
    recorder = RequestRecorder(lambda request: pytest.fail("token endpoint must not be called"))
    oauth = OAuthClient(
        "https://skin.example.test",
        "client-1",
        transport=recorder.transport(),
    )

    with pytest.raises(ValueError) as exc:
        oauth.exchange_code(code="auth-code-1", code_verifier="verifier-1")

    assert str(exc.value) == "redirect_uri is required for authorization code flow"
    assert recorder.requests == []


def test_refresh_posts_exact_form_without_scope_when_not_requested(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(TOKEN_RESPONSE))
    oauth = OAuthClient("https://skin.example.test", "client-1", transport=recorder.transport())

    tokens = oauth.refresh("refresh-token-1", store=False)

    assert tokens.access_token == "access-token-1"
    assert recorder.requests[0].form == {
        "grant_type": ["refresh_token"],
        "client_id": ["client-1"],
        "refresh_token": ["refresh-token-1"],
    }


def test_refresh_with_scope_saves_tokens_and_posts_exact_scope(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(TOKEN_RESPONSE))
    store = MemoryTokenStore()
    oauth = OAuthClient(
        "https://skin.example.test",
        "client-1",
        token_store=store,
        transport=recorder.transport(),
    )

    tokens = oauth.refresh("refresh-token-1", scopes=[AccountScopes.READ_SELF])

    assert store.load() == tokens
    assert recorder.requests[0].form == {
        "grant_type": ["refresh_token"],
        "client_id": ["client-1"],
        "refresh_token": ["refresh-token-1"],
        "scope": ["account.read.self"],
    }


def test_start_device_flow_posts_exact_form(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(DEVICE_CODE_RESPONSE))
    oauth = OAuthClient("https://skin.example.test", "client-1", transport=recorder.transport())

    device = oauth.start_device_flow([ProfileScopes.READ_OWNED])

    assert device.device_code == "device-code-1"
    assert device.verification_uri_complete == "https://skin.example.test/oauth/device?user_code=ABCD-EFGH"
    assert device.permissions == ("profile.read.owned",)
    assert recorder.requests[0].path == "/oauth/device/code"
    assert recorder.requests[0].form == {
        "client_id": ["client-1"],
        "scope": ["profile.read.owned"],
    }


def test_exchange_device_code_maps_oauth_error_exactly(response_json) -> None:
    recorder = RequestRecorder(
        lambda request: response_json(
            {"error": "authorization_pending", "error_description": "authorization pending"},
            400,
        )
    )
    oauth = OAuthClient("https://skin.example.test", "client-1", transport=recorder.transport())

    with pytest.raises(OAuthError) as exc:
        oauth.exchange_device_code("device-code-1")

    assert exc.value.status_code == 400
    assert exc.value.error == "authorization_pending"
    assert exc.value.detail == "authorization pending"
    assert recorder.requests[0].form == {
        "grant_type": ["urn:ietf:params:oauth:grant-type:device_code"],
        "client_id": ["client-1"],
        "device_code": ["device-code-1"],
    }


def test_exchange_device_code_saves_tokens(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(TOKEN_RESPONSE))
    store = MemoryTokenStore()
    oauth = OAuthClient(
        "https://skin.example.test",
        "client-1",
        token_store=store,
        transport=recorder.transport(),
    )

    tokens = oauth.exchange_device_code("device-code-1")

    assert store.load() == tokens
    assert recorder.requests[0].form == {
        "grant_type": ["urn:ietf:params:oauth:grant-type:device_code"],
        "client_id": ["client-1"],
        "device_code": ["device-code-1"],
    }


def test_client_credentials_accepts_server_scope_and_posts_exact_form(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(CLIENT_CREDENTIALS_TOKEN_RESPONSE))
    oauth = OAuthClient(
        "https://skin.example.test",
        "server-client",
        client_secret="server-secret",
        transport=recorder.transport(),
    )

    tokens = oauth.client_credentials([MinecraftScopes.SESSION_HASJOINED_SERVER])

    assert tokens.access_token == "server-token-1"
    assert tokens.permissions == ("minecraft_session.hasjoined.server",)
    assert recorder.requests[0].form == {
        "grant_type": ["client_credentials"],
        "client_id": ["server-client"],
        "scope": ["minecraft_session.hasjoined.server"],
        "client_secret": ["server-secret"],
    }


def test_client_credentials_accepts_any_scope_and_posts_exact_form(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(CLIENT_CREDENTIALS_TOKEN_RESPONSE))
    oauth = OAuthClient(
        "https://skin.example.test",
        "admin-tool-client",
        client_secret="admin-tool-secret",
        transport=recorder.transport(),
    )

    tokens = oauth.client_credentials([InviteScopes.READ_ANY, InviteScopes.CREATE_ANY])

    assert tokens.access_token == "server-token-1"
    assert recorder.requests[0].form == {
        "grant_type": ["client_credentials"],
        "client_id": ["admin-tool-client"],
        "scope": ["invite.read.any invite.create.any"],
        "client_secret": ["admin-tool-secret"],
    }


def test_client_credentials_store_false_does_not_save_tokens(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(CLIENT_CREDENTIALS_TOKEN_RESPONSE))
    store = MemoryTokenStore()
    oauth = OAuthClient(
        "https://skin.example.test",
        "server-client",
        client_secret="server-secret",
        token_store=store,
        transport=recorder.transport(),
    )

    tokens = oauth.client_credentials([MinecraftScopes.SESSION_HASJOINED_SERVER], store=False)

    assert tokens.access_token == "server-token-1"
    assert store.load() is None


def test_client_credentials_rejects_user_delegated_scope() -> None:
    oauth = OAuthClient("https://skin.example.test", "server-client")

    with pytest.raises(InvalidScope) as exc:
        oauth.client_credentials([AccountScopes.READ_SELF])

    assert exc.value.invalid_scopes == ["account.read.self"]
    assert str(exc.value) == "client credentials flow can only request public, server, or any scopes"


def test_revoke_posts_exact_form_and_ignores_empty_body() -> None:
    recorder = RequestRecorder(lambda request: httpx.Response(204))
    oauth = OAuthClient(
        "https://skin.example.test",
        "client-1",
        client_secret="secret-1",
        transport=recorder.transport(),
    )

    oauth.revoke("access-token-1")

    assert recorder.requests[0].path == "/oauth/revoke"
    assert recorder.requests[0].form == {
        "token": ["access-token-1"],
        "client_id": ["client-1"],
        "client_secret": ["secret-1"],
    }


def test_introspect_posts_exact_form(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json({"active": True, "client_id": "client-1"}))
    oauth = OAuthClient(
        "https://skin.example.test",
        "client-1",
        client_secret="secret-1",
        transport=recorder.transport(),
    )

    body = oauth.introspect("access-token-1")

    assert body == {"active": True, "client_id": "client-1"}
    assert recorder.requests[0].path == "/oauth/introspect"
    assert recorder.requests[0].form == {
        "token": ["access-token-1"],
        "client_id": ["client-1"],
        "client_secret": ["secret-1"],
    }


def test_oauth_context_manager_closes_wrapped_http_client(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(TOKEN_RESPONSE))

    with OAuthClient(
        "https://skin.example.test",
        "client-1",
        transport=recorder.transport(),
    ) as oauth:
        assert oauth.exchange_code(
            code="auth-code-1",
            code_verifier="verifier-1",
            redirect_uri="https://app.example.test/callback",
            store=False,
        ).access_token == "access-token-1"

    assert len(recorder.requests) == 1


def test_oauth_close_delegates_to_supplied_http_client() -> None:
    class CloseTrackingHTTPClient:
        def __init__(self):
            self.closed = False

        def close(self) -> None:
            self.closed = True

    http_client = CloseTrackingHTTPClient()
    oauth = OAuthClient(
        "https://skin.example.test",
        "client-1",
        http_client=http_client,  # type: ignore[arg-type]
    )

    oauth.close()

    assert http_client.closed is True


def test_poll_device_token_returns_after_pending_response(monkeypatch, response_json) -> None:
    responses = [
        response_json(
            {"error": "authorization_pending", "error_description": "authorization pending"},
            400,
        ),
        response_json(TOKEN_RESPONSE),
    ]
    recorder = RequestRecorder(lambda request: responses.pop(0))
    sleeps: list[int] = []
    monkeypatch.setattr("element_skin_sdk.oauth.client.time.sleep", sleeps.append)
    oauth = OAuthClient("https://skin.example.test", "client-1", transport=recorder.transport())

    tokens = oauth.poll_device_token("device-code-1", interval=2, timeout=30, store=False)

    assert tokens.access_token == "access-token-1"
    assert sleeps == [2]
    assert [request.form["device_code"] for request in recorder.requests] == [
        ["device-code-1"],
        ["device-code-1"],
    ]


def test_poll_device_token_slow_down_times_out_without_sleep(response_json) -> None:
    recorder = RequestRecorder(
        lambda request: response_json(
            {"error": "slow_down", "error_description": "slow down"},
            400,
        )
    )
    oauth = OAuthClient("https://skin.example.test", "client-1", transport=recorder.transport())

    with pytest.raises(TimeoutError) as exc:
        oauth.poll_device_token("device-code-1", interval=1, timeout=0, store=False)

    assert str(exc.value) == "device authorization timed out"
    assert len(recorder.requests) == 1


def test_poll_device_token_reraises_unexpected_oauth_error(response_json) -> None:
    recorder = RequestRecorder(
        lambda request: response_json(
            {"error": "invalid_grant", "error_description": "device code expired"},
            400,
        )
    )
    oauth = OAuthClient("https://skin.example.test", "client-1", transport=recorder.transport())

    with pytest.raises(OAuthError) as exc:
        oauth.poll_device_token("device-code-1", interval=1, timeout=30, store=False)

    assert exc.value.error == "invalid_grant"
    assert exc.value.detail == "device code expired"
