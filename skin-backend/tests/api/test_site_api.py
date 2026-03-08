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
