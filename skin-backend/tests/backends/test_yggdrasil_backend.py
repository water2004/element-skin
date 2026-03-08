import pytest
import time
from backends.yggdrasil_backend import YggdrasilBackend, ForbiddenOperationException, IllegalArgumentException
from utils.typing import PlayerProfile

@pytest.mark.asyncio
async def test_yggdrasil_authenticate_and_refresh(ygg_backend_fixture, user_factory, db_session):
    """测试 Yggdrasil 认证与刷新流程"""
    # 1. 准备用户
    password = "SecretPassword123"
    user = await user_factory(password=password, username="YggUser")
    
    # 创建一个角色
    from utils.uuid_utils import generate_random_uuid
    pid = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "YggPlayer"))
    
    # 2. 认证 (Authenticate)
    access_token, avail_profiles, selected_profile, user_id = await ygg_backend_fixture.authenticate(
        user.email, password, clientToken="test-client-token"
    )
    
    assert access_token is not None
    assert len(avail_profiles) == 1
    assert avail_profiles[0].id == pid
    assert selected_profile.id == pid
    assert user_id == user.id
    
    # 验证 Token 已存储
    token_data = await db_session.user.get_token(access_token)
    assert token_data.client_token == "test-client-token"
    
    # 3. 刷新 (Refresh)
    refresh_resp = await ygg_backend_fixture.refresh(
        access_token, "test-client-token", selectedProfile_uuid=None
    )
    
    new_access_token = refresh_resp["accessToken"]
    assert new_access_token != access_token
    assert refresh_resp["clientToken"] == "test-client-token"
    assert refresh_resp["selectedProfile"]["id"] == pid
    
    # 旧 Token 应该已失效
    assert (await db_session.user.get_token(access_token)) is None
    # 新 Token 应该有效
    assert (await db_session.user.get_token(new_access_token)) is not None

@pytest.mark.asyncio
async def test_yggdrasil_join_and_has_joined(ygg_backend_fixture, user_factory, db_session):
    """测试 joinServer 和 hasJoined 流程"""
    password = "SecretPassword123"
    user = await user_factory(password=password, username="JoinUser")
    pid = "test_profile_id"
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "JoinPlayer"))
    
    # 1. 认证获取 Token
    access_token, _, _, _ = await ygg_backend_fixture.authenticate(user.email, password, None)
    
    # 2. Join Server
    server_id = "test_server_123"
    await ygg_backend_fixture.join_server(access_token, pid, server_id, "127.0.0.1")
    
    # 3. Has Joined
    profile = await ygg_backend_fixture.has_joined("JoinPlayer", server_id)
    assert profile is not None
    assert profile.id == pid
    
    # 测试错误的服务器 ID
    assert (await ygg_backend_fixture.has_joined("JoinPlayer", "wrong_id")) is None
    
    # 测试错误的玩家名
    assert (await ygg_backend_fixture.has_joined("WrongPlayer", server_id)) is None

@pytest.mark.asyncio
async def test_yggdrasil_invalid_credentials(ygg_backend_fixture, user_factory):
    """测试错误凭据抛出异常"""
    await user_factory(email="test@test.com", password="CorrectPassword")
    
    with pytest.raises(ForbiddenOperationException, match="Invalid credentials"):
        await ygg_backend_fixture.authenticate("test@test.com", "WrongPassword", None)

@pytest.mark.asyncio
async def test_yggdrasil_texture_management(ygg_backend_fixture, user_factory, db_session):
    """测试通过 Yggdrasil 接口管理材质"""
    user = await user_factory(password="Pass123")
    pid = "tex_pid"
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "TexPlayer"))
    
    access_token, _, _, _ = await ygg_backend_fixture.authenticate(user.email, "Pass123", None)
    
    # 模拟材质文件
    from io import BytesIO
    from PIL import Image
    file = BytesIO()
    Image.new('RGBA', size=(64, 64), color=(0, 255, 0, 255)).save(file, 'png')
    file_bytes = file.getvalue()
    
    # 1. 上传材质 (PUT)
    await ygg_backend_fixture.upload_texture(access_token, pid, "skin", file_bytes, model="default")
    
    # 验证
    profile = await db_session.user.get_profile_by_id(pid)
    assert profile.skin_hash is not None
    
    # 2. 删除材质 (DELETE)
    await ygg_backend_fixture.delete_texture(access_token, pid, "skin")
    
    # 验证
    updated_p = await db_session.user.get_profile_by_id(pid)
    assert updated_p.skin_hash is None
