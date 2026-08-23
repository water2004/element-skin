"""Invite-code transport helpers."""

from __future__ import annotations

from base64 import urlsafe_b64encode


def encode_invite_code(code: str) -> str:
    """Encode an invite code as UTF-8, unpadded Base64URL for `/v2` transport."""

    return urlsafe_b64encode(code.encode("utf-8")).rstrip(b"=").decode("ascii")
