import pytest

@pytest.mark.asyncio
async def test_api_admin_get_users(client, admin_headers):
    """测试管理员获取用户列表接口"""
    resp = await client.get("/admin/users", headers={"Authorization": admin_headers["Authorization"]})
    
    assert resp.status_code == 200
    assert isinstance(resp.json(), list)
    # 至少应该有一个用户（即管理员自己）
    assert len(resp.json()) >= 1

@pytest.mark.asyncio
async def test_api_admin_settings_site(client, admin_headers):
    """测试管理员修改站点设置接口"""
    payload = {
        "site_name": "API Test Site",
        "allow_register": True
    }
    resp = await client.post("/admin/settings/site", 
        json=payload,
        headers={"Authorization": admin_headers["Authorization"]}
    )
    
    assert resp.status_code == 200
    
    # 验证是否生效
    get_resp = await client.get("/admin/settings/site", headers={"Authorization": admin_headers["Authorization"]})
    assert get_resp.json()["site_name"] == "API Test Site"

@pytest.mark.asyncio
async def test_api_admin_forbidden_for_normal_user(client, auth_headers):
    """测试普通用户访问管理接口被拒绝"""
    resp = await client.get("/admin/users", headers={"Authorization": auth_headers["Authorization"]})
    assert resp.status_code == 403

@pytest.mark.asyncio
async def test_api_admin_ban_user(client, admin_headers, user_factory):
    """测试管理员封禁用户接口"""
    user = await user_factory(username="ToBan")
    
    import time
    banned_until = int((time.time() + 3600) * 1000)
    
    resp = await client.post(f"/admin/users/{user.id}/ban",
        json={"banned_until": banned_until},
        headers={"Authorization": admin_headers["Authorization"]}
    )
    
    assert resp.status_code == 200
    assert resp.json()["banned_until"] == banned_until

@pytest.mark.asyncio
async def test_api_admin_invite_codes(client, admin_headers):
    """测试管理员管理邀请码接口"""
    # 1. 创建邀请码
    resp_create = await client.post("/admin/invites",
        json={"total_uses": 5, "note": "API Code"},
        headers={"Authorization": admin_headers["Authorization"]}
    )
    assert resp_create.status_code == 200
    code = resp_create.json()["code"]
    
    # 2. 获取邀请码列表
    resp_list = await client.get("/admin/invites", headers={"Authorization": admin_headers["Authorization"]})
    assert any(item["code"] == code for item in resp_list.json())
    
    # 3. 删除邀请码
    resp_del = await client.delete(f"/admin/invites/{code}", headers={"Authorization": admin_headers["Authorization"]})
    assert resp_del.status_code == 200
