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
    1. 创建临时数据库文件
    2. 创建临时材质目录
    3. 覆盖全局配置对象
    """
    # 创建临时目录
    temp_dir = tempfile.mkdtemp()
    db_path = os.path.join(temp_dir, "test_yggdrasil.db")
    textures_dir = os.path.join(temp_dir, "test_textures")
    os.makedirs(textures_dir, exist_ok=True)

    # 备份原始配置 (虽然是在内存中修改对象，但是个好习惯)
    original_db_path = db.db_path
    original_texture_dir = site_backend.db.texture.textures_dir # TextureModule 初始化时读取了配置

    # 覆盖全局 DB 对象的路径
    # 注意：所有引用了这个 db 对象的地方都会受影响，包括 app 中的 routers
    db.db_path = db_path
    
    # 覆盖 Config 对象中的配置
    config._data["database"]["path"] = db_path
    config._data["textures"]["directory"] = textures_dir
    # 覆盖 TextureModule 中的路径 (因为它在初始化时已经读取了配置)
    db.texture.textures_dir = textures_dir

    yield {
        "db_path": db_path,
        "textures_dir": textures_dir
    }

    # 清理
    shutil.rmtree(temp_dir)
    # 恢复 (可选，如果是 session 级别其实无所谓)
    db.db_path = original_db_path

@pytest.fixture(scope="session")
def test_config(test_env_setup):
    """提供覆盖后的配置对象"""
    return config

@pytest.fixture(scope="function")
async def db_session(test_env_setup):
    """
    数据库会话 Fixture：
    每个测试函数运行前初始化表结构，运行后清空数据（或重建库）。
    为了速度，这里选择 truncate/delete 数据而不是重建文件，或者简单地依赖 session 隔离。
    但在 SQLite 中，删除文件重建可能更快更干净。
    这里采用：每次测试前 connect & init，测试后 close。
    由于 test_env_setup 是 session 级的，文件路径不变。
    我们可以在每个 function 级别删除数据库文件并重新初始化。
    """
    db_path = test_env_setup["db_path"]
    
    # 确保文件不存在（干净的状态）
    if os.path.exists(db_path):
        os.remove(db_path)
        
    # 连接并初始化表结构
    await db.connect()
    await db.init()
    
    yield db
    
    # 关闭连接
    await db.close()
    # 清理文件
    if os.path.exists(db_path):
        os.remove(db_path)

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
    使用方法: user = await user_factory(email="...", is_admin=True)
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
        user = User(uid, email, hashed_pw, 1 if is_admin else 0, "zh_CN", username)
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
    return {"Authorization": f"Bearer {token}", "X-User-ID": user.id} # 方便测试中获取 ID

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
    """提供 site_backend 实例"""
    return site_backend

@pytest.fixture
def admin_backend_fixture(db_session):
    """提供 admin_backend 实例"""
    return admin_backend

@pytest.fixture
def ygg_backend_fixture(db_session):
    """提供 ygg_backend 实例"""
    return ygg_backend

@pytest.fixture
def crypto_fixture():
    """提供 crypto 实例"""
    return crypto
