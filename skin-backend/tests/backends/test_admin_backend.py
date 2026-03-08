import pytest
from fastapi import HTTPException
from backends.admin_backend import AdminBackend

@pytest.mark.asyncio
async def test_admin_settings_management(db_session, test_config):
    """测试管理员对设置的分组管理逻辑"""
    backend = AdminBackend(db_session, test_config)
    
    # 1. 保存站点设置
    site_settings = {
        "site_name": "New Test Site",
        "allow_register": False,
        "max_texture_size": 2048
    }
    await backend.save_settings_group("site", site_settings)
    
    # 验证保存结果
    fetched = await backend.get_site_settings()
    assert fetched["site_name"] == "New Test Site"
    assert fetched["allow_register"] is False
    assert fetched["max_texture_size"] == 2048
    
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
    
    # 1. 获取用户列表
    user_list = await backend.get_admin_users()
    assert len(user_list) == 2
    
    # 2. 封禁用户 (先对普通用户操作)
    import time
    banned_until = int((time.time() + 3600) * 1000)
    await backend.ban_user(user.id, banned_until, admin.id)
    assert await db_session.user.is_banned(user.id) is True

    # 3. 切换管理员状态
    await backend.toggle_user_admin(user.id, admin.id)
    assert (await db_session.user.get_by_id(user.id)).is_admin == 1

    # 禁止取消自己的管理员
    with pytest.raises(HTTPException) as exc:
        await backend.toggle_user_admin(admin.id, admin.id)
    assert exc.value.status_code == 403

    # 4. 降级并删除用户
    await backend.toggle_user_admin(user.id, admin.id) # 降级回普通用户
    assert (await db_session.user.get_by_id(user.id)).is_admin == 0

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
