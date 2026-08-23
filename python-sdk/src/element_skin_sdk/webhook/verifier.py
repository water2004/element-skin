"""Framework-independent Element Skin Webhook verification."""

from __future__ import annotations

import hashlib
import hmac
import json
import math
import time
from collections.abc import Callable, Iterable
from dataclasses import dataclass
from heapq import heappop, heappush
from threading import Lock
from typing import Any, Protocol

from ..exceptions import (
    WebhookHeaderError,
    WebhookPayloadError,
    WebhookReplayError,
    WebhookSignatureError,
    WebhookTimestampError,
)

_EVENT_ID_HEADER = "webhook-id"
_DELIVERY_ID_HEADER = "webhook-delivery"
_TIMESTAMP_HEADER = "webhook-timestamp"
_SIGNATURE_HEADER = "webhook-signature"
_REQUIRED_HEADERS = (
    _EVENT_ID_HEADER,
    _DELIVERY_ID_HEADER,
    _TIMESTAMP_HEADER,
    _SIGNATURE_HEADER,
)


@dataclass(frozen=True)
class WebhookEvent:
    """An authenticated Webhook event and its delivery metadata."""

    id: str
    type: str
    created_at: int
    data: dict[str, Any]
    delivery_id: str
    timestamp: int


class WebhookHeaders(Protocol):
    """Structural request-header interface supported by the verifier."""

    def items(self) -> Iterable[tuple[str, str]]:
        """Return header name/value pairs."""


class ReplayGuard(Protocol):
    """Atomic delivery-claim contract used by ``verify_and_claim``."""

    def claim(self, event: WebhookEvent, expires_at_ms: int) -> bool:
        """Atomically accept ``event`` and return ``True`` only on the first claim."""


class MemoryReplayGuard:
    """Thread-safe, process-local replay guard for tests and single-process receivers."""

    def __init__(self, *, clock: Callable[[], int] | None = None):
        self._clock = clock or _now_ms
        self._entries: dict[str, int] = {}
        self._expirations: list[tuple[int, str]] = []
        self._lock = Lock()

    def claim(self, event: WebhookEvent, expires_at_ms: int) -> bool:
        now = self._clock()
        delivery_id = event.delivery_id
        with self._lock:
            while self._expirations and self._expirations[0][0] < now:
                _, expired_delivery_id = heappop(self._expirations)
                del self._entries[expired_delivery_id]
            if delivery_id in self._entries:
                return False
            self._entries[delivery_id] = expires_at_ms
            heappush(self._expirations, (expires_at_ms, delivery_id))
            return True


class WebhookVerifier:
    """Verify HMAC signatures, timestamp freshness, headers, and event payloads."""

    def __init__(
        self,
        signing_secret: str | bytes,
        *,
        tolerance_seconds: float = 300,
        max_body_bytes: int = 65_536,
        clock: Callable[[], int] | None = None,
    ):
        if isinstance(signing_secret, str):
            secret = signing_secret.encode("utf-8")
        elif isinstance(signing_secret, bytes):
            secret = signing_secret
        else:
            raise TypeError("signing_secret must be str or bytes")
        if not secret:
            raise ValueError("signing_secret must not be empty")
        if (
            isinstance(tolerance_seconds, bool)
            or not isinstance(tolerance_seconds, (int, float))
            or not math.isfinite(tolerance_seconds)
            or tolerance_seconds < 0
        ):
            raise ValueError("tolerance_seconds must be a finite non-negative number")
        if (
            isinstance(max_body_bytes, bool)
            or not isinstance(max_body_bytes, int)
            or max_body_bytes <= 0
        ):
            raise ValueError("max_body_bytes must be a positive integer")
        self._secret = secret
        self._tolerance_ms = int(tolerance_seconds * 1000)
        self._max_body_bytes = max_body_bytes
        self._clock = clock or _now_ms

    @property
    def tolerance_ms(self) -> int:
        return self._tolerance_ms

    @property
    def max_body_bytes(self) -> int:
        return self._max_body_bytes

    def verify(
        self,
        raw_body: bytes | bytearray | memoryview,
        headers: WebhookHeaders,
        *,
        now_ms: int | None = None,
    ) -> WebhookEvent:
        """Authenticate and parse one request without claiming replay state."""

        if not isinstance(raw_body, (bytes, bytearray, memoryview)):
            raise TypeError("raw_body must be bytes-like")
        body = bytes(raw_body)
        if len(body) > self._max_body_bytes:
            raise WebhookPayloadError("Webhook payload exceeds max_body_bytes")
        normalized_headers = _normalize_headers(headers)
        timestamp_text = normalized_headers[_TIMESTAMP_HEADER]
        timestamp = _parse_timestamp(timestamp_text)
        provided_signature = _parse_signature(normalized_headers[_SIGNATURE_HEADER])

        expected_signature = hmac.new(
            self._secret,
            timestamp_text.encode("ascii") + b"." + body,
            hashlib.sha256,
        ).digest()
        if not hmac.compare_digest(provided_signature, expected_signature):
            raise WebhookSignatureError("invalid Webhook signature")

        current_time = self._clock() if now_ms is None else now_ms
        if isinstance(current_time, bool) or not isinstance(current_time, int):
            raise TypeError("now_ms must be an integer millisecond timestamp")
        if abs(current_time - timestamp) > self._tolerance_ms:
            raise WebhookTimestampError(
                "Webhook timestamp is outside the allowed tolerance"
            )

        payload = _parse_payload(body)
        event_id = normalized_headers[_EVENT_ID_HEADER]
        if payload["id"] != event_id:
            raise WebhookPayloadError("Webhook-Id does not match payload id")
        return WebhookEvent(
            id=event_id,
            type=payload["type"],
            created_at=payload["created_at"],
            data=payload["data"],
            delivery_id=normalized_headers[_DELIVERY_ID_HEADER],
            timestamp=timestamp,
        )

    def verify_and_claim(
        self,
        raw_body: bytes | bytearray | memoryview,
        headers: WebhookHeaders,
        replay_guard: ReplayGuard,
        *,
        now_ms: int | None = None,
    ) -> WebhookEvent:
        """Verify a request and atomically pass its event to ``replay_guard``."""

        event = self.verify(raw_body, headers, now_ms=now_ms)
        expires_at = event.timestamp + self._tolerance_ms
        if not replay_guard.claim(event, expires_at):
            raise WebhookReplayError("Webhook delivery has already been claimed")
        return event


def _normalize_headers(headers: WebhookHeaders) -> dict[str, str]:
    items = getattr(headers, "items", None)
    if not callable(items):
        raise TypeError("headers must provide an items() method")
    normalized: dict[str, str] = {}
    for name, value in items():
        if not isinstance(name, str) or not isinstance(value, str):
            raise WebhookHeaderError("Webhook header names and values must be strings")
        lower_name = name.lower()
        if lower_name in normalized:
            raise WebhookHeaderError(f"duplicate Webhook header: {name}")
        normalized[lower_name] = value
    for name in _REQUIRED_HEADERS:
        if not normalized.get(name):
            raise WebhookHeaderError(f"missing or empty Webhook header: {name}")
    return normalized


def _parse_timestamp(value: str) -> int:
    if (
        not value.isascii()
        or not value.isdigit()
        or (len(value) > 1 and value.startswith("0"))
    ):
        raise WebhookTimestampError(
            "Webhook-Timestamp must be a canonical millisecond timestamp"
        )
    return int(value)


def _parse_signature(value: str) -> bytes:
    if not value.startswith("v1=") or len(value) != 67:
        raise WebhookSignatureError(
            "Webhook-Signature must use v1 with a SHA-256 hex digest"
        )
    try:
        return bytes.fromhex(value[3:])
    except ValueError as error:
        raise WebhookSignatureError(
            "Webhook-Signature contains invalid hexadecimal data"
        ) from error


def _parse_payload(body: bytes) -> dict[str, Any]:
    try:
        payload = json.loads(body, parse_constant=_reject_json_constant)
    except (UnicodeDecodeError, ValueError) as error:
        raise WebhookPayloadError("Webhook payload must be valid UTF-8 JSON") from error
    if not isinstance(payload, dict):
        raise WebhookPayloadError("Webhook payload must be a JSON object")
    event_id = _payload_string(payload, "id")
    event_type = _payload_string(payload, "type")
    created_at = payload.get("created_at")
    if (
        isinstance(created_at, bool)
        or not isinstance(created_at, int)
        or created_at < 0
    ):
        raise WebhookPayloadError(
            "Webhook payload created_at must be a non-negative integer"
        )
    data = payload.get("data")
    if not isinstance(data, dict):
        raise WebhookPayloadError("Webhook payload data must be a JSON object")
    return {"id": event_id, "type": event_type, "created_at": created_at, "data": data}


def _payload_string(payload: dict[str, Any], name: str) -> str:
    value = payload.get(name)
    if not isinstance(value, str) or not value:
        raise WebhookPayloadError(f"Webhook payload {name} must be a non-empty string")
    return value


def _now_ms() -> int:
    return time.time_ns() // 1_000_000


def _reject_json_constant(value: str) -> None:
    raise ValueError(f"invalid JSON constant: {value}")
