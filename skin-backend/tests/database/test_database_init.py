import pytest
import os
from database_module import Database

@pytest.mark.asyncio
async def test_database_init_scripts(test_env_setup):
    """测试数据库初始化脚本：建表、默认设置预填"""
    
    # 获取测试 DSN
    test_dsn = test_env_setup["dsn"]
    
    # 准备一个干净的新库（通过重置 schema）
    db = Database(dsn=test_dsn)
    await db.connect()
    
    async with db.get_conn() as conn:
        await conn.execute("DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
    
    # 2. 执行 Init (执行核心脚本、预填默认设置)
    await db.init()
    
    # 3. 验证表是否创建成功 (PostgreSQL 使用 information_schema)
    async with db.get_conn() as conn:
        rows = await conn.fetch(
            "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'"
        )
        tables = [r['table_name'] for r in rows]
        assert "users" in tables
        assert "settings" in tables
        assert "fallback_endpoints" in tables
        
        # 验证默认设置
        val = await conn.fetchval("SELECT value FROM settings WHERE key='enable_skin_library'")
        assert val == "true"
        
        # 验证默认设置中的其他项
        smtp_host = await conn.fetchval("SELECT value FROM settings WHERE key='smtp_host'")
        assert smtp_host == "smtp.example.com"
    
    await db.close()
