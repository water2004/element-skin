"""SettingsBackend 测试：默认值回退、分组保存、fallback 校验边界、公开设置"""

import pytest
from fastapi import HTTPException
from backends.settings_backend import SettingsBackend, SETTING_DEFAULTS


@pytest.mark.asyncio
async def test_defaults_when_unset(db_session):
    """未写入任何设置时，getter 回退到 SETTING_DEFAULTS（含类型转换）"""
    backend = SettingsBackend(db_session)

    site = await backend.get_site_settings()
    assert site["site_name"] == SETTING_DEFAULTS["site_name"] == "皮肤站"
    assert site["max_texture_size"] == 1024  # str "1024" → int
    assert site["allow_register"] is True  # str "true" → bool
    assert site["require_invite"] is False
    assert site["profile_uuid_mode"] == "random"

    sec = await backend.get_security_settings()
    assert sec["rate_limit_auth_attempts"] == 5
    assert sec["rate_limit_enabled"] is True

    auth = await backend.get_auth_settings()
    assert auth["jwt_expire_days"] == 7

    email = await backend.get_email_settings()
    assert email["smtp_port"] == 465
    assert email["email_verify_enabled"] is False


@pytest.mark.asyncio
async def test_save_site_group_roundtrip(db_session):
    """保存 site 分组后读回，bool/int 正确转换且持久化"""
    backend = SettingsBackend(db_session)
    await backend.save_settings_group("site", {
        "site_name": "New Site",
        "allow_register": False,
        "max_texture_size": 2048,
        "profile_uuid_mode": "offline",
    })

    site = await backend.get_site_settings()
    assert site["site_name"] == "New Site"
    assert site["allow_register"] is False
    assert site["max_texture_size"] == 2048
    assert site["profile_uuid_mode"] == "offline"


@pytest.mark.asyncio
async def test_save_security_group_roundtrip(db_session):
    backend = SettingsBackend(db_session)
    await backend.save_settings_group("security", {
        "rate_limit_enabled": True,
        "rate_limit_auth_attempts": 10,
    })
    sec = await backend.get_security_settings()
    assert sec["rate_limit_auth_attempts"] == 10
    assert sec["rate_limit_enabled"] is True


@pytest.mark.asyncio
async def test_save_invalid_group_rejected(db_session):
    backend = SettingsBackend(db_session)
    with pytest.raises(HTTPException) as exc:
        await backend.save_settings_group("nonsense", {"x": 1})
    assert exc.value.status_code == 400


@pytest.mark.asyncio
async def test_save_invalid_profile_uuid_mode_rejected(db_session):
    backend = SettingsBackend(db_session)
    with pytest.raises(HTTPException) as exc:
        await backend.save_settings_group("site", {"profile_uuid_mode": "bogus"})
    assert exc.value.status_code == 400
    # 非法值被拒后不应落库，仍返回默认
    assert (await backend.get_site_settings())["profile_uuid_mode"] == "random"


@pytest.mark.asyncio
async def test_empty_smtp_password_is_skipped(db_session):
    """空 smtp_password 不覆盖既有值（保留语义，而非写空串）"""
    backend = SettingsBackend(db_session)
    await db_session.setting.set("smtp_password", "secret")
    await backend.save_settings_group("email", {"smtp_host": "mail.example.com", "smtp_password": ""})
    assert await db_session.setting.get("smtp_password") == "secret"
    assert (await backend.get_email_settings())["smtp_host"] == "mail.example.com"


@pytest.mark.asyncio
async def test_validate_fallback_not_list(db_session):
    backend = SettingsBackend(db_session)
    with pytest.raises(HTTPException) as exc:
        backend._validate_fallback_services("not-a-list")
    assert exc.value.status_code == 400


@pytest.mark.asyncio
async def test_validate_fallback_missing_url(db_session):
    backend = SettingsBackend(db_session)
    with pytest.raises(HTTPException) as exc:
        backend._validate_fallback_services([
            {"session_url": "https://s", "account_url": "", "services_url": "https://x"}
        ])
    assert exc.value.status_code == 400
    assert "urls are required" in exc.value.detail


@pytest.mark.asyncio
async def test_validate_fallback_negative_cache_ttl(db_session):
    backend = SettingsBackend(db_session)
    with pytest.raises(HTTPException) as exc:
        backend._validate_fallback_services([
            {
                "session_url": "https://s",
                "account_url": "https://a",
                "services_url": "https://x",
                "cache_ttl": -5,
            }
        ])
    assert exc.value.status_code == 400
    assert "cache_ttl" in exc.value.detail


@pytest.mark.asyncio
async def test_validate_fallback_normalizes_domains(db_session):
    """合法条目：逗号串与 list 都规整为逗号串，布尔默认到位"""
    backend = SettingsBackend(db_session)
    result = backend._validate_fallback_services([
        {
            "session_url": "https://s",
            "account_url": "https://a",
            "services_url": "https://x",
            "skin_domains": ["example.com", " mc.net ", ""],
            "cache_ttl": "30",
        }
    ])
    assert len(result) == 1
    entry = result[0]
    assert entry["skin_domains"] == "example.com,mc.net"
    assert entry["cache_ttl"] == 30
    assert entry["enable_profile"] is True
    assert entry["enable_whitelist"] is False


@pytest.mark.asyncio
async def test_public_settings_defaults(db_session):
    """公开设置默认值取自同一 SETTING_DEFAULTS，无 fallback 时给 Mojang 兜底"""
    backend = SettingsBackend(db_session)
    pub = await backend.get_public_settings()
    assert pub["site_name"] == SETTING_DEFAULTS["site_name"]
    assert pub["allow_register"] is True
    assert pub["mojang_status_urls"]["session"] == "https://sessionserver.mojang.com"


@pytest.mark.asyncio
async def test_public_settings_reflect_saved(db_session):
    backend = SettingsBackend(db_session)
    await backend.save_settings_group("site", {"site_name": "Public Name", "allow_register": False})
    pub = await backend.get_public_settings()
    assert pub["site_name"] == "Public Name"
    assert pub["allow_register"] is False
