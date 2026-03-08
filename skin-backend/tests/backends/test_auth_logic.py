import pytest
from fastapi import HTTPException

@pytest.mark.asyncio
async def test_first_user_is_admin(site_backend_fixture, db_session):
    """
    测试注册逻辑：第一个注册的用户应自动获得管理员权限
    """
    # 1. 注册第一个用户
    uid1 = await site_backend_fixture.register(
        email="admin@test.com", 
        password="Pass123", 
        username="Admin"
    )
    
    user1 = await site_backend_fixture.get_user_info(uid1)
    assert user1["is_admin"] is True
    
    # 2. 注册第二个用户
    uid2 = await site_backend_fixture.register(
        email="user@test.com", 
        password="Pass123", 
        username="User"
    )
    
    user2 = await site_backend_fixture.get_user_info(uid2)
    assert user2["is_admin"] is False

@pytest.mark.asyncio
async def test_duplicate_email_registration(site_backend_fixture, user_factory):
    """
    测试注册逻辑：重复邮箱应报错
    """
    # 先创建一个用户
    existing_user = await user_factory(email="exist@test.com")
    
    # 尝试用相同邮箱注册
    with pytest.raises(HTTPException) as exc:
        await site_backend_fixture.register(
            email="exist@test.com",
            password="Pass123",
            username="NewUser"
        )
    assert exc.value.status_code == 400
    assert "Email already registered" in exc.value.detail

@pytest.mark.parametrize("password, is_valid", [
    ("12345", False),  # 太短
    ("simplepass", False),  # 只有小写
    ("StrongP@ss1", True)   # 复杂密码
])
@pytest.mark.asyncio
async def test_password_strength_config(site_backend_fixture, db_session, password, is_valid):
    """
    测试密码强度配置开关
    """
    # 开启强密码检查
    await db_session.setting.set("enable_strong_password_check", "true")
    
    if not is_valid:
        with pytest.raises(HTTPException) as exc:
            await site_backend_fixture.register(
                email=f"test_{password}@t.com",
                password=password,
                username=f"User_{password}"
            )
        assert exc.value.status_code == 400
    else:
        # 应该成功
        uid = await site_backend_fixture.register(
            email=f"test_{password}@t.com",
            password=password,
            username=f"User_{password}"
        )
        assert uid is not None
