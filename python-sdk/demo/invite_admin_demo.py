"""Local demo for managing invite codes with an admin-approved OAuth client.

This file is intentionally a demo script, not part of the SDK public API.

Prerequisites:
- The OAuth application has been reviewed by an administrator.
- The application has these app-only permissions:
  - invite.read.any
  - invite.create.any
  - invite.delete.any
- The client secret is stored by `configure` before running operations.

Examples:
  python python-sdk/demo/invite_admin_demo.py configure --base-url http://localhost:8000 --client-id demo --client-secret secret
  python python-sdk/demo/invite_admin_demo.py configure --base-url http://localhost:8000 --client-id demo
  python python-sdk/demo/invite_admin_demo.py authorize-device
  python python-sdk/demo/invite_admin_demo.py list --device-token
  python python-sdk/demo/invite_admin_demo.py list
  python python-sdk/demo/invite_admin_demo.py create --code DEMO_INVITE --total-uses 3 --note "SDK demo"
  python python-sdk/demo/invite_admin_demo.py delete DEMO_INVITE
  python python-sdk/demo/invite_admin_demo.py roundtrip --code DEMO_INVITE --total-uses 1 --note "temporary"
"""

from __future__ import annotations

import argparse
import getpass
import json
import os
import sys
import time
import webbrowser
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "src"
sys.path.insert(0, str(SRC))

from element_skin_sdk import OAuthClient  # noqa: E402
from element_skin_sdk.exceptions import OAuthError  # noqa: E402
from element_skin_sdk.http import HTTPClient  # noqa: E402
from element_skin_sdk.permissions import PermissionValidator  # noqa: E402


CONFIG_PATH = Path(__file__).with_name(".invite_admin_demo_config.json")
DEVICE_TOKEN_PATH = Path(__file__).with_name(".invite_admin_demo_device_tokens.json")
INVITE_SCOPES = ("invite.read.any", "invite.create.any", "invite.delete.any")


def main() -> int:
    parser = argparse.ArgumentParser(description="Manage Element Skin invite codes via OAuth Client Credentials.")
    subparsers = parser.add_subparsers(dest="command", required=True)
    device_token_parent = argparse.ArgumentParser(add_help=False)
    device_token_parent.add_argument(
        "--device-token",
        action="store_true",
        help="Use a delegated token obtained through Device Code Flow.",
    )

    configure = subparsers.add_parser("configure", help="Save OAuth client configuration locally.")
    configure.add_argument("--base-url", required=True)
    configure.add_argument("--client-id", required=True)
    configure.add_argument("--client-secret")

    authorize_device = subparsers.add_parser(
        "authorize-device",
        help="Authorize this confidential app with Device Code Flow and save delegated tokens.",
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
    roundtrip.add_argument("--note", default="SDK invite demo")

    args = parser.parse_args()

    if args.command == "configure":
        client_secret = args.client_secret or getpass.getpass("client secret: ")
        save_json(
            CONFIG_PATH,
            {
                "base_url": args.base_url.rstrip("/"),
                "client_id": args.client_id,
                "client_secret": client_secret,
            }
        )
        print_json({"configured": True, "path": str(CONFIG_PATH)})
        return 0

    config = load_config()
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

    access_token = valid_device_access_token(config) if args.device_token else get_app_token(config).access_token
    api = HTTPClient(config["base_url"], access_token=access_token)

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


def save_json(path: Path, data: dict[str, Any]) -> None:
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")
    try:
        os.chmod(path, 0o600)
    except OSError:
        pass


def load_config() -> dict[str, str]:
    if not CONFIG_PATH.exists():
        raise SystemExit(f"missing config; run configure first: {CONFIG_PATH}")

    data = json.loads(CONFIG_PATH.read_text(encoding="utf-8"))
    required = ("base_url", "client_id", "client_secret")
    missing = [key for key in required if not data.get(key)]
    if missing:
        raise SystemExit(f"config missing required keys: {', '.join(missing)}")
    return {key: str(data[key]) for key in required}


def authorize_device_flow(config: dict[str, str], *, timeout: int, open_browser: bool) -> dict[str, Any]:
    oauth = OAuthClient(
        config["base_url"],
        config["client_id"],
        client_secret=config["client_secret"],
    )
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


def valid_device_access_token(config: dict[str, str]) -> str:
    tokens = load_device_tokens()
    if int(tokens["expires_at"]) - 30 > int(time.time()):
        return str(tokens["access_token"])

    refresh_token = tokens.get("refresh_token")
    if not refresh_token:
        raise SystemExit("device access token expired and no refresh token is available; run authorize-device again")

    oauth = OAuthClient(
        config["base_url"],
        config["client_id"],
        client_secret=config["client_secret"],
    )
    try:
        refreshed = oauth.refresh(
            str(refresh_token),
            scopes=INVITE_SCOPES,
            store=False,
        )
    except OAuthError as exc:
        if exc.error == "invalid_grant":
            raise SystemExit("device refresh token is invalid or revoked; run authorize-device again") from exc
        raise
    finally:
        oauth.close()
    updated = token_payload(refreshed)
    save_json(DEVICE_TOKEN_PATH, updated)
    return refreshed.access_token


def load_device_tokens() -> dict[str, Any]:
    if not DEVICE_TOKEN_PATH.exists():
        raise SystemExit(f"missing device tokens; run authorize-device first: {DEVICE_TOKEN_PATH}")
    data = json.loads(DEVICE_TOKEN_PATH.read_text(encoding="utf-8"))
    required = ("access_token", "expires_at")
    missing = [key for key in required if not data.get(key)]
    if missing:
        raise SystemExit(f"device token file missing required keys: {', '.join(missing)}")
    return data


def token_payload(token_set) -> dict[str, Any]:
    return {
        "access_token": token_set.access_token,
        "refresh_token": token_set.refresh_token,
        "token_type": token_set.token_type,
        "expires_at": int(time.time()) + token_set.expires_in,
        "scope": token_set.scope,
        "permissions": list(token_set.permissions),
    }


def get_app_token(config: dict[str, str]):
    oauth = OAuthClient(
        config["base_url"],
        config["client_id"],
        client_secret=config["client_secret"],
        validator=AdminApprovedClientCredentialsValidator(),
    )
    try:
        return oauth.client_credentials(INVITE_SCOPES)
    finally:
        oauth.close()


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


class AdminApprovedClientCredentialsValidator(PermissionValidator):
    def validate_client_credentials(self, scopes):
        return self.normalize(scopes)


if __name__ == "__main__":
    raise SystemExit(main())
