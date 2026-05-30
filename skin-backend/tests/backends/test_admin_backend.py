import pytest
from fastapi import HTTPException
from backends.admin_backend import AdminBackend
from utils.typing import PlayerProfile
from utils.uuid_utils import generate_random_uuid

@pytest.mark.asyncio
async def test_admin_settings_management(db_session, test_config):
    """测试管理员对设置的分组管理逻辑"""
    backend = AdminBackend(db_session, test_config)
    
    # 1. 保存站点设置
    site_settings = {
        "site_name": "New Test Site",
        "allow_register": False,
        "max_texture_size": 2048,
        "profile_uuid_mode": "offline",
    }
    await backend.save_settings_group("site", site_settings)
    
    # 验证保存结果
    fetched = await backend.get_site_settings()
    assert fetched["site_name"] == "New Test Site"
    assert fetched["allow_register"] is False
    assert fetched["max_texture_size"] == 2048
    assert fetched["profile_uuid_mode"] == "offline"
    
    # 2. 保存安全设置
    security_settings = {
        "rate_limit_enabled": True,
        "rate_limit_auth_attempts": 10
    }
    await backend.save_settings_group("security", security_settings)
    assert (await backend.get_security_settings())["rate_limit_auth_attempts"] == 10

@pytest.mark.asyncio
async def test_admin_user_controls(db_session, test_config, user_factory):
    """测试管理员对用户的管控逻辑：列表、封禁、删除、权限切换"""
    backend = AdminBackend(db_session, test_config)
    admin = await user_factory(is_admin=True, username="AdminUser")
    user = await user_factory(is_admin=False, username="NormalUser")
    
    # 1. 获取用户列表（cursor）
    users_page = await db_session.user.list_users_cursor(limit=15)
    assert users_page["page_size"] >= 2
    assert len(users_page["items"]) >= 2
    
    # 2. 封禁用户 (先对普通用户操作)
    import time
    banned_until = int((time.time() + 3600) * 1000)
    await backend.ban_user(user.id, banned_until, admin.id)
    assert await db_session.user.is_banned(user.id) is True

    # 3. 切换管理员状态
    await backend.toggle_user_admin(user.id, admin.id)
    assert (await db_session.user.get_by_id(user.id)).is_admin is True
    # 禁止取消自己的管理员
    with pytest.raises(HTTPException) as exc:
        await backend.toggle_user_admin(admin.id, admin.id)
    assert exc.value.status_code == 403

    # 4. 降级并删除用户
    await backend.toggle_user_admin(user.id, admin.id) # 降级回普通用户
    assert (await db_session.user.get_by_id(user.id)).is_admin is False

    await backend.delete_user(user.id, is_admin_action=True)
    assert await db_session.user.get_by_id(user.id) is None



@pytest.mark.asyncio
async def test_admin_invite_code_creation(db_session, test_config):
    """测试邀请码生成逻辑"""
    backend = AdminBackend(db_session, test_config)
    
    # 1. 自动生成随机码
    code1 = await backend.create_invite(code=None, total_uses=10, note="auto-gen")
    assert len(code1) > 10
    
    # 2. 指定自定义码
    code2 = await backend.create_invite(code="MY_CUSTOM_CODE", total_uses=5)
    assert code2 == "MY_CUSTOM_CODE"
    
    # 3. 格式校验 (太短)
    with pytest.raises(HTTPException) as exc:
        await backend.create_invite(code="123", total_uses=1)
    assert exc.value.status_code == 400


# ========== Admin Profile & Texture Management Tests ==========


@pytest.mark.asyncio
async def test_admin_get_all_profiles(admin_backend_fixture, db_session, user_factory):
    """测试管理端获取所有角色列表及搜索"""
    user1 = await user_factory()
    user2 = await user_factory()

    pid1 = generate_random_uuid()
    pid2 = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid1, user1.id, "AdminList1", "default", None, None))
    await db_session.user.create_profile(PlayerProfile(pid2, user2.id, "AdminList2", "slim", None, None))

    # 1. List all
    page = await admin_backend_fixture.get_all_profiles(limit=10)
    assert len(page["items"]) >= 2

    # 2. Search by profile name
    search_page = await admin_backend_fixture.get_all_profiles(limit=10, query="AdminList1")
    assert len(search_page["items"]) == 1
    assert search_page["items"][0]["name"] == "AdminList1"


@pytest.mark.asyncio
async def test_admin_update_profile(admin_backend_fixture, db_session, user_factory):
    """测试管理端更新角色名称"""
    user = await user_factory()
    pid = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "OldName", "default", None, None))

    # 1. Update name
    result = await admin_backend_fixture.update_profile(pid, name="NewName")
    assert result["ok"] is True
    profile = await db_session.user.get_profile_by_id(pid)
    assert profile.name == "NewName"

    # 2. Duplicate name → 409
    pid2 = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid2, user.id, "TakenName", "default", None, None))
    with pytest.raises(HTTPException) as exc:
        await admin_backend_fixture.update_profile(pid, name="TakenName")
    assert exc.value.status_code == 409


@pytest.mark.asyncio
async def test_admin_delete_profile(admin_backend_fixture, db_session, user_factory):
    """测试管理端删除角色"""
    user = await user_factory()
    pid = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "ToDelete", "default", None, None))

    # 1. Delete
    result = await admin_backend_fixture.delete_profile(pid)
    assert result["ok"] is True
    assert await db_session.user.get_profile_by_id(pid) is None

    # 2. Non-existent → 404
    with pytest.raises(HTTPException) as exc:
        await admin_backend_fixture.delete_profile("non-existent-id")
    assert exc.value.status_code == 404


@pytest.mark.asyncio
async def test_admin_texture_methods(admin_backend_fixture, db_session, user_factory):
    """测试管理端材质管理：列表、公开状态、删除"""
    user = await user_factory()
    tex_hash, tex_type = "a" * 64, "skin"

    # 1. Record a texture
    await db_session.texture.add_to_library(
        user.id, tex_hash, "skin", note="AdminTexture", is_public=True, model="default"
    )

    # 2. get_all_textures → texture appears
    page = await admin_backend_fixture.get_all_textures(limit=10)
    hashes = [item["hash"] for item in page["items"]]
    assert tex_hash in hashes

    # 3. update_texture_public → set to 0
    result = await admin_backend_fixture.update_texture_public(tex_hash, 0)
    assert result["success"] is True

    # 4. delete_texture (per-user)
    result = await admin_backend_fixture.delete_texture(tex_hash, "skin", user_id=user.id)
    assert result["success"] is True

@pytest.mark.asyncio
async def test_admin_clear_profile_skin(admin_backend_fixture, db_session, user_factory):
    """Clearing skin sets skin_hash to NULL without affecting cape_hash"""
    user = await user_factory()
    pid = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "SkinClear", "default", "skinhash", "capehash"))

    result = await admin_backend_fixture.update_profile_skin(pid, None)
    assert result["ok"] is True
    profile = await db_session.user.get_profile_by_id(pid)
    assert profile.skin_hash is None
    assert profile.cape_hash == "capehash"

@pytest.mark.asyncio
async def test_admin_clear_profile_skin_idempotent(admin_backend_fixture, db_session, user_factory):
    """Clearing already-null skin returns 200"""
    user = await user_factory()
    pid = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "Idempotent", "default", None, None))
    result = await admin_backend_fixture.update_profile_skin(pid, None)
    assert result["ok"] is True

@pytest.mark.asyncio
async def test_admin_clear_profile_skin_404(admin_backend_fixture):
    """Non-existent profile → 404"""
    with pytest.raises(HTTPException) as exc:
        await admin_backend_fixture.update_profile_skin("non-existent", None)
    assert exc.value.status_code == 404


@pytest.mark.asyncio
async def test_admin_get_all_profiles_invalid_cursor(admin_backend_fixture):
    """非法游标字符串 → HTTPException 400（backend 编解码边界）"""
    with pytest.raises(HTTPException) as exc:
        await admin_backend_fixture.get_all_profiles(limit=10, cursor="not-a-valid-cursor!!")
    assert exc.value.status_code == 400


@pytest.mark.asyncio
async def test_admin_get_all_textures_invalid_cursor(admin_backend_fixture):
    """非法游标字符串 → HTTPException 400（backend 编解码边界）"""
    with pytest.raises(HTTPException) as exc:
        await admin_backend_fixture.get_all_textures(limit=10, cursor="garbage==")
    assert exc.value.status_code == 400


@pytest.mark.asyncio
async def test_admin_get_all_profiles_cursor_roundtrip(admin_backend_fixture, db_session, user_factory):
    """backend 返回 next_cursor(base64)，喂回可翻页：全量覆盖且无重叠"""
    user = await user_factory()
    pids = [generate_random_uuid() for _ in range(5)]
    for i, pid in enumerate(pids):
        await db_session.user.create_profile(PlayerProfile(pid, user.id, f"RoundTrip{i}", "default", None, None))

    seen = []
    cursor = None
    for _ in range(20):  # 安全上限
        page = await admin_backend_fixture.get_all_profiles(limit=2, cursor=cursor)
        seen.extend(item["id"] for item in page["items"])
        if not page["has_next"]:
            break
        cursor = page["next_cursor"]
        assert isinstance(cursor, str) and cursor

    # 至少包含本测试创建的 5 个角色，且无重复
    assert set(pids).issubset(set(seen))
    assert len(seen) == len(set(seen))


@pytest.mark.asyncio
async def test_admin_get_all_textures_cursor_roundtrip(admin_backend_fixture, db_session, user_factory):
    """backend 返回 next_cursor(base64)，喂回可翻页：全量覆盖且无重叠"""
    user = await user_factory()
    hashes = [chr(ord("a") + i) * 64 for i in range(5)]
    for i, h in enumerate(hashes):
        await db_session.texture.add_to_library(user.id, h, "skin", note=f"RT{i}", is_public=True)

    seen = []
    cursor = None
    for _ in range(20):  # 安全上限
        page = await admin_backend_fixture.get_all_textures(limit=2, cursor=cursor)
        seen.extend(item["hash"] for item in page["items"])
        if not page["has_next"]:
            break
        cursor = page["next_cursor"]
        assert isinstance(cursor, str) and cursor

    assert set(hashes).issubset(set(seen))
    assert len(seen) == len(set(seen))

@pytest.mark.asyncio
async def test_admin_clear_profile_cape(admin_backend_fixture, db_session, user_factory):
    """Clearing cape sets cape_hash to NULL without affecting skin_hash"""
    user = await user_factory()
    pid = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "CapeClear", "default", "skinhash", "capehash"))

    result = await admin_backend_fixture.update_profile_cape(pid, None)
    assert result["ok"] is True
    profile = await db_session.user.get_profile_by_id(pid)
    assert profile.skin_hash == "skinhash"
    assert profile.cape_hash is None
