import pytest
from unittest.mock import AsyncMock, patch
from backends.microsoft_backend import MicrosoftAuthService

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
