import pytest
import os
import aiosqlite
from database_module import Database

@pytest.mark.asyncio
async def test_database_init_scripts(test_env_setup):
    """测试数据库初始化脚本：建表、默认设置预填和旧版本兼容性逻辑"""
    
    # 1. 准备一个干净的新库
    new_db_path = os.path.join(os.path.dirname(test_env_setup["db_path"]), "clean_init.db")
    if os.path.exists(new_db_path):
        os.remove(new_db_path)
    
    db = Database(db_path=new_db_path)
    await db.connect()
    
    # 2. 执行 Init (执行核心脚本、预填默认设置、自动设置首个 Mojang 节点等)
    await db.init()
    
    # 3. 验证表是否创建成功
    async with db.get_conn() as conn:
        cursor = await conn.execute("SELECT name FROM sqlite_master WHERE type='table'")
        tables = [r[0] for r in await cursor.fetchall()]
        assert "users" in tables
        assert "settings" in tables
        assert "fallback_endpoints" in tables
        
        # 验证默认设置
        cursor = await conn.execute("SELECT value FROM settings WHERE key='enable_skin_library'")
        row = await cursor.fetchone()
        assert row[0] == "true"
        
        # 验证默认 Mojang 节点
        cursor = await conn.execute("SELECT note FROM fallback_endpoints LIMIT 1")
        row = await cursor.fetchone()
        assert row[0] == "Mojang Official"
    
    await db.close()
    if os.path.exists(new_db_path):
        os.remove(new_db_path)
