import pytest
import json
import base64

@pytest.mark.asyncio
async def test_api_yggdrasil_metadata(client):
    """测试 Yggdrasil 服务发现元数据接口"""
    resp = await client.get("/")
    assert resp.status_code == 200
    data = resp.json()
    assert "meta" in data
    assert "signaturePublickey" in data
    assert "skinDomains" in data

@pytest.mark.asyncio
async def test_api_yggdrasil_authenticate(client, user_factory, db_session):
    """测试 Yggdrasil 认证接口 (Authlib-Injector 核心流程)"""
    password = "YggPassword123"
    user = await user_factory(password=password, username="YggPlayer")
    
    # 准备角色
    from utils.typing import PlayerProfile
    pid = "ygg_profile_id"
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "YggPlayer"))
    
    payload = {
        "username": user.email,
        "password": password,
        "requestUser": True
    }
    resp = await client.post("/authserver/authenticate", json=payload)
    
    assert resp.status_code == 200
    data = resp.json()
    assert "accessToken" in data
    assert data["selectedProfile"]["id"] == pid
    assert data["user"]["id"] == user.id

@pytest.mark.asyncio
async def test_api_yggdrasil_get_profile(client, user_factory, db_session):
    """测试获取角色信息及材质 JSON 接口"""
    user = await user_factory()
    pid = "profile_with_textures"
    # 添加一个带哈希的角色
    from utils.typing import PlayerProfile
    await db_session.user.create_profile(PlayerProfile(
        pid, user.id, "Skinny"
    ))
    await db_session.user.update_profile_skin(pid, "my_skin_hash")
    
    resp = await client.get(f"/sessionserver/session/minecraft/profile/{pid}")
    assert resp.status_code == 200
    data = resp.json()
    assert data["name"] == "Skinny"
    
    # 验证材质属性
    textures_prop = next(p for p in data["properties"] if p["name"] == "textures")
    textures_json = json.loads(base64.b64decode(textures_prop["value"]).decode("utf-8"))
    assert "SKIN" in textures_json["textures"]
    assert "my_skin_hash.png" in textures_json["textures"]["SKIN"]["url"]

@pytest.mark.asyncio
async def test_api_yggdrasil_join_has_joined(client, user_factory, db_session):
    """测试服务器加入与验证流程"""
    from utils.typing import PlayerProfile
    password = "Pass"
    user = await user_factory(password=password)
    pid = "p1"
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "Player1"))
    
    # 1. 认证获取 token
    auth_resp = await client.post("/authserver/authenticate", json={
        "username": user.email, "password": password
    })
    token = auth_resp.json()["accessToken"]
    
    # 2. Join
    server_id = "server_1"
    join_resp = await client.post("/sessionserver/session/minecraft/join", json={
        "accessToken": token,
        "selectedProfile": pid,
        "serverId": server_id
    })
    assert join_resp.status_code == 204
    
    # 3. Has Joined
    has_resp = await client.get("/sessionserver/session/minecraft/hasJoined", params={
        "username": "Player1",
        "serverId": server_id
    })
    assert has_resp.status_code == 200
    assert has_resp.json()["id"] == pid
