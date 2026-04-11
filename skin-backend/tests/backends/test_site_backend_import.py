import pytest
from unittest.mock import AsyncMock, patch, MagicMock
from fastapi import HTTPException
from backends.site_backend import SiteBackend
from utils.typing import PlayerProfile

@pytest.mark.asyncio
async def test_import_ygg_profile_success(db_session, test_config):
    backend = SiteBackend(db_session, test_config)
    user_id = "test_user_id"
    api_url = "http://example.com/api/yggdrasil"
    profile_id = "test_profile_id"
    profile_name = "TestPlayer"

    # Mock YggdrasilClient and download_texture
    with patch("backends.site_backend.YggdrasilClient") as MockClient, \
         patch("backends.site_backend.download_texture", new_callable=AsyncMock) as mock_download:
        
        mock_instance = MockClient.return_value
        mock_instance.get_profile_with_textures = AsyncMock(return_value={
            "id": profile_id,
            "name": profile_name,
            "skins": [{"url": "http://skin.url", "variant": "classic"}],
            "capes": []
        })
        mock_download.return_value = b"skin_bytes"

        # Create a dummy user
        await db_session.execute(
            "INSERT INTO users (id, email, password, is_admin, display_name) VALUES ($1, $2, $3, $4, $5)",
            user_id, "test@example.com", "hash", False, "TestUser"
        )
        result = await backend.import_ygg_profile(user_id, api_url, profile_id, profile_name)

        assert result["id"] == profile_id
        assert result["name"] == profile_name
        
        # Verify profile created in DB
        profile = await db_session.user.get_profile_by_id(profile_id)
        assert profile is not None
        assert profile.name == profile_name
        assert profile.user_id == user_id

@pytest.mark.asyncio
async def test_import_ygg_profile_uuid_conflict(db_session, test_config):
    backend = SiteBackend(db_session, test_config)
    user_id = "test_user_id_2"
    api_url = "http://example.com/api/yggdrasil"
    profile_id = "conflict_profile_id"
    profile_name = "ConflictPlayer"

    await db_session.execute(
        "INSERT INTO users (id, email, password, is_admin, display_name) VALUES ($1, $2, $3, $4, $5)",
        user_id, "test2@example.com", "hash", False, "TestUser2"
    )

    # Pre-create a profile with the same ID
    await db_session.user.create_profile(
        PlayerProfile(profile_id, user_id, "ExistingPlayer", "default")
    )

    with patch("backends.site_backend.YggdrasilClient") as MockClient:
        mock_instance = MockClient.return_value
        mock_instance.get_profile_with_textures = AsyncMock(return_value={
            "id": profile_id,
            "name": profile_name,
            "skins": [],
            "capes": []
        })

        with pytest.raises(HTTPException) as exc:
            await backend.import_ygg_profile(user_id, api_url, profile_id, profile_name)
        
        assert exc.value.status_code == 400
        assert "UUID" in exc.value.detail

@pytest.mark.asyncio
async def test_import_ygg_profile_name_conflict(db_session, test_config):
    backend = SiteBackend(db_session, test_config)
    user_id = "test_user_id_3"
    api_url = "http://example.com/api/yggdrasil"
    profile_id = "new_profile_id"
    profile_name = "TakenName"

    await db_session.execute(
        "INSERT INTO users (id, email, password, is_admin, display_name) VALUES ($1, $2, $3, $4, $5)",
        user_id, "test3@example.com", "hash", False, "TestUser3"
    )

    # Pre-create a profile with the same Name but different ID
    await db_session.user.create_profile(
        PlayerProfile("other_id", user_id, profile_name, "default")
    )

    with patch("backends.site_backend.YggdrasilClient") as MockClient, \
         patch("backends.site_backend.download_texture", new_callable=AsyncMock):
        
        mock_instance = MockClient.return_value
        mock_instance.get_profile_with_textures = AsyncMock(return_value={
            "id": profile_id,
            "name": profile_name,
            "skins": [],
            "capes": []
        })

        result = await backend.import_ygg_profile(user_id, api_url, profile_id, profile_name)

        assert result["id"] == profile_id
        assert result["name"] == f"{profile_name}_1"
        
        profile = await db_session.user.get_profile_by_id(profile_id)
        assert profile.name == f"{profile_name}_1"


@pytest.mark.asyncio
async def test_import_ygg_profiles_batch_success_and_partial_failure(db_session, test_config):
    backend = SiteBackend(db_session, test_config)
    user_id = "batch_user_id"
    api_url = "http://example.com/api/yggdrasil"

    await db_session.execute(
        "INSERT INTO users (id, email, password, is_admin, display_name) VALUES ($1, $2, $3, $4, $5)",
        user_id, "batch@example.com", "hash", False, "BatchUser"
    )

    profiles = [
        {"profile_id": "batch_profile_1", "profile_name": "BatchPlayer1"},
        {"profile_id": "batch_profile_2", "profile_name": "BatchPlayer2"},
        {"profile_id": "", "profile_name": "BrokenProfile"},
    ]

    with patch("backends.site_backend.YggdrasilClient") as MockClient, \
         patch("backends.site_backend.download_texture", new_callable=AsyncMock) as mock_download:
        mock_instance = MockClient.return_value

        async def get_profile_with_textures(profile_id):
            return {
                "id": profile_id,
                "name": profile_id.replace("batch_", ""),
                "skins": [{"url": f"http://skin.url/{profile_id}", "variant": "classic"}],
                "capes": [],
            }

        mock_instance.get_profile_with_textures = AsyncMock(side_effect=get_profile_with_textures)
        mock_download.return_value = b"skin_bytes"

        result = await backend.import_ygg_profiles(user_id, api_url, profiles)

    assert result["success_count"] == 2
    assert result["failure_count"] == 1
    assert len(result["items"]) == 2
    assert len(result["failed"]) == 1
    assert result["failed"][0]["detail"] == "profile_id and profile_name are required"

    assert await db_session.user.get_profile_by_id("batch_profile_1") is not None
    assert await db_session.user.get_profile_by_id("batch_profile_2") is not None
