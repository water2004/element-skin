import pytest
from unittest.mock import AsyncMock, patch
from fastapi import HTTPException
from backends.site_backend import SiteBackend
from utils.password_utils import verify_password

@pytest.mark.asyncio
async def test_site_auth_flow(db_session, test_config):
    """测试完整的注册、登录及密码修改流程"""
    backend = SiteBackend(db_session, test_config)
    
    # 1. 注册 (首个用户应为管理员)
    email = "admin@example.com"
    password = "StrongPassword123!"
    username = "SuperAdmin"
    
    uid = await backend.register(email, password, username)
    assert uid is not None
    
    user_info = await backend.get_user_info(uid)
    assert user_info["is_admin"] is True
    assert user_info["display_name"] == username
    
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
    backend = SiteBackend(db_session, test_config)
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
    backend = SiteBackend(db_session, test_config)
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
    backend = SiteBackend(db_session, test_config)
    
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
