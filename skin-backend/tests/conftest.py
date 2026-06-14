import pytest
import asyncio
import os
import re
import shutil
import tempfile
import asyncpg
from typing import AsyncGenerator
from httpx import AsyncClient, ASGITransport

# 导入应用实例和配置对象
from routes_reference import app, db, config, site_backend, admin_backend, ygg_backend, crypto, texture_storage
from utils.jwt_utils import create_access_token
from utils.typing import User
from utils.password_utils import hash_password
from utils.uuid_utils import generate_random_uuid

# --- 基础环境配置 ---

@pytest.fixture(scope="session")
def event_loop():
    """创建一个 session 级别的 event loop"""
    if os.name == 'nt':
        loop = asyncio.SelectorEventLoop()
    else:
        loop = asyncio.new_event_loop()
    yield loop
    loop.close()

async def ensure_test_database():
    """确保测试数据库存在"""
    admin_dsn = os.getenv("ADMIN_DATABASE_DSN", "postgresql://postgres:12345678@localhost:5432/postgres?sslmode=disable")
    test_db_name = "elementskin_test"
    
    conn = await asyncpg.connect(admin_dsn)
    try:
        exists = await conn.fetchval("SELECT 1 FROM pg_database WHERE datname = $1", test_db_name)
        if not exists:
            await conn.execute(f'CREATE DATABASE "{test_db_name}"')
    finally:
        await conn.close()

def _quote_ident(name: str) -> str:
    if not re.fullmatch(r"[A-Za-z0-9_]+", name):
        raise ValueError(f"unsafe database name: {name!r}")
    return '"' + name.replace('"', '""') + '"'

async def drop_test_database(test_db_name: str = "elementskin_test"):
    """关闭残留连接并删除测试数据库。"""
    admin_dsn = os.getenv("ADMIN_DATABASE_DSN", "postgresql://postgres:12345678@localhost:5432/postgres?sslmode=disable")

    conn = await asyncpg.connect(admin_dsn)
    try:
        await conn.execute(
            """
            SELECT pg_terminate_backend(pid)
            FROM pg_stat_activity
            WHERE datname = $1 AND pid <> pg_backend_pid()
            """,
            test_db_name,
        )
        await conn.execute(f"DROP DATABASE IF EXISTS {_quote_ident(test_db_name)}")
    finally:
        await conn.close()

@pytest.fixture(scope="session")
def test_env_setup_data():
    """Session 级别的静态数据准备"""
    temp_dir = tempfile.mkdtemp()
    textures_dir = os.path.join(temp_dir, "test_textures")
    os.makedirs(textures_dir, exist_ok=True)
    
    test_db_name = "elementskin_test"
    test_dsn = f"postgresql://postgres:12345678@localhost:5432/{test_db_name}?sslmode=disable"
    
    yield {
        "dsn": test_dsn,
        "textures_dir": textures_dir,
        "temp_dir": temp_dir
    }
    
    shutil.rmtree(temp_dir)

@pytest.fixture(scope="session")
def test_config(test_env_setup_data):
    """供 backends 测试使用的配置对象"""
    return config

@pytest.fixture(scope="function")
async def db_session(test_env_setup_data):
    """每个测试函数运行前重置数据库"""
    await ensure_test_database()
    
    test_dsn = test_env_setup_data["dsn"]
    original_dsn = db.dsn
    original_textures_dir = texture_storage.textures_dir
    db.dsn = test_dsn
    config._data["database"]["dsn"] = test_dsn
    texture_storage.textures_dir = test_env_setup_data["textures_dir"]

    await db.connect()
    async with db.get_conn() as conn:
        await conn.execute("DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
        await conn.execute("GRANT ALL ON SCHEMA public TO public;")
        await conn.execute("GRANT ALL ON SCHEMA public TO postgres;")

    await db.init()
    try:
        yield db
    finally:
        await db.close()
        db.dsn = original_dsn
        config._data["database"]["dsn"] = original_dsn
        texture_storage.textures_dir = original_textures_dir
        await drop_test_database()

@pytest.fixture(scope="function")
async def client(db_session) -> AsyncGenerator[AsyncClient, None]:
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as ac:
        yield ac

# --- 数据工厂 ---

@pytest.fixture
def user_factory(db_session):
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
        # PostgreSQL boolean 需要 True/False
        user = User(uid, email, hashed_pw, is_admin, "zh_CN", username)
        await db_session.user.create(user)
        return user
    return _create

@pytest.fixture
async def auth_headers(user_factory):
    user = await user_factory(is_admin=False)
    token = create_access_token(user.id, is_admin=False)
    return {"cookies": {"access_token": token}, "X-User-ID": user.id}

@pytest.fixture
async def admin_headers(user_factory):
    user = await user_factory(is_admin=True)
    token = create_access_token(user.id, is_admin=True)
    return {"cookies": {"access_token": token}, "X-User-ID": user.id}

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

@pytest.fixture
def texture_storage_fixture(db_session):
    return texture_storage
