import pytest
from fastapi import HTTPException
from utils.typing import PlayerProfile
from utils.uuid_utils import generate_random_uuid


@pytest.mark.asyncio
async def test_update_profile_invalid_name(admin_backend_fixture, db_session, user_factory):
    """验证 name 格式校验：特殊字符应返回 400"""
    user = await user_factory()
    pid = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "ValidName", "default", None, None))

    with pytest.raises(HTTPException) as exc:
        await admin_backend_fixture.update_profile(pid, name="invalid@name")
    assert exc.value.status_code == 400


@pytest.mark.asyncio
async def test_update_profile_duplicate_name(admin_backend_fixture, db_session, user_factory):
    """验证重复名处理：重复名称应返回 409"""
    user = await user_factory()
    pid1 = generate_random_uuid()
    pid2 = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid1, user.id, "Original", "default", None, None))
    await db_session.user.create_profile(PlayerProfile(pid2, user.id, "TakenName", "default", None, None))

    with pytest.raises(HTTPException) as exc:
        await admin_backend_fixture.update_profile(pid1, name="TakenName")
    assert exc.value.status_code == 409


@pytest.mark.asyncio
async def test_update_profile_success(admin_backend_fixture, db_session, user_factory):
    """验证成功更新 name"""
    user = await user_factory()
    pid = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "OldName", "default", None, None))

    result = await admin_backend_fixture.update_profile(pid, name="NewName")
    assert result["ok"] is True

    profile = await db_session.user.get_profile_by_id(pid)
    assert profile.name == "NewName"


@pytest.mark.asyncio
async def test_delete_profile_not_found(admin_backend_fixture):
    """验证删除不存在的角色返回 404"""
    with pytest.raises(HTTPException) as exc:
        await admin_backend_fixture.delete_profile("non-existent-id")
    assert exc.value.status_code == 404


@pytest.mark.asyncio
async def test_delete_profile_cascades_tokens(admin_backend_fixture, db_session, user_factory):
    """验证删除角色时级联删除关联 tokens"""
    import time
    from utils.typing import Token
    user = await user_factory()
    pid = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "ToDelete", "default", None, None))

    # Create a token referencing the profile
    token = Token("test-token-cascade", "test-client", user.id, pid, int(time.time() * 1000))
    await db_session.user.add_token(token)
    assert await db_session.user.get_token("test-token-cascade") is not None

    # Delete profile
    result = await admin_backend_fixture.delete_profile(pid)
    assert result["ok"] is True

    # Verify profile and token are gone
    assert await db_session.user.get_profile_by_id(pid) is None
    assert await db_session.user.get_token("test-token-cascade") is None


@pytest.mark.asyncio
async def test_delete_profile_success(admin_backend_fixture, db_session, user_factory):
    """验证成功删除角色"""
    user = await user_factory()
    pid = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "ToDelete", "default", None, None))

    result = await admin_backend_fixture.delete_profile(pid)
    assert result["ok"] is True
    assert await db_session.user.get_profile_by_id(pid) is None
