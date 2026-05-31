import pytest
from unittest.mock import AsyncMock, patch
from fastapi import HTTPException
from backends.site_backend import SiteBackend
from routes_reference import texture_storage
from utils.password_utils import verify_password
from utils.typing import PlayerProfile
from utils.uuid_utils import get_offline_uuid

@pytest.mark.asyncio
async def test_site_auth_flow(db_session, test_config):
    """测试完整的注册、登录及密码修改流程"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    
    # 1. 注册 (首个用户应为管理员)
    email = "admin@example.com"
    password = "StrongPassword123!"
    username = "SuperAdmin"
    
    uid = await backend.register(email, password, username)
    assert uid is not None
    
    user_info = await backend.get_user_info(uid)
    assert user_info["is_admin"] is True
    assert user_info["display_name"] == username
    assert "profile_count" in user_info
    assert "profiles" not in user_info
    
    # 2. 登录
    login_res = await backend.login(email, password)
    assert login_res["user_id"] == uid
    assert "token" in login_res
    
    # 3. 修改密码
    new_password = "NewStrongPassword456!"
    await backend.change_password(uid, password, new_password)
    
    # 验证新密码是否生效
    user_row = await db_session.user.get_by_id(uid)
    assert verify_password(new_password, user_row.password) is True
    
    # 4. 验证登录失败 (使用旧密码)
    with pytest.raises(HTTPException) as exc:
        await backend.login(email, password)
    assert exc.value.status_code == 401

@pytest.mark.asyncio
async def test_verification_code_flow(db_session, test_config):
    """测试邮箱验证码发送与校验流程"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    email = "verify@test.com"
    
    # 启用邮件验证
    await db_session.setting.set("email_verify_enabled", "true")
    
    # 使用 Mock 模拟邮件发送
    with patch.object(backend.email_sender, 'send_verification_code', new_callable=AsyncMock) as mock_send:
        mock_send.return_value = True
        
        # 发送注册验证码
        res = await backend.send_verification_code(email, "register")
        assert res["ok"] is True
        mock_send.assert_called_once()
        
        # 获取存储的验证码并验证
        record = await db_session.verification.get_code(email, "register")
        code = record[0]
        
        assert await backend.verify_code(email, code, "register") is True
        assert await backend.verify_code(email, "WRONG", "register") is False

@pytest.mark.asyncio
async def test_profile_and_texture_application(db_session, test_config, user_factory):
    """测试角色创建及材质应用逻辑"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()
    
    # 1. 创建角色
    profile_data = await backend.create_profile(user.id, "MyPlayer", "default")
    pid = profile_data["id"]
    assert profile_data["name"] == "MyPlayer"
    
    # 2. 准备材质 (模拟已在库中)
    tex_hash = "some_skin_hash"
    await db_session.texture.add_to_library(user.id, tex_hash, "skin", is_public=False)
    
    # 3. 应用材质到角色
    await backend.apply_texture_to_profile(user.id, pid, tex_hash, "skin")
    
    # 验证
    updated_p = await db_session.user.get_profile_by_id(pid)
    assert updated_p.skin_hash == tex_hash
    
    # 4. 清除材质
    await backend.clear_profile_texture(user.id, pid, "skin")
    cleared_p = await db_session.user.get_profile_by_id(pid)
    assert cleared_p.skin_hash is None

@pytest.mark.asyncio
async def test_registration_restrictions(db_session, test_config, user_factory):
    """测试注册限制逻辑：邀请码、注册开关、用户名重复"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    
    # 1. 禁用注册
    await db_session.setting.set("allow_register", "false")
    with pytest.raises(HTTPException) as exc:
        await backend.register("t@t.com", "p", "u")
    assert exc.value.status_code == 403
    
    await db_session.setting.set("allow_register", "true")
    
    # 2. 强制邀请码
    await db_session.setting.set("require_invite", "true")
    with pytest.raises(HTTPException) as exc:
        await backend.register("t@t.com", "p", "u")
    assert "invite code required" in exc.value.detail
    
    # 使用无效邀请码
    with pytest.raises(HTTPException) as exc:
        await backend.register("t@t.com", "p", "u", invite_code="INVALID")
    assert "invalid invite code" in exc.value.detail
    
    # 使用有效邀请码
    from utils.typing import InviteCode
    import time
    await db_session.user.create_invite(InviteCode("VALID_CODE", int(time.time()*1000), total_uses=1))
    uid = await backend.register("t@t.com", "Pass123!", "UniqueUser", invite_code="VALID_CODE")
    assert uid is not None
    
    # 3. 用户名占用
    with pytest.raises(HTTPException) as exc:
        await backend.register("t2@t.com", "p", "UniqueUser")
    assert "Username already exists" in exc.value.detail


@pytest.mark.asyncio
async def test_create_profile_uses_offline_uuid_when_enabled(db_session, test_config, user_factory):
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()
    await db_session.setting.set("profile_uuid_mode", "offline")

    created = await backend.create_profile(user.id, "OfflinePlayerA", "default")
    assert created["id"] == get_offline_uuid("OfflinePlayerA")


@pytest.mark.asyncio
async def test_create_profile_rejects_uuid_conflict(db_session, test_config, user_factory):
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()

    conflict_id = "abcdabcdabcdabcdabcdabcdabcdabcd"
    await db_session.user.create_profile(PlayerProfile(conflict_id, user.id, "TakenRole", "default"))

    with patch("backends.site_backend.generate_random_uuid", return_value=conflict_id):
        with pytest.raises(HTTPException) as exc:
            await backend.create_profile(user.id, "BrandNewRole", "default")

    assert exc.value.status_code == 400
    assert exc.value.detail == "角色 UUID 冲突，无法新建角色"


# ========== Phase 4: orchestration methods moved from router ==========


@pytest.mark.asyncio
async def test_get_public_skin_library_aggregates_uploader_name(db_session, test_config, user_factory):
    """皮肤库聚合：每条 item 带正确的 uploader_name，且翻页跟随游标无重叠"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    uploader = await user_factory(username="LibOwner")
    hashes = [chr(ord("a") + i) * 64 for i in range(3)]
    for i, h in enumerate(hashes):
        await db_session.texture.add_to_library(uploader.id, h, "skin", note=f"L{i}", is_public=True)

    seen = []
    cursor = None
    for _ in range(10):
        page = await backend.get_public_skin_library(cursor, 2, None)
        for item in page["items"]:
            assert item["uploader_name"] == "LibOwner"
        seen.extend(item["hash"] for item in page["items"])
        if not page["has_next"]:
            break
        cursor = page["next_cursor"]
        assert isinstance(cursor, str) and cursor

    assert set(hashes).issubset(set(seen))
    assert len(seen) == len(set(seen))


@pytest.mark.asyncio
async def test_get_public_skin_library_disabled(db_session, test_config):
    backend = SiteBackend(db_session, test_config, texture_storage)
    await db_session.setting.set("enable_skin_library", "false")
    with pytest.raises(HTTPException) as exc:
        await backend.get_public_skin_library(None, 20, None)
    assert exc.value.status_code == 403


@pytest.mark.asyncio
async def test_get_public_skin_library_invalid_cursor(db_session, test_config):
    backend = SiteBackend(db_session, test_config, texture_storage)
    with pytest.raises(HTTPException) as exc:
        await backend.get_public_skin_library("garbage!!", 20, None)
    assert exc.value.status_code == 400


@pytest.mark.asyncio
async def test_update_my_texture_field_branches(db_session, test_config, user_factory):
    """note/model/is_public 三个分支独立生效，返回最新 info"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()
    h = "f" * 64
    await db_session.texture.add_to_library(user.id, h, "skin", note="orig", is_public=True, model="default")

    res = await backend.update_my_texture(user.id, h, "skin", {"note": "renamed"})
    assert res["ok"] is True
    assert res["note"] == "renamed"

    res = await backend.update_my_texture(user.id, h, "skin", {"model": "slim"})
    assert res["model"] == "slim"

    res = await backend.update_my_texture(user.id, h, "skin", {"is_public": False})
    assert res["is_public"] == 0


@pytest.mark.asyncio
async def test_upload_and_apply_texture_three_steps(db_session, test_config, user_factory):
    """上传→应用→更新模型 三步串联，皮肤 hash 与 slim 模型落到角色上"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()
    profile = await backend.create_profile(user.id, "ApplyTarget", "default")
    pid = profile["id"]

    with patch.object(texture_storage, "process_and_save", return_value="applied_hash"):
        result = await backend.upload_and_apply_texture(
            user.id, pid, b"fake-png-bytes", "skin", model="slim", is_public=False
        )
    assert result["ok"] is True

    updated = await db_session.user.get_profile_by_id(pid)
    assert updated.skin_hash == "applied_hash"
    assert updated.texture_model == "slim"


@pytest.mark.asyncio
async def test_add_texture_to_wardrobe_missing(db_session, test_config, user_factory):
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()
    with pytest.raises(HTTPException) as exc:
        await backend.add_texture_to_wardrobe(user.id, "nonexistent_hash")
    assert exc.value.status_code == 404


@pytest.mark.asyncio
async def test_get_my_texture_detail_missing(db_session, test_config, user_factory):
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()
    with pytest.raises(HTTPException) as exc:
        await backend.get_my_texture_detail(user.id, "nope", "skin")
    assert exc.value.status_code == 404
