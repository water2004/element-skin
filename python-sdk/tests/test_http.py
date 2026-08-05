from __future__ import annotations

import httpx
import pytest

from element_skin_sdk.exceptions import APIError, AuthenticationError, NotFound, OAuthError, PermissionDenied
from element_skin_sdk.http import HTTPClient

from .conftest import RequestRecorder


def test_http_context_manager_returns_self_and_gets_json(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json({"ok": True}))

    with HTTPClient("https://skin.example.test", transport=recorder.transport()) as client:
        assert client.get("/v2/ping") == {"ok": True}

    assert recorder.requests[0].method == "GET"
    assert recorder.requests[0].path == "/v2/ping"


def test_http_close_does_not_close_external_client(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json({"ok": True}))
    external = httpx.Client(base_url="https://skin.example.test", transport=recorder.transport())
    client = HTTPClient("https://skin.example.test", client=external)

    client.close()
    body = external.get("/v2/ping").json()
    external.close()

    assert body == {"ok": True}
    assert recorder.requests[0].path == "/v2/ping"


def test_http_put_and_delete_helpers_use_exact_methods(response_json) -> None:
    responses = [response_json({"ok": True}), httpx.Response(204)]
    recorder = RequestRecorder(lambda request: responses.pop(0))
    client = HTTPClient("https://skin.example.test", transport=recorder.transport())

    assert client.put("/v2/resource", json={"value": "new"}) == {"ok": True}
    assert client.delete("/v2/resource") is None

    assert [(request.method, request.path, request.json_body) for request in recorder.requests] == [
        ("PUT", "/v2/resource", {"value": "new"}),
        ("DELETE", "/v2/resource", None),
    ]


@pytest.mark.parametrize(
    ("status_code", "payload", "expected_cls", "expected_detail"),
    [
        (401, {"detail": "missing token"}, AuthenticationError, "missing token"),
        (403, {"detail": "forbidden"}, PermissionDenied, "forbidden"),
        (404, {"detail": "missing"}, NotFound, "missing"),
        (409, {"detail": "conflict"}, APIError, "conflict"),
        (500, {"message": "boom"}, APIError, "Internal Server Error"),
    ],
)
def test_http_error_mapping_for_site_api(
    response_json,
    status_code: int,
    payload: dict[str, str],
    expected_cls: type[APIError],
    expected_detail: str,
) -> None:
    recorder = RequestRecorder(lambda request: response_json(payload, status_code))
    client = HTTPClient("https://skin.example.test", transport=recorder.transport())

    with pytest.raises(expected_cls) as exc:
        client.get("/v2/failure")

    assert exc.value.status_code == status_code
    assert exc.value.detail == expected_detail
    assert exc.value.response_body == payload


def test_http_error_mapping_for_plain_text_response() -> None:
    recorder = RequestRecorder(lambda request: httpx.Response(418, text="plain failure"))
    client = HTTPClient("https://skin.example.test", transport=recorder.transport())

    with pytest.raises(APIError) as exc:
        client.get("/v2/plain")

    assert exc.value.status_code == 418
    assert exc.value.detail == "I'm a teapot"
    assert exc.value.response_body == "plain failure"


def test_http_oauth_error_uses_error_when_description_missing(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json({"error": "invalid_client"}, 401))
    client = HTTPClient("https://skin.example.test", transport=recorder.transport())

    with pytest.raises(OAuthError) as exc:
        client.post("/oauth/token", oauth_error=True)

    assert exc.value.status_code == 401
    assert exc.value.error == "invalid_client"
    assert exc.value.detail == "invalid_client"
    assert exc.value.response_body == {"error": "invalid_client"}
