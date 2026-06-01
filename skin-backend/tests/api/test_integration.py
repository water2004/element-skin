import pytest

@pytest.mark.asyncio
async def test_get_public_settings(client):
    """测试无需认证的公开接口"""
    response = await client.get("/public/settings")
    assert response.status_code == 200
    data = response.json()
    assert "site_name" in data
    assert "allow_register" in data

@pytest.mark.asyncio
async def test_admin_access_control(client, auth_headers, admin_headers):
    """
    测试权限控制：
    1. 普通用户访问管理接口 -> 403
    2. 管理员访问管理接口 -> 200
    """
    endpoint = "/admin/users"
    
    # 普通用户尝试访问
    resp_user = await client.get(endpoint, cookies=auth_headers["cookies"])
    assert resp_user.status_code == 403
    
    # 管理员尝试访问
    resp_admin = await client.get(endpoint, cookies=admin_headers["cookies"])
    assert resp_admin.status_code == 200
    data = resp_admin.json()
    assert "items" in data
    assert "has_next" in data
    assert isinstance(data["items"], list)

@pytest.mark.asyncio
async def test_login_flow(client, user_factory, db_session):
    """测试完整的登录流程"""
    # 准备数据
    email = "login_test@example.com"
    password = "MySecretPassword"
    # 工厂创建的用户密码会被哈希，所以我们需要知道原始密码用于登录
    # 但 user_factory 内部做了哈希，我们无法直接获取明文。
    # 解决办法：我们手动创建一个已知密码的用户，或者让 factory 支持传入明文并返回明文（虽然它返回的是 User 对象）
    # 这里我们在 factory 调用时指定 password，所以我们知道密码是 "MySecretPassword"
    await user_factory(email=email, password=password)
    
    # 发起登录请求
    payload = {
        "email": email,
        "password": password
    }
    response = await client.post("/site-login", json=payload)
    
    assert response.status_code == 200
    data = response.json()
    assert "user_id" in data

    # token 现在在 HttpOnly cookie 中，不再出现在 body 里
    assert "set-cookie" in response.headers
    cookie_header = response.headers["set-cookie"]
    assert "access_token=" in cookie_header
    assert "refresh_token=" in cookie_header
    assert "HttpOnly" in cookie_header

    # 从 cookie 中提取 access token 用于 /me 验证
    token = cookie_header.split("access_token=")[1].split(";")[0]
    me_resp = await client.get("/me", cookies={"access_token": token})
    assert me_resp.status_code == 200
    assert me_resp.json()["email"] == email
