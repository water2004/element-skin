import pytest
import asyncio
import os
import shutil
import tempfile
from typing import AsyncGenerator, Callable
from httpx import AsyncClient, ASGITransport

# 导入应用实例和配置对象
# 注意：这会触发 routes_reference 模块级代码执行，包括 db = Database(...)
from routes_reference import app, db, config, site_backend, admin_backend, ygg_backend, crypto
from utils.jwt_utils import create_jwt_token
from utils.typing import User
from utils.password_utils import hash_password
from utils.uuid_utils import generate_random_uuid

# --- 基础环境配置 ---

@pytest.fixture(scope="session")
def event_loop():
    """创建一个 session 级别的 event loop，供整个测试会话使用"""
    loop = asyncio.new_event_loop()
    yield loop
    loop.close()

@pytest.fixture(scope="session")
def test_env_setup():
    """
    配置测试环境：
    1. 配置测试数据库 DSN
    2. 创建临时材质目录
    3. 覆盖全局配置对象
    """
    # 创建临时目录用于存储材质
    temp_dir = tempfile.mkdtemp()
    textures_dir = os.path.join(temp_dir, "test_textures")
    os.makedirs(textures_dir, exist_ok=True)

    # 测试数据库 DSN (建议在环境变量中配置，或者使用默认的测试库)
    test_dsn = os.getenv("TEST_DATABASE_DSN", "postgresql://elementskin:password@localhost:5432/elementskin_test")

    # 备份原始配置
    original_dsn = db.dsn
    
    # 覆盖全局 DB 对象的 DSN
    db.dsn = test_dsn
    
    # 覆盖 Config 对象中的配置
    config._data["database"]["dsn"] = test_dsn
    config._data["textures"]["directory"] = textures_dir
    # 覆盖 TextureModule 中的路径
    db.texture.textures_dir = textures_dir

    yield {
        "dsn": test_dsn,
        "textures_dir": textures_dir
    }

    # 清理材质目录
    shutil.rmtree(temp_dir)
    # 恢复 DSN
    db.dsn = original_dsn

@pytest.fixture(scope="session")
def test_config(test_env_setup):
    """提供覆盖后的配置对象"""
    return config

@pytest.fixture(scope="function")
async def db_session(test_env_setup):
    """
    数据库会话 Fixture：
    每个测试函数运行前重置数据库。
    """
    # 连接数据库
    await db.connect()
    
    # 重置公共模式 (快速重置所有表)
    async with db.get_conn() as conn:
        await conn.execute("DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
        # 重新授予权限 (如果是特定用户)
        await conn.execute("GRANT ALL ON SCHEMA public TO public;")
        await conn.execute(f"GRANT ALL ON SCHEMA public TO {os.getenv('POSTGRES_USER', 'elementskin')};")

    # 重新初始化表结构
    await db.init()
    
    yield db
    
    # 关闭连接
    await db.close()

@pytest.fixture(scope="function")
async def client(db_session) -> AsyncGenerator[AsyncClient, None]:
    """
    API 客户端 Fixture：
    集成测试使用，基于 httpx。
    """
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as ac:
        yield ac

# --- 数据工厂 (Factories) ---

@pytest.fixture
def user_factory(db_session):
    """
    用户工厂：快速创建测试用户
    """
    async def _create(
        email: str = None, 
        password: str = "Password123!", 
        username: str = None, 
        is_admin: bool = False
    ) -> User:
        uid = generate_random_uuid()
        if not email:
            email = f"user_{uid[:8]}@example.com"
        if not username:
            username = f"User_{uid[:8]}"
            
        hashed_pw = hash_password(password)
        # User 构造函数: id, email, password, is_admin, preferred_language, display_name, banned_until
        user = User(uid, email, hashed_pw, is_admin, "zh_CN", username)
        await db_session.user.create(user)
        return user
    return _create

@pytest.fixture
async def auth_headers(user_factory):
    """
    普通用户授权头 Fixture
    """
    user = await user_factory(is_admin=False)
    token = create_jwt_token(user.id, is_admin=False, expire_days=1)
    return {"Authorization": f"Bearer {token}", "X-User-ID": user.id}

@pytest.fixture
async def admin_headers(user_factory):
    """
    管理员授权头 Fixture
    """
    user = await user_factory(is_admin=True)
    token = create_jwt_token(user.id, is_admin=True, expire_days=1)
    return {"Authorization": f"Bearer {token}", "X-User-ID": user.id}

@pytest.fixture
def site_backend_fixture(db_session):
    return site_backend

@pytest.fixture
def admin_backend_fixture(db_session):
    return admin_backend

@pytest.fixture
def ygg_backend_fixture(db_session):
    return ygg_backend

@pytest.fixture
def crypto_fixture():
    return crypto
