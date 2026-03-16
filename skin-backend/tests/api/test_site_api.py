import pytest
from httpx import AsyncClient

@pytest.mark.asyncio
async def test_api_site_login_success(client, user_factory):
    """测试 Web 登录接口成功路径"""
    email = "api_login@test.com"
    password = "ApiPassword123"
    await user_factory(email=email, password=password)
    
    resp = await client.post("/site-login", json={
        "email": email,
        "password": password
    })
    
    assert resp.status_code == 200
    data = resp.json()
    assert "token" in data
    assert "user_id" in data

@pytest.mark.asyncio
async def test_api_get_me_info(client, auth_headers):
    """测试获取当前用户信息接口"""
    resp = await client.get("/me", headers={"Authorization": auth_headers["Authorization"]})
    
    assert resp.status_code == 200
    data = resp.json()
    assert data["id"] == auth_headers["X-User-ID"]
    assert "profiles" in data

@pytest.mark.asyncio
async def test_api_update_me_info(client, auth_headers):
    """测试更新个人信息接口"""
    new_name = "UpdatedDisplayName"
    resp = await client.patch("/me", 
        json={"display_name": new_name},
        headers={"Authorization": auth_headers["Authorization"]}
    )
    
    assert resp.status_code == 200
    
    # 验证是否生效
    me_resp = await client.get("/me", headers={"Authorization": auth_headers["Authorization"]})
    assert me_resp.json()["display_name"] == new_name

@pytest.mark.asyncio
async def test_api_texture_upload_multipart(client, auth_headers):
    """测试通过 Multipart 表单上传材质接口"""
    from io import BytesIO
    from PIL import Image
    
    file_content = BytesIO()
    Image.new('RGBA', size=(64, 64), color=(255, 255, 0, 255)).save(file_content, 'png')
    file_content.seek(0)
    
    files = {
        "file": ("skin.png", file_content, "image/png")
    }
    data = {
        "texture_type": "skin",
        "note": "API Upload",
        "is_public": "true",
        "model": "default"
    }
    
    resp = await client.post("/me/textures", 
        data=data, 
        files=files,
        headers={"Authorization": auth_headers["Authorization"]}
    )
    
    assert resp.status_code == 200
    assert "hash" in resp.json()

@pytest.mark.asyncio
async def test_api_create_profile(client, auth_headers):
    """测试创建角色接口"""
    resp = await client.post("/me/profiles",
        json={"name": "ApiPlayer", "model": "default"},
        headers={"Authorization": auth_headers["Authorization"]}
    )
    
    assert resp.status_code == 200
    assert resp.json()["name"] == "ApiPlayer"

@pytest.mark.asyncio
async def test_api_rename_profile(client, auth_headers, db_session):
    """测试重命名角色接口"""
    # 先创建一个角色
    from utils.typing import PlayerProfile
    pid = "p_to_rename"
    await db_session.user.create_profile(PlayerProfile(pid, auth_headers["X-User-ID"], "OldName"))
    
    resp = await client.patch(f"/me/profiles/{pid}",
        json={"name": "NewFancyName"},
        headers={"Authorization": auth_headers["Authorization"]}
    )
    
    assert resp.status_code == 200
    # 验证数据库
    p = await db_session.user.get_profile_by_id(pid)
    assert p.name == "NewFancyName"

@pytest.mark.asyncio
async def test_api_remote_ygg_import_flow(client, auth_headers, db_session):
    """测试从远程 Yggdrasil 导入角色的 API 流程 (Mocked)"""
    from unittest.mock import patch, AsyncMock
    
    # 1. 测试获取列表
    mock_profiles = [{"id": "remote_pid", "name": "RemotePlayer"}]
    with patch("backends.yggdrasil_client.YggdrasilClient.authenticate", new_callable=AsyncMock) as mock_auth:
        mock_auth.return_value = {"availableProfiles": mock_profiles}
        
        resp = await client.post("/remote-ygg/get-profiles",
            json={"api_url": "https://remote.com", "username": "u", "password": "p"},
            headers={"Authorization": auth_headers["Authorization"]}
        )
        assert resp.status_code == 200
        assert resp.json()["profiles"] == mock_profiles

    # 2. 测试导入
    mock_profile_data = {
        "id": "remote_pid",
        "name": "RemotePlayer",
        "skins": [{"url": "http://tex.com/skin.png", "variant": "classic"}],
        "capes": []
    }
    
    with patch("backends.yggdrasil_client.YggdrasilClient.get_profile_with_textures", new_callable=AsyncMock) as mock_get_p, \
         patch("backends.yggdrasil_client.download_texture", new_callable=AsyncMock) as mock_down:
        
        mock_get_p.return_value = mock_profile_data
        mock_down.return_value = b"fake_image_bytes"
        
        # 还需要 Mock 数据库的材质上传，因为 fake_image_bytes 不是真的 PNG
        with patch.object(db_session.texture, "upload", new_callable=AsyncMock) as mock_upload:
            mock_upload.return_value = ("fake_hash", "skin")
            
            resp = await client.post("/remote-ygg/import-profile",
                json={
                    "api_url": "https://remote.com",
                    "profile_id": "remote_pid",
                    "profile_name": "RemotePlayer"
                },
                headers={"Authorization": auth_headers["Authorization"]}
            )
            
            assert resp.status_code == 200
            assert "id" in resp.json()
            
            # 验证本地是否创建了角色
            local_pid = resp.json()["id"]
            p = await db_session.user.get_profile_by_id(local_pid)
            assert p.name == "RemotePlayer"
            assert p.skin_hash == "fake_hash"
