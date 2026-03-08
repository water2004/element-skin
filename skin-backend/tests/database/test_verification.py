import pytest
import asyncio
import time

@pytest.mark.asyncio
async def test_verification_code_logic(db_session):
    """测试验证码的创建、查询、过期和删除"""
    email = "test@verify.com"
    code = "ABCDEFGH"
    v_type = "register"
    ttl = 1 # 1s 过期
    
    # 1. Create
    await db_session.verification.create_code(email, code, v_type, ttl)
    
    # 2. Get & Verify
    record = await db_session.verification.get_code(email, v_type)
    assert record is not None
    assert record[0] == code
    
    # 3. Check Expiry
    await asyncio.sleep(1.1)
    record_expired = await db_session.verification.get_code(email, v_type)
    # record[1] 是 expires_at (ms)
    assert int(time.time() * 1000) > record_expired[1]
    
    # 4. Delete
    await db_session.verification.delete_code(email, v_type)
    assert (await db_session.verification.get_code(email, v_type)) is None
