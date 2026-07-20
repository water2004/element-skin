"""OAuth 2.1 client for Element Skin."""

from __future__ import annotations

import secrets
import time
from collections.abc import Iterable
from typing import Any
from urllib.parse import urlencode

import httpx

from ..http import HTTPClient
from ..models import AuthorizationSession, DeviceAuthorization, TokenSet
from ..permissions.validator import PermissionValidator
from .pkce import create_code_challenge, generate_code_verifier
from .token_store import TokenStore


class OAuthClient:
    def __init__(
        self,
        base_url: str,
        client_id: str,
        *,
        redirect_uri: str | None = None,
        client_secret: str | None = None,
        access_token: str | None = None,
        token_store: TokenStore | None = None,
        validator: PermissionValidator | None = None,
        http_client: HTTPClient | None = None,
        transport: httpx.BaseTransport | None = None,
        timeout: float = 10.0,
    ):
        self.base_url = base_url.rstrip("/")
        self.client_id = client_id
        self.redirect_uri = redirect_uri
        self.client_secret = client_secret
        self.token_store = token_store
        self.validator = validator or PermissionValidator()
        self._http = http_client or HTTPClient(
            self.base_url,
            access_token=access_token,
            transport=transport,
            timeout=timeout,
        )

    def close(self) -> None:
        self._http.close()

    def __enter__(self) -> "OAuthClient":
        return self

    def __exit__(self, exc_type: object, exc: object, tb: object) -> None:
        self.close()

    def authorization_url(
        self,
        scopes: Iterable[str] | str,
        *,
        state: str | None = None,
        code_verifier: str | None = None,
        redirect_uri: str | None = None,
    ) -> AuthorizationSession:
        requested_scopes = self.validator.validate_delegated(scopes)
        final_redirect_uri = redirect_uri or self.redirect_uri
        if not final_redirect_uri:
            raise ValueError("redirect_uri is required for authorization code flow")

        verifier = code_verifier or generate_code_verifier()
        challenge = create_code_challenge(verifier)
        final_state = state or secrets.token_urlsafe(24)
        query = urlencode(
            {
                "response_type": "code",
                "client_id": self.client_id,
                "redirect_uri": final_redirect_uri,
                "scope": " ".join(requested_scopes),
                "state": final_state,
                "code_challenge": challenge,
                "code_challenge_method": "S256",
            }
        )

        return AuthorizationSession(
            authorization_url=f"{self.base_url}/oauth/authorize?{query}",
            code_verifier=verifier,
            code_challenge=challenge,
            state=final_state,
            scopes=requested_scopes,
        )

    def authorization_info(self, params: dict[str, Any]) -> dict[str, Any]:
        return self._http.get("/oauth/authorize", params=params)

    def approve_authorization(self, params: dict[str, Any]) -> dict[str, Any]:
        return self._http.post("/oauth/authorize", json=params)

    def exchange_code(
        self,
        *,
        code: str,
        code_verifier: str,
        redirect_uri: str | None = None,
        client_secret: str | None = None,
        store: bool = True,
    ) -> TokenSet:
        final_redirect_uri = redirect_uri or self.redirect_uri
        if not final_redirect_uri:
            raise ValueError("redirect_uri is required for authorization code flow")
        tokens = self._token_request(
            {
                "grant_type": "authorization_code",
                "client_id": self.client_id,
                "code": code,
                "code_verifier": code_verifier,
                "redirect_uri": final_redirect_uri,
                **self._client_secret_payload(client_secret),
            }
        )
        if store:
            self._save_tokens(tokens)
        return tokens

    def refresh(
        self,
        refresh_token: str,
        *,
        scopes: Iterable[str] | str | None = None,
        client_secret: str | None = None,
        store: bool = True,
    ) -> TokenSet:
        requested_scopes = self.validator.validate_delegated(scopes)
        payload: dict[str, Any] = {
            "grant_type": "refresh_token",
            "client_id": self.client_id,
            "refresh_token": refresh_token,
            **self._client_secret_payload(client_secret),
        }
        if requested_scopes:
            payload["scope"] = " ".join(requested_scopes)

        tokens = self._token_request(payload)
        if store:
            self._save_tokens(tokens)
        return tokens

    def revoke(self, token: str, *, client_secret: str | None = None) -> None:
        self._http.post(
            "/oauth/revoke",
            data={
                "token": token,
                "client_id": self.client_id,
                **self._client_secret_payload(client_secret),
            },
            oauth_error=True,
        )

    def introspect(self, token: str, *, client_secret: str | None = None) -> dict[str, Any]:
        return self._http.post(
            "/oauth/introspect",
            data={
                "token": token,
                "client_id": self.client_id,
                **self._client_secret_payload(client_secret),
            },
            oauth_error=True,
        )

    def start_device_flow(
        self,
        scopes: Iterable[str] | str,
        *,
        client_secret: str | None = None,
    ) -> DeviceAuthorization:
        requested_scopes = self.validator.validate_delegated(scopes)
        payload = self._http.post(
            "/oauth/device/code",
            data={
                "client_id": self.client_id,
                "scope": " ".join(requested_scopes),
                **self._client_secret_payload(client_secret),
            },
            oauth_error=True,
        )
        return DeviceAuthorization.from_mapping(payload)

    def exchange_device_code(
        self,
        device_code: str,
        *,
        client_secret: str | None = None,
        store: bool = True,
    ) -> TokenSet:
        tokens = self._token_request(
            {
                "grant_type": "urn:ietf:params:oauth:grant-type:device_code",
                "client_id": self.client_id,
                "device_code": device_code,
                **self._client_secret_payload(client_secret),
            }
        )
        if store:
            self._save_tokens(tokens)
        return tokens

    def poll_device_token(
        self,
        device_code: str,
        *,
        interval: int = 5,
        timeout: int = 600,
        client_secret: str | None = None,
        store: bool = True,
    ) -> TokenSet:
        deadline = time.monotonic() + timeout
        delay = interval
        while True:
            try:
                return self.exchange_device_code(
                    device_code,
                    client_secret=client_secret,
                    store=store,
                )
            except Exception as exc:
                error = getattr(exc, "error", None)
                if error == "slow_down":
                    delay += 5
                elif error != "authorization_pending":
                    raise

            if time.monotonic() + delay > deadline:
                raise TimeoutError("device authorization timed out")
            time.sleep(delay)

    def client_credentials(
        self,
        scopes: Iterable[str] | str,
        *,
        client_secret: str | None = None,
        store: bool = True,
    ) -> TokenSet:
        requested_scopes = self.validator.validate_client_credentials(scopes)
        tokens = self._token_request(
            {
                "grant_type": "client_credentials",
                "client_id": self.client_id,
                "scope": " ".join(requested_scopes),
                **self._client_secret_payload(client_secret),
            }
        )
        if store:
            self._save_tokens(tokens)
        return tokens

    def _token_request(self, payload: dict[str, Any]) -> TokenSet:
        data = self._http.post("/oauth/token", data=payload, oauth_error=True)
        return TokenSet.from_mapping(data)

    def _client_secret_payload(self, client_secret: str | None) -> dict[str, str]:
        secret = client_secret if client_secret is not None else self.client_secret
        return {"client_secret": secret} if secret else {}

    def _save_tokens(self, tokens: TokenSet) -> None:
        if self.token_store is not None:
            self.token_store.save(tokens)
