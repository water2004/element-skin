import pytest

@pytest.mark.asyncio
async def test_api_admin_get_users(client, admin_headers):
    """测试管理员获取用户列表接口"""
    resp = await client.get("/admin/users", headers={"Authorization": admin_headers["Authorization"]})
    
    assert resp.status_code == 200
    data = resp.json()
    assert "items" in data
    assert "has_next" in data
    assert "next_cursor" in data
    assert isinstance(data["items"], list)
    # 至少应该有一个用户（即管理员自己）
    assert len(data["items"]) >= 1

@pytest.mark.asyncio
async def test_api_admin_get_user_profiles(client, admin_headers, user_factory, db_session):
    """测试管理员获取特定用户角色列表接口"""
    user = await user_factory(username="ProfileUser")
    from utils.typing import PlayerProfile
    await db_session.user.create_profile(PlayerProfile("p_admin_test", user.id, "AdminTestPlayer"))
    
    resp = await client.get(f"/admin/users/{user.id}/profiles", 
        params={"limit": 10},
        headers={"Authorization": admin_headers["Authorization"]}
    )
    
    assert resp.status_code == 200
    data = resp.json()
    assert len(data["items"]) >= 1
    assert "has_next" in data
    assert data["items"][0]["name"] == "AdminTestPlayer"

@pytest.mark.asyncio
async def test_api_admin_settings_site(client, admin_headers):
    """测试管理员修改站点设置接口"""
    payload = {
        "site_name": "API Test Site",
        "allow_register": True,
        "profile_uuid_mode": "offline",
    }
    resp = await client.post("/admin/settings/site", 
        json=payload,
        headers={"Authorization": admin_headers["Authorization"]}
    )
    
    assert resp.status_code == 200
    
    # 验证是否生效
    get_resp = await client.get("/admin/settings/site", headers={"Authorization": admin_headers["Authorization"]})
    assert get_resp.json()["site_name"] == "API Test Site"
    assert get_resp.json()["profile_uuid_mode"] == "offline"

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
    assert any(item["code"] == code for item in resp_list.json()["items"])
    
    # 3. 删除邀请码
    resp_del = await client.delete(f"/admin/invites/{code}", headers={"Authorization": admin_headers["Authorization"]})
    assert resp_del.status_code == 200

@pytest.mark.asyncio
async def test_api_admin_search_users(client, admin_headers, user_factory, db_session):
    """测试管理员搜索用户接口"""
    # 准备测试数据
    u1 = await user_factory(username="SearchUser1", email="SearchUser1@example.com")
    u2 = await user_factory(username="SearchUser2", email="SearchUser2@example.com")
    
    # 给 u2 创建一个角色
    from utils.typing import PlayerProfile
    await db_session.user.create_profile(PlayerProfile("p_search_test", u2.id, "KnightRole"))
    
    # 1. 按用户名搜索
    resp = await client.get("/admin/users", 
        params={"q": "SearchUser1"},
        headers={"Authorization": admin_headers["Authorization"]}
    )
    assert resp.status_code == 200
    data = resp.json()
    assert len(data["items"]) == 1
    assert data["items"][0]["email"] == "SearchUser1@example.com"
    
    # 2. 按角色名搜索
    resp = await client.get("/admin/users", 
        params={"q": "Knight"},
        headers={"Authorization": admin_headers["Authorization"]}
    )
    assert resp.status_code == 200
    data = resp.json()
    assert len(data["items"]) >= 1
    assert any(item["id"] == u2.id for item in data["items"])
    
    # 3. 搜索不存在的内容
    resp = await client.get("/admin/users", 
        params={"q": "NonExistentUserXYZ"},
        headers={"Authorization": admin_headers["Authorization"]}
    )
    assert resp.status_code == 200
    assert len(resp.json()["items"]) == 0


@pytest.mark.asyncio
async def test_admin_profiles_list(client, admin_headers, auth_headers, db_session, user_factory):
    """Test GET /admin/profiles — list + search + 403"""
    user = await user_factory()
    from utils.uuid_utils import generate_random_uuid
    from utils.typing import PlayerProfile
    pid = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "SearchTest", "default", None, None))

    # Non-admin → 403
    resp = await client.get("/admin/profiles", headers=auth_headers)
    assert resp.status_code == 403

    # Admin → 200
    resp = await client.get("/admin/profiles", headers=admin_headers)
    assert resp.status_code == 200
    data = resp.json()
    assert "items" in data
    assert len(data["items"]) >= 1

    # Search
    resp = await client.get("/admin/profiles?q=SearchTest", headers=admin_headers)
    assert resp.status_code == 200
    assert len(resp.json()["items"]) >= 1


@pytest.mark.asyncio
async def test_admin_profile_update(client, admin_headers, auth_headers, db_session, user_factory):
    """Test PATCH /admin/profiles/{id} — update + 403 + 409"""
    from utils.uuid_utils import generate_random_uuid
    from utils.typing import PlayerProfile
    user = await user_factory()
    pid = generate_random_uuid()
    pid2 = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "UpdateTest", "default", None, None))
    await db_session.user.create_profile(PlayerProfile(pid2, user.id, "OtherName", "default", None, None))

    # Non-admin → 403
    resp = await client.patch(f"/admin/profiles/{pid}", json={"name": "NewName"}, headers=auth_headers)
    assert resp.status_code == 403

    # Admin success
    resp = await client.patch(f"/admin/profiles/{pid}", json={"name": "Renamed"}, headers=admin_headers)
    assert resp.status_code == 200

    # Duplicate name → 409
    resp = await client.patch(f"/admin/profiles/{pid}", json={"name": "OtherName"}, headers=admin_headers)
    assert resp.status_code == 409


@pytest.mark.asyncio
async def test_admin_profile_delete(client, admin_headers, auth_headers, db_session, user_factory):
    """Test DELETE /admin/profiles/{id} — delete + cascade + 403 + 404"""
    import time
    from utils.uuid_utils import generate_random_uuid
    from utils.typing import PlayerProfile, Token
    user = await user_factory()
    pid = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "DeleteTest", "default", None, None))
    # Add a token for cascade test
    await db_session.user.add_token(Token("test-del-token", "client", user.id, pid, int(time.time() * 1000)))

    # Non-admin → 403
    resp = await client.delete(f"/admin/profiles/{pid}", headers=auth_headers)
    assert resp.status_code == 403

    # Admin success
    resp = await client.delete(f"/admin/profiles/{pid}", headers=admin_headers)
    assert resp.status_code == 200

    # Verify cascade: token gone
    assert await db_session.user.get_token("test-del-token") is None

    # Non-existent → 404
    resp = await client.delete(f"/admin/profiles/{pid}", headers=admin_headers)
    assert resp.status_code == 404


@pytest.mark.asyncio
async def test_admin_textures_endpoints(client, admin_headers, auth_headers, db_session, user_factory):
    """Test all /admin/textures endpoints — list + toggle + delete + 403"""
    from io import BytesIO
    from PIL import Image

    def make_img(color):
        f = BytesIO()
        Image.new('RGBA', (64, 64), color).save(f, 'png')
        f.seek(0)
        return f.read()

    user = await user_factory()
    tex_hash, tex_type = await db_session.texture.upload(user.id, make_img((255, 0, 0)), "skin", note="APITest", is_public=True)

    # GET — 403 for non-admin
    resp = await client.get("/admin/textures", headers=auth_headers)
    assert resp.status_code == 403

    # GET — 200 for admin
    resp = await client.get("/admin/textures", headers=admin_headers)
    assert resp.status_code == 200
    assert len(resp.json()["items"]) >= 1

    # PATCH — 403
    resp = await client.patch(f"/admin/textures/{tex_hash}", json={"is_public": 0}, headers=auth_headers)
    assert resp.status_code == 403

    # PATCH — 200 (toggle to private)
    resp = await client.patch(f"/admin/textures/{tex_hash}", json={"is_public": 0}, headers=admin_headers)
    assert resp.status_code == 200

    # DELETE — 403
    resp = await client.delete(f"/admin/textures/{tex_hash}?user_id={user.id}&type=skin", headers=auth_headers)
    assert resp.status_code == 403

    # DELETE — 200
    resp = await client.delete(f"/admin/textures/{tex_hash}?user_id={user.id}&type=skin", headers=admin_headers)
    assert resp.status_code == 200

