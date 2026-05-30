import pytest
import time
import json
import base64
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


# ========== Phase 4: presentation logic moved from yggdrasil router ==========


def test_build_profile_json_skin_slim_and_cape(ygg_backend_fixture):
    """slim 皮肤带 model metadata，cape 出现在 textures，且 SKIN url 含 hash"""
    profile = PlayerProfile("uuid1", "owner", "SlimGuy", "slim", "skinhash", "capehash")
    data = ygg_backend_fixture.build_profile_json(profile, sign=False)

    assert data["id"] == "uuid1"
    assert data["name"] == "SlimGuy"
    textures_prop = next(p for p in data["properties"] if p["name"] == "textures")
    # 未签名时不应有 signature 字段
    assert "signature" not in textures_prop

    decoded = json.loads(base64.b64decode(textures_prop["value"]).decode("utf-8"))
    assert "skinhash.png" in decoded["textures"]["SKIN"]["url"]
    assert decoded["textures"]["SKIN"]["metadata"] == {"model": "slim"}
    assert "capehash.png" in decoded["textures"]["CAPE"]["url"]
    # uploadableTextures 扩展属性保留
    assert any(p["name"] == "uploadableTextures" for p in data["properties"])


def test_build_profile_json_default_model_no_metadata(ygg_backend_fixture):
    """default 模型不带 metadata，无 cape 时不出现 CAPE"""
    profile = PlayerProfile("uuid2", "owner", "Plain", "default", "sh", None)
    data = ygg_backend_fixture.build_profile_json(profile, sign=False)
    textures_prop = next(p for p in data["properties"] if p["name"] == "textures")
    decoded = json.loads(base64.b64decode(textures_prop["value"]).decode("utf-8"))
    assert "metadata" not in decoded["textures"]["SKIN"]
    assert "CAPE" not in decoded["textures"]


def test_build_profile_json_sign_adds_signature(ygg_backend_fixture):
    """sign=True 时 textures 属性附带 signature"""
    profile = PlayerProfile("uuid3", "owner", "Signed", "default", "sh", None)
    data = ygg_backend_fixture.build_profile_json(profile, sign=True)
    textures_prop = next(p for p in data["properties"] if p["name"] == "textures")
    assert textures_prop.get("signature")


@pytest.mark.asyncio
async def test_build_authenticate_response_request_user(ygg_backend_fixture, user_factory, db_session):
    """build_authenticate_response：requestUser=True 带 user.preferredLanguage 属性"""
    password = "RespPass123"
    user = await user_factory(password=password, username="RespUser")
    await db_session.user.create_profile(PlayerProfile("resp_pid", user.id, "RespUser"))

    resp = await ygg_backend_fixture.build_authenticate_response(
        user.email, password, "client-tok", request_user=True
    )
    assert resp["clientToken"] == "client-tok"
    assert resp["selectedProfile"]["id"] == "resp_pid"
    assert resp["user"]["id"] == user.id
    lang_prop = next(p for p in resp["user"]["properties"] if p["name"] == "preferredLanguage")
    assert lang_prop["value"] == user.preferred_language


@pytest.mark.asyncio
async def test_build_authenticate_response_no_request_user(ygg_backend_fixture, user_factory, db_session):
    """requestUser 缺省时不带 user 对象；clientToken 缺省时回退为 accessToken"""
    password = "RespPass123"
    user = await user_factory(password=password, username="NoUserResp")
    await db_session.user.create_profile(PlayerProfile("nu_pid", user.id, "NoUserResp"))

    resp = await ygg_backend_fixture.build_authenticate_response(
        user.email, password, None, request_user=False
    )
    assert "user" not in resp
    assert resp["clientToken"] == resp["accessToken"]


@pytest.mark.asyncio
async def test_build_metadata_shape(ygg_backend_fixture, db_session):
    """build_metadata：含 meta/skinDomains/signaturePublickey，skinDomains 为 list"""
    await db_session.setting.set("site_name", "My Ygg Station")
    meta = await ygg_backend_fixture.build_metadata("http://fallback.example")
    assert meta["meta"]["serverName"] == "My Ygg Station"
    assert isinstance(meta["skinDomains"], list)
    assert meta["signaturePublickey"]
