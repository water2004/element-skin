import pytest
from fastapi import HTTPException
from backends.admin_backend import AdminBackend
from utils.typing import PlayerProfile
from utils.uuid_utils import generate_random_uuid

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


# ========== Phase 4: methods moved from admin router ==========


@pytest.mark.asyncio
async def test_admin_list_users_search_branch(admin_backend_fixture, db_session, user_factory):
    """list_users 走 search 分支：query 命中目标且字段映射正确"""
    target = await user_factory(email="finder@test.com", username="FindMe", is_admin=True)
    await user_factory(username="Unrelated")

    page = await admin_backend_fixture.list_users(None, 50, "FindMe")
    matches = [u for u in page["items"] if u.id == target.id]
    assert len(matches) == 1
    found = matches[0]
    assert found.email == "finder@test.com"
    assert found.display_name == "FindMe"
    assert found.is_admin is True
    # 非命中用户不应出现
    assert all(u.display_name == "FindMe" or "FindMe" in (u.email or "") for u in page["items"])


@pytest.mark.asyncio
async def test_admin_list_users_cursor_roundtrip(admin_backend_fixture, db_session, user_factory):
    """list_users 翻页：跟随 next_cursor 取下一页，全量覆盖且无重叠"""
    created = [(await user_factory(username=f"PageUser{i}")).id for i in range(5)]

    seen = []
    cursor = None
    for _ in range(50):  # 安全上限
        page = await admin_backend_fixture.list_users(cursor, 2, None)
        seen.extend(u.id for u in page["items"])
        cursor = page["next_cursor"]
        if not cursor:
            break
        assert isinstance(cursor, str) and cursor

    assert set(created).issubset(set(seen))
    assert len(seen) == len(set(seen))


@pytest.mark.asyncio
async def test_admin_list_users_invalid_cursor(admin_backend_fixture):
    with pytest.raises(HTTPException) as exc:
        await admin_backend_fixture.list_users("garbage!!", 15, None)
    assert exc.value.status_code == 400


@pytest.mark.asyncio
async def test_admin_get_user_profiles_mapping_and_paging(admin_backend_fixture, db_session, user_factory):
    """get_user_profiles：对象→dict 映射正确，且翻页无重叠"""
    user = await user_factory()
    pids = [generate_random_uuid() for _ in range(3)]
    await db_session.user.create_profile(
        PlayerProfile(pids[0], user.id, "ProfA", "slim", "skinA", "capeA")
    )
    for pid, name in zip(pids[1:], ("ProfB", "ProfC")):
        await db_session.user.create_profile(PlayerProfile(pid, user.id, name, "default", None, None))

    seen = {}
    cursor = None
    for _ in range(20):  # 安全上限
        page = await admin_backend_fixture.get_user_profiles(user.id, cursor, 2)
        for item in page["items"]:
            seen[item["id"]] = item
        if not page["has_next"]:
            break
        cursor = page["next_cursor"]
        assert isinstance(cursor, str) and cursor

    assert set(pids) == set(seen)
    assert len(seen) == 3  # 无重叠
    a = seen[pids[0]]
    assert a["name"] == "ProfA"
    assert a["model"] == "slim"
    assert a["skin_hash"] == "skinA"
    assert a["cape_hash"] == "capeA"


@pytest.mark.asyncio
async def test_admin_get_user_profiles_invalid_cursor(admin_backend_fixture, user_factory):
    user = await user_factory()
    with pytest.raises(HTTPException) as exc:
        await admin_backend_fixture.get_user_profiles(user.id, "not-valid!!", 20)
    assert exc.value.status_code == 400


@pytest.mark.asyncio
async def test_admin_list_invites_mapping_and_paging(admin_backend_fixture, db_session):
    """list_invites：InviteCode→dict 字段映射正确，翻页无重叠"""
    codes = []
    for i in range(3):
        c = await admin_backend_fixture.create_invite(code=f"INVITE_PAGE_{i}", total_uses=7, note=f"note{i}")
        codes.append(c)

    seen = {}
    cursor = None
    for _ in range(20):  # 安全上限
        page = await admin_backend_fixture.list_invites(cursor, 2)
        for item in page["items"]:
            seen[item["code"]] = item
        if not page["has_next"]:
            break
        cursor = page["next_cursor"]
        assert isinstance(cursor, str) and cursor

    assert set(codes).issubset(set(seen))
    sample = seen[codes[0]]
    assert sample["total_uses"] == 7
    assert sample["note"] == "note0"
    assert sample["used_count"] == 0
    assert "created_at" in sample and isinstance(sample["created_at"], int)


@pytest.mark.asyncio
async def test_admin_list_invites_invalid_cursor(admin_backend_fixture):
    with pytest.raises(HTTPException) as exc:
        await admin_backend_fixture.list_invites("bogus==", 15)
    assert exc.value.status_code == 400


@pytest.mark.asyncio
async def test_admin_delete_invite(admin_backend_fixture, db_session):
    code = await admin_backend_fixture.create_invite(code="TO_DELETE_CODE", total_uses=1)
    assert await db_session.user.get_invite(code) is not None
    result = await admin_backend_fixture.delete_invite(code)
    assert result["ok"] is True
    assert await db_session.user.get_invite(code) is None
