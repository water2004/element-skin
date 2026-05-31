"""站点设置后端：默认值单一事实来源 + 分组读写与校验"""

from typing import Any

from fastapi import HTTPException

from database_module import Database

# 所有设置项默认值的单一事实来源（值统一为字符串，与 setting 表存储一致）
SETTING_DEFAULTS = {
    "site_name": "皮肤站",
    "site_subtitle": "简洁、高效、现代的 Minecraft 皮肤管理站",
    "require_invite": "false",
    "allow_register": "true",
    "enable_skin_library": "true",
    "max_texture_size": "1024",
    "footer_text": "",
    "filing_icp": "",
    "filing_icp_link": "",
    "filing_mps": "",
    "filing_mps_link": "",
    "profile_uuid_mode": "random",
    "rate_limit_enabled": "true",
    "rate_limit_auth_attempts": "5",
    "rate_limit_auth_window": "15",
    "enable_strong_password_check": "false",
    "jwt_expire_days": "7",
    "microsoft_client_id": "",
    "microsoft_client_secret": "",
    "microsoft_redirect_uri": "",
    "email_verify_enabled": "false",
    "email_verify_ttl": "300",
    "smtp_host": "",
    "smtp_port": "465",
    "smtp_user": "",
    "smtp_ssl": "true",
    "smtp_sender": "",
    "fallback_strategy": "serial",
}


def _bool(s: dict, key: str) -> bool:
    return s.get(key, SETTING_DEFAULTS[key]) == "true"


def _int(s: dict, key: str) -> int:
    return int(s.get(key, SETTING_DEFAULTS[key]))


def _str(s: dict, key: str) -> str:
    return s.get(key, SETTING_DEFAULTS[key])


class SettingsBackend:
    def __init__(self, db: Database):
        self.db = db

    async def get_public_settings(self) -> dict:
        s = await self.db.setting.get_all()
        fallbacks = await self.db.fallback.list_endpoints()
        primary = fallbacks[0] if fallbacks else None
        return {
            "site_name": _str(s, "site_name"),
            "site_subtitle": _str(s, "site_subtitle"),
            "allow_register": _bool(s, "allow_register"),
            "enable_skin_library": _bool(s, "enable_skin_library"),
            "email_verify_enabled": _bool(s, "email_verify_enabled"),
            "footer_text": _str(s, "footer_text"),
            "filing_icp": _str(s, "filing_icp"),
            "filing_icp_link": _str(s, "filing_icp_link"),
            "filing_mps": _str(s, "filing_mps"),
            "filing_mps_link": _str(s, "filing_mps_link"),
            "mojang_status_urls": {
                "session": (primary or {}).get("session_url", "https://sessionserver.mojang.com"),
                "account": (primary or {}).get("account_url", "https://api.mojang.com"),
                "services": (primary or {}).get("services_url", "https://api.minecraftservices.com"),
            },
        }

    async def get_site_settings(self) -> dict:
        s = await self.db.setting.get_all()
        return {
            "site_name": _str(s, "site_name"),
            "site_subtitle": _str(s, "site_subtitle"),
            "require_invite": _bool(s, "require_invite"),
            "allow_register": _bool(s, "allow_register"),
            "enable_skin_library": _bool(s, "enable_skin_library"),
            "max_texture_size": _int(s, "max_texture_size"),
            "footer_text": _str(s, "footer_text"),
            "filing_icp": _str(s, "filing_icp"),
            "filing_icp_link": _str(s, "filing_icp_link"),
            "filing_mps": _str(s, "filing_mps"),
            "filing_mps_link": _str(s, "filing_mps_link"),
            "profile_uuid_mode": _str(s, "profile_uuid_mode"),
        }

    async def get_security_settings(self) -> dict:
        s = await self.db.setting.get_all()
        return {
            "rate_limit_enabled": _bool(s, "rate_limit_enabled"),
            "rate_limit_auth_attempts": _int(s, "rate_limit_auth_attempts"),
            "rate_limit_auth_window": _int(s, "rate_limit_auth_window"),
            "enable_strong_password_check": _bool(s, "enable_strong_password_check"),
        }

    async def get_auth_settings(self) -> dict:
        s = await self.db.setting.get_all()
        return {
            "jwt_expire_days": _int(s, "jwt_expire_days"),
        }

    async def get_microsoft_settings(self) -> dict:
        s = await self.db.setting.get_all()
        return {
            "microsoft_client_id": _str(s, "microsoft_client_id"),
            "microsoft_client_secret": _str(s, "microsoft_client_secret"),
            "microsoft_redirect_uri": _str(s, "microsoft_redirect_uri"),
        }

    async def get_email_settings(self) -> dict:
        s = await self.db.setting.get_all()
        return {
            "email_verify_enabled": _bool(s, "email_verify_enabled"),
            "email_verify_ttl": _int(s, "email_verify_ttl"),
            "smtp_host": _str(s, "smtp_host"),
            "smtp_port": _int(s, "smtp_port"),
            "smtp_user": _str(s, "smtp_user"),
            "smtp_ssl": _bool(s, "smtp_ssl"),
            "smtp_sender": _str(s, "smtp_sender"),
        }

    async def get_fallback_settings(self) -> dict:
        s = await self.db.setting.get_all()
        return {
            "fallback_strategy": _str(s, "fallback_strategy"),
            "fallbacks": await self.db.fallback.list_endpoints(),
        }

    async def save_settings_group(self, group: str, body: dict):
        allowed_keys = {
            "site": [
                "site_name",
                "site_subtitle",
                "require_invite",
                "allow_register",
                "enable_skin_library",
                "max_texture_size",
                "footer_text",
                "filing_icp",
                "filing_icp_link",
                "filing_mps",
                "filing_mps_link",
                "profile_uuid_mode",
            ],
            "security": ["rate_limit_enabled", "rate_limit_auth_attempts", "rate_limit_auth_window", "enable_strong_password_check"],
            "auth": ["jwt_expire_days"],
            "microsoft": ["microsoft_client_id", "microsoft_client_secret", "microsoft_redirect_uri"],
            "email": ["email_verify_enabled", "email_verify_ttl", "smtp_host", "smtp_port", "smtp_user", "smtp_password", "smtp_ssl", "smtp_sender"],
            "fallback": ["fallback_strategy"],
        }

        if group not in allowed_keys:
            raise HTTPException(status_code=400, detail="Invalid settings group")

        for key in allowed_keys[group]:
            if key in body:
                val = body[key]
                if key == "profile_uuid_mode":
                    mode = str(val).strip().lower()
                    if mode not in ["random", "offline"]:
                        raise HTTPException(status_code=400, detail="profile_uuid_mode must be random or offline")
                    val = mode
                # Special handling for password
                if key == "smtp_password" and not val:
                    continue

                value = "true" if isinstance(val, bool) and val else ("false" if isinstance(val, bool) else str(val))
                await self.db.setting.set(key, value)

        if group == "fallback" and "fallbacks" in body:
            fallbacks = self._validate_fallback_services(body.get("fallbacks"))
            await self.db.fallback.save_endpoints(fallbacks)

    def _validate_fallback_services(self, services: Any) -> list[dict]:
        if not isinstance(services, list):
            raise HTTPException(status_code=400, detail="fallbacks must be a list")

        normalized: list[dict] = []
        for idx, entry in enumerate(services, start=1):
            if not isinstance(entry, dict):
                raise HTTPException(status_code=400, detail="invalid fallback entry")

            endpoint_id = entry.get("id")
            if endpoint_id is not None:
                try:
                    endpoint_id = int(endpoint_id)
                except (TypeError, ValueError):
                    raise HTTPException(status_code=400, detail=f"fallback[{idx}] id invalid")

            session_url = str(entry.get("session_url", "")).strip()
            account_url = str(entry.get("account_url", "")).strip()
            services_url = str(entry.get("services_url", "")).strip()
            cache_ttl = entry.get("cache_ttl", 60)
            raw_domains = entry.get("skin_domains", "")

            if not session_url or not account_url or not services_url:
                raise HTTPException(status_code=400, detail=f"fallback[{idx}] urls are required")

            if isinstance(raw_domains, list):
                skin_domains = [str(item).strip() for item in raw_domains if str(item).strip()]
            else:
                skin_domains = [item.strip() for item in str(raw_domains).split(",") if item.strip()]

            try:
                cache_ttl = int(cache_ttl)
            except (TypeError, ValueError):
                raise HTTPException(status_code=400, detail=f"fallback[{idx}] cache_ttl invalid")

            if cache_ttl < 0:
                raise HTTPException(status_code=400, detail=f"fallback[{idx}] cache_ttl must be non-negative")

            normalized.append({
                "id": endpoint_id,
                "session_url": session_url,
                "account_url": account_url,
                "services_url": services_url,
                "cache_ttl": cache_ttl,
                "skin_domains": ",".join(skin_domains),
                "enable_profile": bool(entry.get("enable_profile", True)),
                "enable_hasjoined": bool(entry.get("enable_hasjoined", True)),
                "enable_whitelist": bool(entry.get("enable_whitelist", False)),
                "note": str(entry.get("note", "")).strip(),
            })
        return normalized

