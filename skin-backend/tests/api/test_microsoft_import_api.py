import pytest
from unittest.mock import AsyncMock, patch

from routers.microsoft_routes import oauth_states


def _seed_import_session(user_id, *, profile_id, profile_name, skin_url=None,
                         skin_variant="classic", cape_url=None, kind="import"):
    """在模块级 oauth_states 中放入一次性导入会话，返回 import_token。

    模拟 get-profile 换发的、服务端固化的可信资料。
    """
    import secrets

    token = secrets.token_urlsafe(16)
    skins = []
    if skin_url is not None:
        skins.append({"url": skin_url, "variant": skin_variant})
    capes = []
    if cape_url is not None:
        capes.append({"url": cape_url})
    oauth_states.put(
        token,
        {
            "user_id": user_id,
            "kind": kind,
            "profile": {
                "id": profile_id,
                "name": profile_name,
                "skins": skins,
                "capes": capes,
            },
        },
        ttl_seconds=300,
    )
    return token


@pytest.mark.asyncio
async def test_microsoft_import_profile_success(client, auth_headers, db_session):
    profile_id = "ms_profile_id"
    profile_name = "MsPlayer"

    token = _seed_import_session(
        auth_headers["X-User-ID"],
        profile_id=profile_id,
        profile_name=profile_name,
        skin_url="http://skin.url",
        skin_variant="classic",
    )

    with patch("backends.microsoft_backend.download_texture", new_callable=AsyncMock) as mock_download:
        mock_download.return_value = b"skin_bytes"

        response = await client.post(
            "/microsoft/import-profile",
            cookies=auth_headers["cookies"],
            json={"ms_token": token},
        )

        assert response.status_code == 200
        data = response.json()
        assert data["ok"] is True
        assert data["profile"]["id"] == profile_id
        assert data["profile"]["name"] == profile_name

        profile = await db_session.user.get_profile_by_id(profile_id)
        assert profile is not None
        assert profile.name == profile_name


@pytest.mark.asyncio
async def test_microsoft_import_profile_uuid_conflict(client, auth_headers, db_session, user_factory):
    profile_id = "conflict_ms_id"
    profile_name = "ConflictMsPlayer"

    user = await user_factory()
    from utils.typing import PlayerProfile
    await db_session.user.create_profile(
        PlayerProfile(profile_id, user.id, "ExistingOne", "default")
    )

    token = _seed_import_session(
        auth_headers["X-User-ID"],
        profile_id=profile_id,
        profile_name=profile_name,
    )

    with patch("backends.microsoft_backend.download_texture", new_callable=AsyncMock):
        response = await client.post(
            "/microsoft/import-profile",
            cookies=auth_headers["cookies"],
            json={"ms_token": token},
        )

        assert response.status_code == 400
        assert "UUID" in response.json()["detail"]


@pytest.mark.asyncio
async def test_microsoft_import_profile_name_conflict(client, auth_headers, db_session, user_factory):
    profile_id = "new_ms_id"
    profile_name = "TakenMsName"

    user = await user_factory()
    from utils.typing import PlayerProfile
    await db_session.user.create_profile(
        PlayerProfile("other_id", user.id, profile_name, "default")
    )

    token = _seed_import_session(
        auth_headers["X-User-ID"],
        profile_id=profile_id,
        profile_name=profile_name,
        skin_url="http://skin.url",
    )

    with patch("backends.microsoft_backend.download_texture", new_callable=AsyncMock) as mock_download:
        mock_download.return_value = b"skin_bytes"

        response = await client.post(
            "/microsoft/import-profile",
            cookies=auth_headers["cookies"],
            json={"ms_token": token},
        )

        assert response.status_code == 200
        data = response.json()
        assert data["profile"]["id"] == profile_id
        assert data["profile"]["name"] == f"{profile_name}_1"

        profile = await db_session.user.get_profile_by_id(profile_id)
        assert profile.name == f"{profile_name}_1"


@pytest.mark.asyncio
async def test_microsoft_import_ignores_client_supplied_fields(client, auth_headers, db_session):
    """导入只信任服务端固化的会话资料：前端额外伪造的字段必须被忽略。"""
    verified_id = "verified_ms_id"
    verified_name = "VerifiedPlayer"

    token = _seed_import_session(
        auth_headers["X-User-ID"],
        profile_id=verified_id,
        profile_name=verified_name,
    )

    with patch("backends.microsoft_backend.download_texture", new_callable=AsyncMock):
        response = await client.post(
            "/microsoft/import-profile",
            cookies=auth_headers["cookies"],
            json={
                "ms_token": token,
                # 攻击者伪造：应被完全忽略
                "profile_id": "forged_id",
                "profile_name": "ForgedName",
                "skin_url": "http://evil/skin.png",
            },
        )

        assert response.status_code == 200
        data = response.json()
        assert data["profile"]["id"] == verified_id
        assert data["profile"]["name"] == verified_name

        # 伪造的 UUID 不应落库
        assert await db_session.user.get_profile_by_id("forged_id") is None
        assert await db_session.user.get_profile_by_id(verified_id) is not None


@pytest.mark.asyncio
async def test_microsoft_import_token_is_one_time(client, auth_headers, db_session):
    """import_token 一次性：消费后再次使用应 400。"""
    token = _seed_import_session(
        auth_headers["X-User-ID"],
        profile_id="one_time_id",
        profile_name="OneTime",
    )

    with patch("backends.microsoft_backend.download_texture", new_callable=AsyncMock):
        first = await client.post(
            "/microsoft/import-profile",
            cookies=auth_headers["cookies"],
            json={"ms_token": token},
        )
        assert first.status_code == 200

        second = await client.post(
            "/microsoft/import-profile",
            cookies=auth_headers["cookies"],
            json={"ms_token": token},
        )
        assert second.status_code == 400


@pytest.mark.asyncio
async def test_microsoft_import_invalid_token(client, auth_headers, db_session):
    """不存在/过期的 token → 400。"""
    response = await client.post(
        "/microsoft/import-profile",
        cookies=auth_headers["cookies"],
        json={"ms_token": "does-not-exist"},
    )
    assert response.status_code == 400


@pytest.mark.asyncio
async def test_microsoft_import_other_users_token(client, auth_headers, db_session):
    """会话属于其他用户 → 403。"""
    token = _seed_import_session(
        "some-other-user-id",
        profile_id="x_id",
        profile_name="X",
    )

    response = await client.post(
        "/microsoft/import-profile",
        cookies=auth_headers["cookies"],
        json={"ms_token": token},
    )
    assert response.status_code == 403


@pytest.mark.asyncio
async def test_microsoft_import_rejects_wrong_kind_token(client, auth_headers, db_session):
    """用 profile 类型的 token 调 import（类型混用）→ 400。"""
    token = _seed_import_session(
        auth_headers["X-User-ID"],
        profile_id="wrong_kind_id",
        profile_name="WrongKind",
        kind="profile",
    )

    response = await client.post(
        "/microsoft/import-profile",
        cookies=auth_headers["cookies"],
        json={"ms_token": token},
    )
    assert response.status_code == 400
