import pytest

@pytest.mark.asyncio
async def test_fallback_endpoints(db_session):
    """测试外部节点 CRUD 及其缓存逻辑"""
    
    # 1. 默认状态 (Database.init 会添加一个默认 Mojang 节点)
    endpoints = await db_session.fallback.list_endpoints()
    assert len(endpoints) == 1
    assert endpoints[0]["note"] == "Mojang Official"
    
    # 2. Save new endpoints (replace existing)
    new_eps = [
        {
            "id": None,
            "priority": 1,
            "session_url": "s1", "account_url": "a1", "services_url": "v1",
            "cache_ttl": 60, "skin_domains": "d1,d2",
            "enable_profile": True, "enable_hasjoined": True, "enable_whitelist": False,
            "note": "CustomEP"
        }
    ]
    await db_session.fallback.save_endpoints(new_eps)
    
    # 刷新后的 list
    updated_eps = await db_session.fallback.list_endpoints()
    assert len(updated_eps) == 1
    assert updated_eps[0]["note"] == "CustomEP"
    
    # 3. Cache Check: Skin Domains
    domains = await db_session.fallback.collect_skin_domains()
    assert "d1" in domains
    assert "d2" in domains
    
    # 4. Primary Endpoint
    primary = await db_session.fallback.get_primary_endpoint()
    assert primary["note"] == "CustomEP"

@pytest.mark.asyncio
async def test_fallback_whitelist(db_session):
    """测试外部节点的白名单及其高效缓存"""
    endpoint_id = 1
    username = "WhitelistedPlayer"
    
    # 1. Add to whitelist
    await db_session.fallback.add_whitelist_user(username, endpoint_id)
    
    # 2. Check Cache
    assert await db_session.fallback.is_user_in_whitelist(username, endpoint_id) is True
    assert await db_session.fallback.is_user_in_whitelist("NonExistent", endpoint_id) is False
    
    # 3. List
    list_users = await db_session.fallback.list_whitelist_users(endpoint_id)
    assert len(list_users) == 1
    assert list_users[0]["username"] == username
    
    # 4. Remove
    await db_session.fallback.remove_whitelist_user(username, endpoint_id)
    assert await db_session.fallback.is_user_in_whitelist(username, endpoint_id) is False
