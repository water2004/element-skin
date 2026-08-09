from __future__ import annotations

import hashlib
import hmac
import json
from concurrent.futures import ThreadPoolExecutor
from typing import Any

import pytest

from element_skin_sdk import (
    MemoryReplayGuard as ExportedMemoryReplayGuard,
    ReplayGuard as ExportedReplayGuard,
    WebhookError,
    WebhookEvent as ExportedWebhookEvent,
    WebhookHeaderError,
    WebhookHeaders as ExportedWebhookHeaders,
    WebhookPayloadError,
    WebhookReplayError,
    WebhookSignatureError,
    WebhookTimestampError,
    WebhookVerifier as ExportedWebhookVerifier,
)
from element_skin_sdk.exceptions import ValidationError
from element_skin_sdk.webhook import (
    MemoryReplayGuard,
    ReplayGuard,
    WebhookEvent,
    WebhookHeaders,
    WebhookVerifier,
)

from .fixtures import WEBHOOK_PAYLOAD, WEBHOOK_SIGNING_SECRET, WEBHOOK_TIMESTAMP


def webhook_body(payload: object = WEBHOOK_PAYLOAD) -> bytes:
    return json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode(
        "utf-8"
    )


def webhook_signature(body: bytes, timestamp: int = WEBHOOK_TIMESTAMP) -> str:
    digest = hmac.new(
        WEBHOOK_SIGNING_SECRET.encode(),
        str(timestamp).encode() + b"." + body,
        hashlib.sha256,
    ).hexdigest()
    return f"v1={digest}"


def webhook_headers(
    body: bytes,
    *,
    timestamp: int = WEBHOOK_TIMESTAMP,
    event_id: str = "evt_profile_updated_1",
) -> dict[str, str]:
    return {
        "Webhook-Id": event_id,
        "Webhook-Delivery": "whd_delivery_1",
        "Webhook-Timestamp": str(timestamp),
        "Webhook-Signature": webhook_signature(body, timestamp),
        "Content-Type": "application/json",
    }


def assert_webhook_error(error: pytest.ExceptionInfo[Exception], message: str) -> None:
    assert str(error.value) == message


def test_webhook_public_exports_and_valid_event_are_exact() -> None:
    body = webhook_body()
    headers = webhook_headers(body)
    headers["Webhook-Signature"] = (
        headers["Webhook-Signature"][:3] + headers["Webhook-Signature"][3:].upper()
    )
    verifier = WebhookVerifier(WEBHOOK_SIGNING_SECRET, clock=lambda: WEBHOOK_TIMESTAMP)

    event = verifier.verify(bytearray(body), headers)

    assert event == WebhookEvent(
        id="evt_profile_updated_1",
        type="profile.updated",
        created_at=1_786_118_399_000,
        data={"profile_id": "profile-uuid", "user_id": "user-uuid"},
        delivery_id="whd_delivery_1",
        timestamp=WEBHOOK_TIMESTAMP,
    )
    assert verifier.tolerance_ms == 300_000
    assert verifier.max_body_bytes == 65_536
    assert ExportedWebhookVerifier is WebhookVerifier
    assert ExportedWebhookEvent is WebhookEvent
    assert ExportedMemoryReplayGuard is MemoryReplayGuard
    assert ExportedReplayGuard is ReplayGuard
    assert ExportedWebhookHeaders is WebhookHeaders
    assert issubclass(WebhookError, ValidationError)


def test_webhook_verifier_accepts_bytes_secret_memoryview_and_boundary_timestamp() -> (
    None
):
    body = webhook_body()
    verifier = WebhookVerifier(
        WEBHOOK_SIGNING_SECRET.encode(),
        tolerance_seconds=2.5,
        clock=lambda: 0,
    )

    event = verifier.verify(
        memoryview(body), webhook_headers(body), now_ms=WEBHOOK_TIMESTAMP + 2_500
    )

    assert event.timestamp == WEBHOOK_TIMESTAMP
    assert verifier.tolerance_ms == 2_500


@pytest.mark.parametrize(
    ("secret", "error_type", "message"),
    [
        ("", ValueError, "signing_secret must not be empty"),
        (b"", ValueError, "signing_secret must not be empty"),
        (123, TypeError, "signing_secret must be str or bytes"),
    ],
)
def test_webhook_verifier_rejects_invalid_secrets_exactly(
    secret: object,
    error_type: type[Exception],
    message: str,
) -> None:
    with pytest.raises(error_type) as error:
        WebhookVerifier(secret)  # type: ignore[arg-type]

    assert_webhook_error(error, message)


@pytest.mark.parametrize("tolerance", [-1, float("inf"), float("nan"), True, "300"])
def test_webhook_verifier_rejects_invalid_tolerance_exactly(tolerance: object) -> None:
    with pytest.raises(ValueError) as error:
        WebhookVerifier(WEBHOOK_SIGNING_SECRET, tolerance_seconds=tolerance)  # type: ignore[arg-type]

    assert_webhook_error(
        error, "tolerance_seconds must be a finite non-negative number"
    )


@pytest.mark.parametrize("max_body_bytes", [0, -1, True, 1.5, "65536"])
def test_webhook_verifier_rejects_invalid_body_limit_exactly(
    max_body_bytes: object,
) -> None:
    with pytest.raises(ValueError) as error:
        WebhookVerifier(WEBHOOK_SIGNING_SECRET, max_body_bytes=max_body_bytes)  # type: ignore[arg-type]

    assert_webhook_error(error, "max_body_bytes must be a positive integer")


def test_webhook_verifier_rejects_invalid_body_and_headers_collection() -> None:
    verifier = WebhookVerifier(WEBHOOK_SIGNING_SECRET)

    with pytest.raises(TypeError) as body_error:
        verifier.verify("{}", {})  # type: ignore[arg-type]
    assert_webhook_error(body_error, "raw_body must be bytes-like")

    with pytest.raises(TypeError) as headers_error:
        verifier.verify(b"{}", [])  # type: ignore[arg-type]
    assert_webhook_error(headers_error, "headers must provide an items() method")


class FrameworkHeaders:
    """Models Flask/Werkzeug's structural, non-Mapping headers collection."""

    def __init__(self, headers: dict[str, str]):
        self.headers = headers

    def items(self) -> list[tuple[str, str]]:
        return list(self.headers.items())


def test_webhook_verifier_accepts_structural_framework_headers() -> None:
    body = webhook_body()
    headers = FrameworkHeaders(webhook_headers(body))
    verifier = WebhookVerifier(WEBHOOK_SIGNING_SECRET)

    event = verifier.verify(body, headers, now_ms=WEBHOOK_TIMESTAMP)

    assert event.id == "evt_profile_updated_1"


def test_webhook_verifier_rejects_oversized_body_before_authentication_exactly() -> (
    None
):
    verifier = WebhookVerifier(WEBHOOK_SIGNING_SECRET, max_body_bytes=2)

    with pytest.raises(WebhookPayloadError) as error:
        verifier.verify(b"{}\n", {})

    assert_webhook_error(error, "Webhook payload exceeds max_body_bytes")


@pytest.mark.parametrize(
    ("headers", "message"),
    [
        ({1: "value"}, "Webhook header names and values must be strings"),
        ({"Webhook-Id": 1}, "Webhook header names and values must be strings"),
        (
            {"Webhook-Id": "evt_1", "webhook-id": "evt_1"},
            "duplicate Webhook header: webhook-id",
        ),
        ({}, "missing or empty Webhook header: webhook-id"),
        ({"Webhook-Id": ""}, "missing or empty Webhook header: webhook-id"),
    ],
)
def test_webhook_verifier_rejects_invalid_headers_exactly(
    headers: dict[Any, Any], message: str
) -> None:
    verifier = WebhookVerifier(WEBHOOK_SIGNING_SECRET)

    with pytest.raises(WebhookHeaderError) as error:
        verifier.verify(b"{}", headers)

    assert_webhook_error(error, message)


@pytest.mark.parametrize("timestamp", ["01", "-1", "１", "not-a-timestamp"])
def test_webhook_verifier_rejects_noncanonical_timestamps_exactly(
    timestamp: str,
) -> None:
    body = webhook_body()
    headers = webhook_headers(body)
    headers["Webhook-Timestamp"] = timestamp
    verifier = WebhookVerifier(WEBHOOK_SIGNING_SECRET)

    with pytest.raises(WebhookTimestampError) as error:
        verifier.verify(body, headers, now_ms=WEBHOOK_TIMESTAMP)

    assert_webhook_error(
        error, "Webhook-Timestamp must be a canonical millisecond timestamp"
    )


@pytest.mark.parametrize(
    ("signature", "message"),
    [
        ("v2=" + "0" * 64, "Webhook-Signature must use v1 with a SHA-256 hex digest"),
        ("v1=short", "Webhook-Signature must use v1 with a SHA-256 hex digest"),
        ("v1=" + "g" * 64, "Webhook-Signature contains invalid hexadecimal data"),
        ("v1=" + "0" * 64, "invalid Webhook signature"),
    ],
)
def test_webhook_verifier_rejects_invalid_signatures_exactly(
    signature: str, message: str
) -> None:
    body = webhook_body()
    headers = webhook_headers(body)
    headers["Webhook-Signature"] = signature
    verifier = WebhookVerifier(WEBHOOK_SIGNING_SECRET)

    with pytest.raises(WebhookSignatureError) as error:
        verifier.verify(body, headers, now_ms=WEBHOOK_TIMESTAMP)

    assert_webhook_error(error, message)


@pytest.mark.parametrize(
    "now_ms", [WEBHOOK_TIMESTAMP - 300_001, WEBHOOK_TIMESTAMP + 300_001]
)
def test_webhook_verifier_rejects_past_and_future_timestamps_outside_tolerance(
    now_ms: int,
) -> None:
    body = webhook_body()
    verifier = WebhookVerifier(WEBHOOK_SIGNING_SECRET)

    with pytest.raises(WebhookTimestampError) as error:
        verifier.verify(body, webhook_headers(body), now_ms=now_ms)

    assert_webhook_error(error, "Webhook timestamp is outside the allowed tolerance")


@pytest.mark.parametrize("now_ms", [True, 1.5, "1786118400123"])
def test_webhook_verifier_rejects_invalid_current_time_exactly(now_ms: object) -> None:
    body = webhook_body()
    verifier = WebhookVerifier(WEBHOOK_SIGNING_SECRET)

    with pytest.raises(TypeError) as error:
        verifier.verify(body, webhook_headers(body), now_ms=now_ms)  # type: ignore[arg-type]

    assert_webhook_error(error, "now_ms must be an integer millisecond timestamp")


@pytest.mark.parametrize(
    ("body", "message"),
    [
        (b"\xff", "Webhook payload must be valid UTF-8 JSON"),
        (b"{", "Webhook payload must be valid UTF-8 JSON"),
        (b'{"id":NaN}', "Webhook payload must be valid UTF-8 JSON"),
        (b"[]", "Webhook payload must be a JSON object"),
        (
            webhook_body({"type": "profile.updated", "created_at": 1, "data": {}}),
            "Webhook payload id must be a non-empty string",
        ),
        (
            webhook_body(
                {"id": 1, "type": "profile.updated", "created_at": 1, "data": {}}
            ),
            "Webhook payload id must be a non-empty string",
        ),
        (
            webhook_body(
                {"id": "", "type": "profile.updated", "created_at": 1, "data": {}}
            ),
            "Webhook payload id must be a non-empty string",
        ),
        (
            webhook_body({"id": "evt_1", "created_at": 1, "data": {}}),
            "Webhook payload type must be a non-empty string",
        ),
        (
            webhook_body(
                {
                    "id": "evt_1",
                    "type": "profile.updated",
                    "created_at": True,
                    "data": {},
                }
            ),
            "Webhook payload created_at must be a non-negative integer",
        ),
        (
            webhook_body(
                {"id": "evt_1", "type": "profile.updated", "created_at": -1, "data": {}}
            ),
            "Webhook payload created_at must be a non-negative integer",
        ),
        (
            webhook_body({"id": "evt_1", "type": "profile.updated", "created_at": 1}),
            "Webhook payload data must be a JSON object",
        ),
        (
            webhook_body(
                {"id": "evt_1", "type": "profile.updated", "created_at": 1, "data": []}
            ),
            "Webhook payload data must be a JSON object",
        ),
    ],
)
def test_webhook_verifier_rejects_authenticated_invalid_payloads_exactly(
    body: bytes, message: str
) -> None:
    verifier = WebhookVerifier(WEBHOOK_SIGNING_SECRET)
    headers = webhook_headers(body, event_id="evt_1")

    with pytest.raises(WebhookPayloadError) as error:
        verifier.verify(body, headers, now_ms=WEBHOOK_TIMESTAMP)

    assert_webhook_error(error, message)


def test_webhook_verifier_rejects_header_payload_event_id_mismatch_exactly() -> None:
    body = webhook_body()
    verifier = WebhookVerifier(WEBHOOK_SIGNING_SECRET)

    with pytest.raises(WebhookPayloadError) as error:
        verifier.verify(
            body,
            webhook_headers(body, event_id="evt_different"),
            now_ms=WEBHOOK_TIMESTAMP,
        )

    assert_webhook_error(error, "Webhook-Id does not match payload id")


class RecordingReplayGuard:
    def __init__(self, result: bool):
        self.result = result
        self.calls: list[tuple[WebhookEvent, int]] = []

    def claim(self, event: WebhookEvent, expires_at_ms: int) -> bool:
        self.calls.append((event, expires_at_ms))
        return self.result


def test_webhook_verify_and_claim_uses_exact_delivery_and_expiration() -> None:
    body = webhook_body()
    verifier = WebhookVerifier(WEBHOOK_SIGNING_SECRET, tolerance_seconds=60)
    guard = RecordingReplayGuard(True)

    event = verifier.verify_and_claim(
        body,
        webhook_headers(body),
        guard,
        now_ms=WEBHOOK_TIMESTAMP,
    )

    assert event.delivery_id == "whd_delivery_1"
    assert guard.calls == [(event, WEBHOOK_TIMESTAMP + 60_000)]


def test_webhook_verify_and_claim_rejects_replayed_delivery_exactly() -> None:
    body = webhook_body()
    verifier = WebhookVerifier(WEBHOOK_SIGNING_SECRET)
    guard = RecordingReplayGuard(False)

    with pytest.raises(WebhookReplayError) as error:
        verifier.verify_and_claim(
            body,
            webhook_headers(body),
            guard,
            now_ms=WEBHOOK_TIMESTAMP,
        )

    assert_webhook_error(error, "Webhook delivery has already been claimed")
    assert len(guard.calls) == 1
    assert guard.calls[0][0].delivery_id == "whd_delivery_1"
    assert guard.calls[0][1] == WEBHOOK_TIMESTAMP + 300_000


def test_memory_replay_guard_rejects_duplicates_until_expired_exactly() -> None:
    current_time = [WEBHOOK_TIMESTAMP]
    guard = MemoryReplayGuard(clock=lambda: current_time[0])
    event = WebhookEvent(
        id="evt_1",
        type="profile.updated",
        created_at=WEBHOOK_TIMESTAMP,
        data={},
        delivery_id="whd_1",
        timestamp=WEBHOOK_TIMESTAMP,
    )

    assert guard.claim(event, WEBHOOK_TIMESTAMP + 1_000) is True
    assert guard.claim(event, WEBHOOK_TIMESTAMP + 2_000) is False
    current_time[0] = WEBHOOK_TIMESTAMP + 1_000
    assert guard.claim(event, WEBHOOK_TIMESTAMP + 2_000) is False
    current_time[0] = WEBHOOK_TIMESTAMP + 1_001
    assert guard.claim(event, WEBHOOK_TIMESTAMP + 2_000) is True


def test_memory_replay_guard_claim_is_atomic_across_threads() -> None:
    guard = MemoryReplayGuard(clock=lambda: 0)
    event = WebhookEvent("evt_1", "profile.updated", 0, {}, "whd_concurrent", 0)

    with ThreadPoolExecutor(max_workers=16) as executor:
        results = list(executor.map(lambda _: guard.claim(event, 1_000), range(64)))

    assert results.count(True) == 1
    assert results.count(False) == 63


def test_memory_replay_guard_default_clock_accepts_new_delivery() -> None:
    guard = MemoryReplayGuard()
    event = WebhookEvent("evt_1", "profile.updated", 0, {}, "whd_default_clock", 0)

    assert guard.claim(event, 10**18) is True
