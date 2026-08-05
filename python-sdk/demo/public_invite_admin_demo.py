"""Local demo for managing invite codes through a public OAuth app.

This demo represents an administrator through Authorization Code + PKCE:
the browser signs in as an administrator, approves the public app, and the
script uses the delegated bearer token to manage invite codes.

The file is intentionally a demo script and is not part of the SDK public API.

Examples:
  python python-sdk/demo/public_invite_admin_demo.py configure --api-base-url https://test.gpa.ac.cn/skinapi --site-base-url https://test.gpa.ac.cn --client-id demo --redirect-uri http://127.0.0.1:8766/oauth/callback
  python python-sdk/demo/public_invite_admin_demo.py authorize
  python python-sdk/demo/public_invite_admin_demo.py authorize-device
  python python-sdk/demo/public_invite_admin_demo.py list --device-token
  python python-sdk/demo/public_invite_admin_demo.py list
  python python-sdk/demo/public_invite_admin_demo.py create --code PUBLIC_DEMO --total-uses 1 --note "delegated SDK demo"
  python python-sdk/demo/public_invite_admin_demo.py delete PUBLIC_DEMO
  python python-sdk/demo/public_invite_admin_demo.py roundtrip --code PUBLIC_DEMO --total-uses 1
"""

from __future__ import annotations

import argparse
import json
import os
import secrets
import sys
import threading
import time
import webbrowser
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any
from urllib.parse import parse_qs, urlencode, urlparse

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "src"
sys.path.insert(0, str(SRC))

from element_skin_sdk import OAuthClient  # noqa: E402
from element_skin_sdk.exceptions import OAuthError  # noqa: E402
from element_skin_sdk.http import HTTPClient  # noqa: E402
from element_skin_sdk.oauth.pkce import create_code_challenge, generate_code_verifier  # noqa: E402


CONFIG_PATH = Path(__file__).with_name(".public_invite_admin_demo_config.json")
TOKEN_PATH = Path(__file__).with_name(".public_invite_admin_demo_tokens.json")
DEVICE_TOKEN_PATH = Path(__file__).with_name(".public_invite_admin_demo_device_tokens.json")
INVITE_SCOPES = ("invite.read.any", "invite.create.any", "invite.delete.any")


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Manage Element Skin invite codes via delegated OAuth public app."
    )
    subparsers = parser.add_subparsers(dest="command", required=True)
    device_token_parent = argparse.ArgumentParser(add_help=False)
    device_token_parent.add_argument(
        "--device-token",
        action="store_true",
        help="Use a delegated token obtained through Device Code Flow.",
    )

    configure = subparsers.add_parser("configure", help="Save public OAuth app configuration locally.")
    configure.add_argument("--api-base-url", required=True)
    configure.add_argument("--site-base-url", required=True)
    configure.add_argument("--client-id", required=True)
    configure.add_argument("--redirect-uri", default="http://127.0.0.1:8766/oauth/callback")

    authorize = subparsers.add_parser("authorize", help="Open browser and save delegated tokens.")
    authorize.add_argument("--timeout", type=int, default=300)
    authorize.add_argument("--no-browser", action="store_true")

    authorize_device = subparsers.add_parser(
        "authorize-device",
        help="Authorize this public app with Device Code Flow and save delegated tokens.",
    )
    authorize_device.add_argument("--timeout", type=int, default=600)
    authorize_device.add_argument("--no-browser", action="store_true")

    list_cmd = subparsers.add_parser("list", parents=[device_token_parent], help="List invite codes.")
    list_cmd.add_argument("--limit", type=int, default=15)
    list_cmd.add_argument("--cursor")

    create = subparsers.add_parser("create", parents=[device_token_parent], help="Create an invite code.")
    create.add_argument("--code", required=True)
    create.add_argument("--total-uses", type=int)
    create.add_argument("--note", default="")

    delete = subparsers.add_parser("delete", parents=[device_token_parent], help="Delete an invite code.")
    delete.add_argument("code")

    roundtrip = subparsers.add_parser(
        "roundtrip",
        parents=[device_token_parent],
        help="Create, list, delete, then list again.",
    )
    roundtrip.add_argument("--code", required=True)
    roundtrip.add_argument("--total-uses", type=int, default=1)
    roundtrip.add_argument("--note", default="delegated public OAuth demo")

    args = parser.parse_args()

    if args.command == "configure":
        save_json(
            CONFIG_PATH,
            {
                "api_base_url": args.api_base_url.rstrip("/"),
                "site_base_url": args.site_base_url.rstrip("/"),
                "client_id": args.client_id,
                "redirect_uri": args.redirect_uri,
            },
        )
        print_json({"configured": True, "path": str(CONFIG_PATH)})
        return 0

    config = load_config()
    if args.command == "authorize":
        tokens = authorize_public_app(config, timeout=args.timeout, open_browser=not args.no_browser)
        save_json(TOKEN_PATH, tokens)
        print_json(
            {
                "authorized": True,
                "path": str(TOKEN_PATH),
                "scope": tokens["scope"],
                "permissions": tokens["permissions"],
            }
        )
        return 0

    if args.command == "authorize-device":
        tokens = authorize_device_flow(config, timeout=args.timeout, open_browser=not args.no_browser)
        save_json(DEVICE_TOKEN_PATH, tokens)
        print_json(
            {
                "authorized": True,
                "path": str(DEVICE_TOKEN_PATH),
                "scope": tokens["scope"],
                "permissions": tokens["permissions"],
            }
        )
        return 0

    access_token = valid_device_access_token(config) if args.device_token else valid_access_token(config)
    api = HTTPClient(config["api_base_url"], access_token=access_token)
    try:
        if args.command == "list":
            print_json(list_invites(api, limit=args.limit, cursor=args.cursor))
            return 0
        if args.command == "create":
            print_json(create_invite(api, args.code, args.total_uses, args.note))
            return 0
        if args.command == "delete":
            delete_invite(api, args.code)
            print_json({"deleted": args.code})
            return 0
        if args.command == "roundtrip":
            created = create_invite(api, args.code, args.total_uses, args.note)
            after_create = list_invites(api, limit=15)
            delete_invite(api, args.code)
            after_delete = list_invites(api, limit=15)
            print_json(
                {
                    "created": created,
                    "listed_after_create": contains_invite(after_create, args.code),
                    "deleted": args.code,
                    "listed_after_delete": contains_invite(after_delete, args.code),
                }
            )
            return 0
    finally:
        api.close()

    raise AssertionError(f"unknown command: {args.command}")


def authorize_public_app(config: dict[str, str], *, timeout: int, open_browser: bool) -> dict[str, Any]:
    redirect = urlparse(config["redirect_uri"])
    if redirect.scheme != "http" or redirect.hostname not in {"127.0.0.1", "localhost"}:
        raise SystemExit("redirect_uri must be a local http callback for this demo")
    if redirect.port is None:
        raise SystemExit("redirect_uri must include a port for this demo")

    code_verifier = generate_code_verifier()
    state = secrets.token_urlsafe(24)
    callback = CallbackState(expected_state=state)
    server = ThreadingHTTPServer((redirect.hostname, redirect.port), callback_handler(callback))
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()

    authorization_url = build_authorization_url(config, code_verifier=code_verifier, state=state)
    print_json({"authorization_url": authorization_url})
    if open_browser:
        webbrowser.open(authorization_url)

    try:
        code = callback.wait(timeout)
    finally:
        server.shutdown()
        server.server_close()

    oauth = OAuthClient(
        config["api_base_url"],
        config["client_id"],
        redirect_uri=config["redirect_uri"],
    )
    try:
        token_set = oauth.exchange_code(code=code, code_verifier=code_verifier, store=False)
    finally:
        oauth.close()

    expires_at = int(time.time()) + token_set.expires_in
    return {
        "access_token": token_set.access_token,
        "refresh_token": token_set.refresh_token,
        "token_type": token_set.token_type,
        "expires_at": expires_at,
        "scope": token_set.scope,
        "permissions": list(token_set.permissions),
    }


def authorize_device_flow(config: dict[str, str], *, timeout: int, open_browser: bool) -> dict[str, Any]:
    oauth = OAuthClient(config["api_base_url"], config["client_id"])
    try:
        device = oauth.start_device_flow(INVITE_SCOPES)
        print_json(
            {
                "user_code": device.user_code,
                "verification_uri": device.verification_uri,
                "verification_uri_complete": device.verification_uri_complete,
                "expires_in": device.expires_in,
                "interval": device.interval,
                "scope": device.scope,
                "permissions": list(device.permissions),
            }
        )
        if open_browser and device.verification_uri_complete:
            webbrowser.open(device.verification_uri_complete)
        token_set = oauth.poll_device_token(
            device.device_code,
            interval=device.interval,
            timeout=timeout,
            store=False,
        )
    finally:
        oauth.close()

    return token_payload(token_set)


def build_authorization_url(config: dict[str, str], *, code_verifier: str, state: str) -> str:
    query = urlencode(
        {
            "response_type": "code",
            "client_id": config["client_id"],
            "redirect_uri": config["redirect_uri"],
            "scope": " ".join(INVITE_SCOPES),
            "state": state,
            "code_challenge": create_code_challenge(code_verifier),
            "code_challenge_method": "S256",
        }
    )
    return f'{config["site_base_url"]}/oauth/authorize?{query}'


def valid_access_token(config: dict[str, str]) -> str:
    tokens = load_tokens()
    if int(tokens["expires_at"]) - 30 > int(time.time()):
        return str(tokens["access_token"])
    refresh_token = tokens.get("refresh_token")
    if not refresh_token:
        raise SystemExit("access token expired and no refresh token is available; run authorize again")

    oauth = OAuthClient(config["api_base_url"], config["client_id"], redirect_uri=config["redirect_uri"])
    try:
        refreshed = oauth.refresh(str(refresh_token), scopes=INVITE_SCOPES, store=False)
    except OAuthError as exc:
        if exc.error == "invalid_grant":
            raise SystemExit("refresh token is invalid or revoked; run authorize again") from exc
        raise
    finally:
        oauth.close()
    updated = {
        "access_token": refreshed.access_token,
        "refresh_token": refreshed.refresh_token,
        "token_type": refreshed.token_type,
        "expires_at": int(time.time()) + refreshed.expires_in,
        "scope": refreshed.scope,
        "permissions": list(refreshed.permissions),
    }
    save_json(TOKEN_PATH, updated)
    return refreshed.access_token


def valid_device_access_token(config: dict[str, str]) -> str:
    tokens = load_device_tokens()
    if int(tokens["expires_at"]) - 30 > int(time.time()):
        return str(tokens["access_token"])

    refresh_token = tokens.get("refresh_token")
    if not refresh_token:
        raise SystemExit("device access token expired and no refresh token is available; run authorize-device again")

    oauth = OAuthClient(config["api_base_url"], config["client_id"])
    try:
        refreshed = oauth.refresh(str(refresh_token), scopes=INVITE_SCOPES, store=False)
    except OAuthError as exc:
        if exc.error == "invalid_grant":
            raise SystemExit("device refresh token is invalid or revoked; run authorize-device again") from exc
        raise
    finally:
        oauth.close()
    updated = token_payload(refreshed)
    save_json(DEVICE_TOKEN_PATH, updated)
    return refreshed.access_token


def token_payload(token_set) -> dict[str, Any]:
    return {
        "access_token": token_set.access_token,
        "refresh_token": token_set.refresh_token,
        "token_type": token_set.token_type,
        "expires_at": int(time.time()) + token_set.expires_in,
        "scope": token_set.scope,
        "permissions": list(token_set.permissions),
    }


class CallbackState:
    def __init__(self, *, expected_state: str):
        self.expected_state = expected_state
        self.code: str | None = None
        self.error: str | None = None
        self.event = threading.Event()

    def wait(self, timeout: int) -> str:
        if not self.event.wait(timeout):
            raise SystemExit("timed out waiting for OAuth callback")
        if self.error:
            raise SystemExit(self.error)
        if not self.code:
            raise SystemExit("callback did not include authorization code")
        return self.code


def callback_handler(callback: CallbackState) -> type[BaseHTTPRequestHandler]:
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self) -> None:
            parsed = urlparse(self.path)
            params = parse_qs(parsed.query)
            state = one(params, "state")
            error = one(params, "error")
            code = one(params, "code")
            if state != callback.expected_state:
                callback.error = "state mismatch in OAuth callback"
            elif error:
                callback.error = f"authorization failed: {error}"
            else:
                callback.code = code
            callback.event.set()
            body = b"Authorization received. You can close this window."
            self.send_response(200)
            self.send_header("Content-Type", "text/plain; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, _format: str, *_args: object) -> None:
            return

    return Handler


def one(params: dict[str, list[str]], key: str) -> str:
    values = params.get(key)
    return values[0] if values else ""


def load_config() -> dict[str, str]:
    if not CONFIG_PATH.exists():
        raise SystemExit(f"missing config; run configure first: {CONFIG_PATH}")
    data = json.loads(CONFIG_PATH.read_text(encoding="utf-8"))
    required = ("api_base_url", "site_base_url", "client_id", "redirect_uri")
    missing = [key for key in required if not data.get(key)]
    if missing:
        raise SystemExit(f"config missing required keys: {', '.join(missing)}")
    return {key: str(data[key]) for key in required}


def load_tokens() -> dict[str, Any]:
    if not TOKEN_PATH.exists():
        raise SystemExit(f"missing tokens; run authorize first: {TOKEN_PATH}")
    data = json.loads(TOKEN_PATH.read_text(encoding="utf-8"))
    required = ("access_token", "expires_at")
    missing = [key for key in required if not data.get(key)]
    if missing:
        raise SystemExit(f"token file missing required keys: {', '.join(missing)}")
    return data


def load_device_tokens() -> dict[str, Any]:
    if not DEVICE_TOKEN_PATH.exists():
        raise SystemExit(f"missing device tokens; run authorize-device first: {DEVICE_TOKEN_PATH}")
    data = json.loads(DEVICE_TOKEN_PATH.read_text(encoding="utf-8"))
    required = ("access_token", "expires_at")
    missing = [key for key in required if not data.get(key)]
    if missing:
        raise SystemExit(f"device token file missing required keys: {', '.join(missing)}")
    return data


def save_json(path: Path, data: dict[str, Any]) -> None:
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")
    try:
        os.chmod(path, 0o600)
    except OSError:
        pass


def list_invites(api: HTTPClient, *, limit: int, cursor: str | None = None) -> dict[str, Any]:
    params: dict[str, Any] = {"limit": limit}
    if cursor:
        params["cursor"] = cursor
    return api.get("/v2/admin/invites", params=params)


def create_invite(api: HTTPClient, code: str, total_uses: int | None, note: str) -> dict[str, Any]:
    return api.post(
        "/v2/admin/invites",
        json={
            "code": code,
            "total_uses": total_uses,
            "note": note,
        },
    )


def delete_invite(api: HTTPClient, code: str) -> None:
    api.delete(f"/v2/admin/invites/{code}")


def contains_invite(page: dict[str, Any], code: str) -> bool:
    items = page.get("items", [])
    return any(isinstance(item, dict) and item.get("code") == code for item in items)


def print_json(value: object) -> None:
    print(json.dumps(value, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    raise SystemExit(main())
