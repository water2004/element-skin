import pytest
from unittest.mock import AsyncMock, patch

@pytest.mark.asyncio
async def test_microsoft_import_profile_success(client, auth_headers, db_session):
    # Prepare data
    profile_id = "ms_profile_id"
    profile_name = "MsPlayer"
    skin_url = "http://skin.url"
    
    payload = {
        "profile_id": profile_id,
        "profile_name": profile_name,
        "skin_url": skin_url,
        "skin_variant": "classic"
    }

    # Mock download_texture
    # Note: The import in routers/microsoft_routes.py is: from backends.microsoft_backend import download_texture
    # So we must patch where it is used: routers.microsoft_routes.download_texture
    with patch("routers.microsoft_routes.download_texture", new_callable=AsyncMock) as mock_download:
        mock_download.return_value = b"skin_bytes"
        
        response = await client.post(
            "/microsoft/import-profile",
            headers=auth_headers,
            json=payload
        )
        
        assert response.status_code == 200
        data = response.json()
        assert data["ok"] is True
        assert data["profile"]["id"] == profile_id
        assert data["profile"]["name"] == profile_name
        
        # Verify DB
        profile = await db_session.user.get_profile_by_id(profile_id)
        assert profile is not None
        assert profile.name == profile_name

@pytest.mark.asyncio
async def test_microsoft_import_profile_uuid_conflict(client, auth_headers, db_session, user_factory):
    # Prepare conflict
    profile_id = "conflict_ms_id"
    profile_name = "ConflictMsPlayer"
    
    # Create existing profile
    user = await user_factory()
    from utils.typing import PlayerProfile
    await db_session.user.create_profile(
        PlayerProfile(profile_id, user.id, "ExistingOne", "default")
    )

    payload = {
        "profile_id": profile_id,
        "profile_name": profile_name,
        "skin_url": None
    }

    with patch("routers.microsoft_routes.download_texture", new_callable=AsyncMock):
        response = await client.post(
            "/microsoft/import-profile",
            headers=auth_headers,
            json=payload
        )
        
        assert response.status_code == 400
        assert "UUID" in response.json()["detail"]

@pytest.mark.asyncio
async def test_microsoft_import_profile_name_conflict(client, auth_headers, db_session, user_factory):
    # Prepare name conflict
    profile_id = "new_ms_id"
    profile_name = "TakenMsName"
    
    # Create existing profile with same name but different ID
    user = await user_factory()
    from utils.typing import PlayerProfile
    await db_session.user.create_profile(
        PlayerProfile("other_id", user.id, profile_name, "default")
    )

    payload = {
        "profile_id": profile_id,
        "profile_name": profile_name,
        "skin_url": "http://skin.url"
    }

    with patch("routers.microsoft_routes.download_texture", new_callable=AsyncMock) as mock_download:
        mock_download.return_value = b"skin_bytes"
        
        response = await client.post(
            "/microsoft/import-profile",
            headers=auth_headers,
            json=payload
        )
        
        assert response.status_code == 200
        data = response.json()
        assert data["profile"]["id"] == profile_id
        assert data["profile"]["name"] == f"{profile_name}_1"
        
        # Verify DB
        profile = await db_session.user.get_profile_by_id(profile_id)
        assert profile.name == f"{profile_name}_1"
