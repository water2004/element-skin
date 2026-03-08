import pytest
import aiohttp
import asyncio
from unittest.mock import AsyncMock, patch, MagicMock
from backends.fallback_backend import FallbackBackend
from fastapi.responses import Response

@pytest.mark.asyncio
async def test_fallback_has_joined(db_session):
    """测试 Fallback 节点的 hasJoined 转发逻辑"""
    backend = FallbackBackend(db_session)
    
    # 1. 准备一个模拟的外部节点
    await db_session.fallback.save_endpoints([{
        "id": None,
        "priority": 1,
        "session_url": "https://external-auth.com",
        "account_url": "https://external-account.com",
        "services_url": "https://external-services.com",
        "cache_ttl": 60,
        "enable_hasjoined": True,
        "enable_profile": True,
        "note": "MockNode"
    }])
    
    # 2. Mock aiohttp.ClientSession.get
    mock_response_content = b'{"id":"mock-uuid","name":"MockPlayer"}'
    
    # 创建一个模拟 Response 对象
    mock_resp = AsyncMock()
    mock_resp.status = 200
    mock_resp.read.return_value = mock_response_content
    mock_resp.__aenter__.return_value = mock_resp
    
    with patch('aiohttp.ClientSession.get', return_value=mock_resp) as mock_get:
        # 执行调用
        res = await backend.has_joined("MockPlayer", "mock_server_id")
        
        # 验证返回
        assert isinstance(res, Response)
        assert res.status_code == 200
        assert res.body == mock_response_content
        
        # 验证 URL 是否正确
        mock_get.assert_called_once()
        args, kwargs = mock_get.call_args
        assert "https://external-auth.com/session/minecraft/hasJoined" in args[0]
        assert kwargs["params"]["username"] == "MockPlayer"

@pytest.mark.asyncio
async def test_fallback_strategy_parallel(db_session):
    """测试 Fallback 节点的并行请求策略"""
    backend = FallbackBackend(db_session)
    await db_session.setting.set("fallback_strategy", "parallel")
    
    # 准备两个节点，显式启用功能
    await db_session.fallback.save_endpoints([
        {
            "id": None, "priority": 1, 
            "session_url": "https://node1.com", "account_url": "a", "services_url": "s", 
            "cache_ttl": 60, "note": "N1",
            "enable_profile": True, "enable_hasjoined": True
        },
        {
            "id": None, "priority": 2, 
            "session_url": "https://node2.com", "account_url": "a", "services_url": "s", 
            "cache_ttl": 60, "note": "N2",
            "enable_profile": True, "enable_hasjoined": True
        }
    ])
    
    def mock_get_impl(url, **kwargs):
        m = AsyncMock()
        if "node1.com" in url:
            # 模拟延迟和 404
            async def delayed_404():
                await asyncio.sleep(0.1)
                return 404
            m.status = 404 # 实际上这里需要更复杂的模拟来支持 await m.status 或类似的，但 FallbackBackend 是同步读取 .status
            # 修正：FallbackBackend 中是 async with session.get(...) as resp: 然后 if resp.status == 200:
            # 所以 m.status 必须在 __aenter__ 之后立即可用，或者本身就是一个普通属性
            
            # 为了模拟延迟，我们让 __aenter__ 变成异步延迟
            original_aenter = m.__aenter__
            async def slow_aenter(*args, **kwargs):
                await asyncio.sleep(0.1)
                return await original_aenter(*args, **kwargs)
            m.__aenter__ = slow_aenter
            m.status = 404
        else:
            m.status = 200
            m.read.return_value = b'{"fast":true}'
            m.__aenter__.return_value = m
        return m

    with patch('aiohttp.ClientSession.get', side_effect=mock_get_impl):
        res = await backend.get_profile("some-uuid")
        assert res is not None
        assert res.status_code == 200
        assert res.body == b'{"fast":true}'

@pytest.mark.asyncio
async def test_fallback_whitelist_filter(db_session):
    """测试 Fallback 节点的白名单拦截逻辑"""
    backend = FallbackBackend(db_session)
    
    # 准备一个开启白名单的节点
    await db_session.fallback.save_endpoints([{
        "id": None,
        "priority": 1,
        "session_url": "https://mock.com",
        "account_url": "a", "services_url": "s", "cache_ttl": 60,
        "enable_whitelist": True,
        "note": "WhitelistedNode",
        "enable_hasjoined": True
    }])
    
    # 获取实际分配的 ID
    endpoints = await db_session.fallback.list_endpoints()
    endpoint_id = endpoints[0]["id"]
    
    # 用户不在白名单中
    res = await backend.has_joined("Stranger", "sid")
    assert res is None # 应该直接返回 None
    
    # 用户在白名单中
    await db_session.fallback.add_whitelist_user("Stranger", endpoint_id)
    
    mock_resp = AsyncMock()
    mock_resp.status = 200
    mock_resp.read.return_value = b'{"ok":true}'
    mock_resp.__aenter__.return_value = mock_resp
    
    with patch('aiohttp.ClientSession.get', return_value=mock_resp) as mock_get:
        res = await backend.has_joined("Stranger", "sid")
        assert res is not None
        assert res.status_code == 200
        mock_get.assert_called_once()
