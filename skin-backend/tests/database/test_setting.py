import pytest

@pytest.mark.asyncio
async def test_setting_crud_and_cache(db_session):
    """测试设置项的读写、默认值和缓存一致性"""
    
    # 1. 初始状态 (conftest 的 db_session 会调用 db.init())
    # Database.init() 会预填一些默认值，如 enable_skin_library='true'
    val = await db_session.setting.get("enable_skin_library")
    assert val == "true"
    
    # 2. Get with default
    val_default = await db_session.setting.get("non_existent_key", "default_val")
    assert val_default == "default_val"
    
    # 3. Set & Get (Update Cache)
    await db_session.setting.set("test_key", "test_value")
    assert await db_session.setting.get("test_key") == "test_value"
    
    # 4. Get all
    all_settings = await db_session.setting.get_all()
    assert all_settings["test_key"] == "test_value"
    assert "enable_skin_library" in all_settings

@pytest.mark.asyncio
async def test_setting_reinit_cache(db_session):
    """测试设置项模块初始化缓存"""
    await db_session.setting.set("reinit_key", "reinit_val")
    
    # 手动触发初始化
    await db_session.setting.init()
    
    assert await db_session.setting.get("reinit_key") == "reinit_val"
