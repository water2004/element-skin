import pytest
from unittest.mock import AsyncMock, patch
from fastapi import HTTPException
from backends.microsoft_backend import MicrosoftAuthService, MicrosoftBackend
from routes_reference import texture_storage
from utils.typing import PlayerProfile

@pytest.mark.asyncio
async def test_microsoft_auth_url_generation():
    """测试微软 OAuth URL 生成"""
    service = MicrosoftAuthService("client_id", "client_secret", "https://redirect.com")
    url = service.get_authorization_url("state123")
    
    assert "client_id=client_id" in url
    assert "state=state123" in url
    assert "redirect_uri=https%3A%2F%2Fredirect.com" in url

@pytest.mark.asyncio
async def test_microsoft_auth_flow_mock(test_config):
    """测试微软认证全流程 (Mocked)"""
    service = MicrosoftAuthService("id", "secret", "uri")
    
    with patch('aiohttp.ClientSession.post') as mock_post, \
         patch('aiohttp.ClientSession.get') as mock_get:
        
        # 1. Mock Exchange Code for Token
        mock_token_resp = AsyncMock()
        mock_token_resp.status = 200
        mock_token_resp.json.return_value = {"access_token": "ms_access_token"}
        mock_token_resp.__aenter__.return_value = mock_token_resp
        
        # 2. Mock XBL Auth
        mock_xbl_resp = AsyncMock()
        mock_xbl_resp.status = 200
        mock_xbl_resp.json.return_value = {
            "Token": "xbl_token",
            "DisplayClaims": {"xui": [{"uhs": "user_hash"}]}
        }
        mock_xbl_resp.__aenter__.return_value = mock_xbl_resp
        
        # 3. Mock XSTS Auth
        mock_xsts_resp = AsyncMock()
        mock_xsts_resp.status = 200
        mock_xsts_resp.json.return_value = {
            "Token": "xsts_token",
            "DisplayClaims": {"xui": [{"uhs": "user_hash"}]}
        }
        mock_xsts_resp.__aenter__.return_value = mock_xsts_resp
        
        # 4. Mock Minecraft Login
        mock_mc_login_resp = AsyncMock()
        mock_mc_login_resp.status = 200
        mock_mc_login_resp.json.return_value = {"access_token": "mc_access_token"}
        mock_mc_login_resp.__aenter__.return_value = mock_mc_login_resp
        
        # 5. Mock Ownership Check
        mock_entitlements_resp = AsyncMock()
        mock_entitlements_resp.status = 200
        mock_entitlements_resp.json.return_value = {"items": [{"name": "game_minecraft"}]}
        mock_entitlements_resp.__aenter__.return_value = mock_entitlements_resp
        
        # 6. Mock Profile Fetch
        mock_profile_resp = AsyncMock()
        mock_profile_resp.status = 200
        mock_profile_resp.json.return_value = {"id": "uuid", "name": "McPlayer"}
        mock_profile_resp.__aenter__.return_value = mock_profile_resp
        
        # Set up side effects for post and get
        mock_post.side_effect = [mock_token_resp, mock_xbl_resp, mock_xsts_resp, mock_mc_login_resp]
        mock_get.side_effect = [mock_entitlements_resp, mock_profile_resp]
        
        # 执行流程
        # exchange_code_for_token
        token_data = await service.exchange_code_for_token("auth_code")
        assert token_data["access_token"] == "ms_access_token"
        
        # complete_auth_flow
        result = await service.complete_auth_flow("ms_access_token")
        
        assert result["mc_access_token"] == "mc_access_token"
        assert result["has_game"] is True
        assert result["profile"]["name"] == "McPlayer"
        
        # 验证调用次数
        assert mock_post.call_count == 4
        assert mock_get.call_count == 2


# ========== Phase 4: MicrosoftBackend.import_profile orchestration ==========


@pytest.mark.asyncio
async def test_microsoft_import_profile_skin_and_cape(db_session, test_config, user_factory):
    """皮肤+披风都下载：hash 落到角色，模型按 variant=slim 设为 slim"""
    backend = MicrosoftBackend(db_session, test_config, texture_storage)
    user = await user_factory()

    with patch("backends.microsoft_backend.download_texture", new_callable=AsyncMock) as mock_dl, \
         patch.object(texture_storage, "process_and_save", side_effect=["skin_h", "cape_h"]):
        mock_dl.return_value = b"bytes"
        result = await backend.import_profile(
            user.id, "ms_uuid_1", "MsImported",
            skin_url="http://skin", skin_variant="slim", cape_url="http://cape",
        )

    assert result["ok"] is True
    assert result["profile"]["model"] == "slim"
    profile = await db_session.user.get_profile_by_id("ms_uuid_1")
    assert profile.skin_hash == "skin_h"
    assert profile.cape_hash == "cape_h"
    assert profile.texture_model == "slim"


@pytest.mark.asyncio
async def test_microsoft_import_profile_uuid_conflict(db_session, test_config, user_factory):
    backend = MicrosoftBackend(db_session, test_config, texture_storage)
    user = await user_factory()
    await db_session.user.create_profile(
        PlayerProfile("dup_uuid", user.id, "AlreadyHere", "default")
    )

    with pytest.raises(HTTPException) as exc:
        await backend.import_profile(
            user.id, "dup_uuid", "Whatever", None, "classic", None
        )
    assert exc.value.status_code == 400
    assert "UUID" in exc.value.detail


@pytest.mark.asyncio
async def test_microsoft_import_profile_name_dedup(db_session, test_config, user_factory):
    """同名角色已存在时自动加 _1 后缀"""
    backend = MicrosoftBackend(db_session, test_config, texture_storage)
    user = await user_factory()
    await db_session.user.create_profile(
        PlayerProfile("other_uuid", user.id, "TakenName", "default")
    )

    result = await backend.import_profile(
        user.id, "fresh_uuid", "TakenName", None, "classic", None
    )
    assert result["profile"]["name"] == "TakenName_1"
    profile = await db_session.user.get_profile_by_id("fresh_uuid")
    assert profile.name == "TakenName_1"
    assert profile.skin_hash is None


@pytest.mark.asyncio
async def test_microsoft_import_profile_skin_download_failure_is_tolerated(db_session, test_config, user_factory):
    """皮肤下载失败不应中断导入：角色仍创建，skin_hash 为 None"""
    backend = MicrosoftBackend(db_session, test_config, texture_storage)
    user = await user_factory()

    with patch("backends.microsoft_backend.download_texture", new_callable=AsyncMock) as mock_dl:
        mock_dl.side_effect = Exception("network down")
        result = await backend.import_profile(
            user.id, "tolerant_uuid", "Tolerant", "http://skin", "classic", None
        )

    assert result["ok"] is True
    assert result["profile"]["skin_hash"] is None
    profile = await db_session.user.get_profile_by_id("tolerant_uuid")
    assert profile is not None
    assert profile.skin_hash is None
