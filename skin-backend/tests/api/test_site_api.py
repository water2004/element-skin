import pytest
from httpx import AsyncClient
from unittest.mock import patch

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
    assert "user_id" in data
    assert data["is_admin"] is False
    # token 现在通过 Set-Cookie header 返回（access + refresh 两个）
    assert "set-cookie" in resp.headers
    set_cookie = resp.headers["set-cookie"]
    assert "access_token=" in set_cookie
    assert "refresh_token=" in set_cookie

@pytest.mark.asyncio
async def test_api_get_me_info(client, auth_headers):
    """测试获取当前用户信息接口"""
    resp = await client.get("/me", cookies=auth_headers["cookies"])

    assert resp.status_code == 200
    data = resp.json()
    assert data["id"] == auth_headers["X-User-ID"]
    assert "profile_count" in data
    assert "texture_count" in data
    assert "profiles" not in data


@pytest.mark.asyncio
async def test_banned_user_can_still_access_site_api(client, auth_headers, db_session):
    """封禁仅限制通过 Yggdrasil 登录游戏，被封禁用户仍可正常访问主站。"""
    user_id = auth_headers["X-User-ID"]
    assert (await client.get("/me", cookies=auth_headers["cookies"])).status_code == 200

    import time
    await db_session.user.ban(user_id, int((time.time() + 3600) * 1000))

    # 封禁后主站访问不受影响
    resp = await client.get("/me", cookies=auth_headers["cookies"])
    assert resp.status_code == 200


@pytest.mark.asyncio
async def test_deleted_user_token_is_rejected(client, auth_headers, db_session):
    """删号后旧 JWT 立即失效（401）。"""
    user_id = auth_headers["X-User-ID"]
    await db_session.user.delete(user_id)

    resp = await client.get("/me", cookies=auth_headers["cookies"])
    assert resp.status_code == 401


@pytest.mark.asyncio
async def test_admin_token_loses_access_after_demotion(client, db_session, user_factory):
    """携带 is_admin=true 的旧 JWT，在用户被降权后不应再通过 admin 校验。"""
    from utils.jwt_utils import create_access_token

    user = await user_factory(is_admin=True)
    # 颁发一个 is_admin=True 的 token
    token = create_access_token(user.id, is_admin=True)
    cookies = {"access_token": token}

    # 降权（DB 内 is_admin -> False）
    await db_session.user.toggle_admin(user.id)

    # 旧 token 仍声称 is_admin，但 deps 以 DB 为准，应拒绝管理员接口
    resp = await client.get("/admin/users", cookies=cookies)
    assert resp.status_code == 403

@pytest.mark.asyncio
async def test_api_get_me_profiles_paginated(client, auth_headers, db_session):
    """测试分页获取个人角色列表接口：跟随 next_cursor 翻页，全量覆盖且无重叠"""
    user_id = auth_headers["X-User-ID"]
    from utils.typing import PlayerProfile
    for i in range(5):
        await db_session.user.create_profile(PlayerProfile(f"p_{i}", user_id, f"Player_{i}"))

    seen = []
    cursor = None
    for _ in range(20):  # 安全上限
        params = {"limit": 2}
        if cursor:
            params["cursor"] = cursor
        resp = await client.get("/me/profiles", params=params, cookies=auth_headers["cookies"])
        assert resp.status_code == 200
        data = resp.json()
        seen.extend(item["id"] for item in data["items"])
        if not data["has_next"]:
            break
        cursor = data["next_cursor"]
        assert isinstance(cursor, str) and cursor

    assert set(seen) == {f"p_{i}" for i in range(5)}
    assert len(seen) == 5

@pytest.mark.asyncio
async def test_api_get_me_textures_paginated(client, auth_headers, db_session):
    """测试分页获取个人材质列表接口：跟随 next_cursor 翻页，全量覆盖且无重叠"""
    user_id = auth_headers["X-User-ID"]
    for i in range(3):
        await db_session.texture.add_to_library(user_id, f"hash_{i}", "skin", note=f"Note {i}")

    seen = []
    cursor = None
    for _ in range(20):  # 安全上限
        params = {"limit": 2}
        if cursor:
            params["cursor"] = cursor
        resp = await client.get("/me/textures", params=params, cookies=auth_headers["cookies"])
        assert resp.status_code == 200
        data = resp.json()
        assert "hash" in data["items"][0]
        seen.extend(item["hash"] for item in data["items"])
        if not data["has_next"]:
            break
        cursor = data["next_cursor"]
        assert isinstance(cursor, str) and cursor

    assert set(seen) == {f"hash_{i}" for i in range(3)}
    assert len(seen) == 3

@pytest.mark.asyncio
async def test_api_update_me_info(client, auth_headers):
    """测试更新个人信息接口"""
    new_name = "UpdatedDisplayName"
    new_avatar = "fake_avatar_hash_123"
    resp = await client.patch("/me", 
        json={"display_name": new_name, "avatar_hash": new_avatar},
        cookies=auth_headers["cookies"]
    )
    
    assert resp.status_code == 200
    
    # 验证是否生效
    me_resp = await client.get("/me", cookies=auth_headers["cookies"])
    assert me_resp.json()["display_name"] == new_name
    assert me_resp.json()["avatar_hash"] == new_avatar

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
        cookies=auth_headers["cookies"]
    )
    
    assert resp.status_code == 200
    assert "hash" in resp.json()

@pytest.mark.asyncio
async def test_api_create_profile(client, auth_headers):
    """测试创建角色接口"""
    resp = await client.post("/me/profiles",
        json={"name": "ApiPlayer", "model": "default"},
        cookies=auth_headers["cookies"]
    )
    
    assert resp.status_code == 200
    assert resp.json()["name"] == "ApiPlayer"


@pytest.mark.asyncio
async def test_api_create_profile_uuid_conflict(client, auth_headers, db_session):
    from utils.typing import PlayerProfile

    conflict_id = "feedfeedfeedfeedfeedfeedfeedfeed"
    await db_session.user.create_profile(
        PlayerProfile(conflict_id, auth_headers["X-User-ID"], "TakenApiRole")
    )

    with patch("backends.site_backend.generate_random_uuid", return_value=conflict_id):
        resp = await client.post(
            "/me/profiles",
            json={"name": "ApiRoleC1", "model": "default"},
            cookies=auth_headers["cookies"],
        )

    assert resp.status_code == 400
    assert resp.json()["detail"] == "角色 UUID 冲突，无法新建角色"

@pytest.mark.asyncio
async def test_api_rename_profile(client, auth_headers, db_session):
    """测试重命名角色接口"""
    # 先创建一个角色
    from utils.typing import PlayerProfile
    pid = "p_to_rename"
    await db_session.user.create_profile(PlayerProfile(pid, auth_headers["X-User-ID"], "OldName"))
    
    resp = await client.patch(f"/me/profiles/{pid}",
        json={"name": "NewFancyName"},
        cookies=auth_headers["cookies"]
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
    mock_profiles = [
        {"id": "remote_pid_1", "name": "RemotePlayer1"},
        {"id": "remote_pid_2", "name": "RemotePlayer2"},
    ]
    with patch("backends.yggdrasil_client.YggdrasilClient.authenticate", new_callable=AsyncMock) as mock_auth:
        mock_auth.return_value = {"availableProfiles": mock_profiles}
        
        resp = await client.post("/remote-ygg/get-profiles",
            json={"api_url": "https://remote.com", "username": "u", "password": "p"},
            cookies=auth_headers["cookies"]
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
         patch("backends.profile_import_backend.download_texture", new_callable=AsyncMock) as mock_down:
        
        mock_get_p.return_value = mock_profile_data
        mock_down.return_value = b"fake_image_bytes"
        
        # process_and_save 需 mock，因为 fake_image_bytes 不是真的 PNG
        with patch("services.texture_storage.TextureStorage.process_and_save", return_value="fake_hash"):
            resp = await client.post("/remote-ygg/import-profiles",
                json={
                    "api_url": "https://remote.com",
                    "profiles": [
                        {"profile_id": "remote_pid_1", "profile_name": "RemotePlayer1"},
                        {"profile_id": "remote_pid_2", "profile_name": "RemotePlayer2"},
                    ]
                },
                cookies=auth_headers["cookies"]
            )
            
            assert resp.status_code == 200
            data = resp.json()
            assert data["success_count"] == 2
            assert data["failure_count"] == 0
            assert len(data["items"]) == 2

            # 验证本地是否创建了角色
            p1 = await db_session.user.get_profile_by_id("remote_pid_1")
            p2 = await db_session.user.get_profile_by_id("remote_pid_2")
            assert p1.name == "RemotePlayer1"
            assert p2.name == "RemotePlayer2"
            assert p1.skin_hash == "fake_hash"
            assert p2.skin_hash == "fake_hash"

@pytest.mark.asyncio
async def test_api_add_texture_from_library_preserves_name(client, auth_headers, db_session):
    """测试从皮肤库添加材质到衣柜时保留名称"""
    user_id = auth_headers["X-User-ID"]
    tex_hash = "lib_tex_hash_123"
    tex_name = "Epic Skin Name"
    
    # 1. 先在皮肤库中创建一个有名称的材质 (通过 db_session 直接操作，模拟库中已有数据)
    # add_to_library(self, user_id, texture_hash, texture_type, note="", is_public=False, model="default")
    await db_session.texture.add_to_library(user_id, tex_hash, "skin", note=tex_name, is_public=True, model="default")
    
    # 2. 调用添加接口 (POST /me/textures/{hash}/add)
    resp = await client.post(f"/me/textures/{tex_hash}/add", 
        cookies=auth_headers["cookies"]
    )
    assert resp.status_code == 200
    
    # 3. 验证个人材质列表中是否有该材质，且 note 正确 (user_textures.note 应该等于 skin_library.name)
    me_tex_resp = await client.get("/me/textures", cookies=auth_headers["cookies"])
    items = me_tex_resp.json()["items"]
    assert any(item["hash"] == tex_hash and item["note"] == tex_name for item in items)



@pytest.mark.asyncio
async def test_api_list_textures_limit_clamped(client, auth_headers, db_session):
    """异常 limit（-1 / 0 / 超大）不应触发 500，且结果数量受 MAX_LIMIT 收敛。"""
    from utils.pagination import MAX_LIMIT

    user_id = auth_headers["X-User-ID"]
    # 造 5 条材质，确保有数据可分页
    for i in range(5):
        await db_session.texture.add_to_library(
            user_id, f"clamp_tex_{i}", "skin", note=f"t{i}", is_public=False, model="default"
        )

    for bad_limit in (-1, 0, 99999999):
        resp = await client.get(
            f"/me/textures?limit={bad_limit}", cookies=auth_headers["cookies"]
        )
        assert resp.status_code == 200, f"limit={bad_limit} 应返回 200，实际 {resp.status_code}"
        items = resp.json()["items"]
        assert len(items) <= MAX_LIMIT


@pytest.mark.asyncio
async def test_api_public_skin_library_limit_clamped(client):
    """公开皮肤库异常 limit 不触发 500。"""
    for bad_limit in (-1, 0, 99999999):
        resp = await client.get(f"/public/skin-library?limit={bad_limit}")
        assert resp.status_code == 200
