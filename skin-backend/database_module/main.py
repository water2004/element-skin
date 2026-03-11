from .core import BaseDB
from .modules.user import UserModule
from .modules.setting import SettingModule
from .modules.texture import TextureModule
from .modules.verification import VerificationModule
from .modules.fallback import FallbackModule
from config_loader import config
import asyncio

# PostgreSQL 兼容语法
INIT_SQL = """
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    preferred_language TEXT DEFAULT 'zh_CN',
    display_name TEXT DEFAULT '',
    is_admin BOOLEAN DEFAULT FALSE,
    banned_until BIGINT DEFAULT NULL
);

CREATE TABLE IF NOT EXISTS profiles (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT UNIQUE NOT NULL,
    texture_model TEXT DEFAULT 'default',
    skin_hash TEXT,
    cape_hash TEXT,
    FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS tokens (
    access_token TEXT PRIMARY KEY,
    client_token TEXT NOT NULL,
    user_id TEXT NOT NULL,
    profile_id TEXT,
    created_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    server_id TEXT PRIMARY KEY,
    access_token TEXT NOT NULL,
    ip TEXT,
    created_at BIGINT NOT NULL
);
 
CREATE TABLE IF NOT EXISTS invites (
    code TEXT PRIMARY KEY,
    created_by TEXT,
    used_by TEXT,
    total_uses INTEGER DEFAULT 1,
    used_count INTEGER DEFAULT 0,
    created_at BIGINT,
    note TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT
);

CREATE TABLE IF NOT EXISTS user_textures (
    user_id TEXT NOT NULL,
    hash TEXT NOT NULL,
    texture_type TEXT NOT NULL,
    note TEXT DEFAULT '',
    model TEXT DEFAULT 'default',
    is_public INTEGER DEFAULT 0,
    created_at BIGINT NOT NULL,
    PRIMARY KEY(user_id, hash, texture_type),
    FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS skin_library (
    skin_hash TEXT PRIMARY KEY,
    texture_type TEXT NOT NULL,
    is_public INTEGER DEFAULT 0,
    uploader TEXT,
    model TEXT DEFAULT 'default',
    name TEXT DEFAULT '',
    created_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS fallback_endpoints (
    id SERIAL PRIMARY KEY,
    priority INTEGER NOT NULL,
    session_url TEXT NOT NULL,
    account_url TEXT NOT NULL,
    services_url TEXT NOT NULL,
    cache_ttl INTEGER NOT NULL,
    skin_domains TEXT DEFAULT '',
    enable_profile BOOLEAN DEFAULT TRUE,
    enable_hasjoined BOOLEAN DEFAULT TRUE,
    enable_whitelist BOOLEAN DEFAULT FALSE,
    note TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS whitelisted_users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    endpoint_id INTEGER NOT NULL,
    created_at BIGINT NOT NULL,
    UNIQUE(username, endpoint_id),
    FOREIGN KEY(endpoint_id) REFERENCES fallback_endpoints(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS verification_codes (
    email TEXT,
    code TEXT NOT NULL,
    type TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    expires_at BIGINT NOT NULL,
    PRIMARY KEY(email, type)
);
"""

class Database(BaseDB):
    def __init__(self, dsn: str, max_connections: int = 10):
        super().__init__(dsn, max_connections)
        self.user = UserModule(self)
        self.setting = SettingModule(self)
        self.texture = TextureModule(self)
        self.verification = VerificationModule(self)
        self.fallback = FallbackModule(self)

    async def init(self):
        """初始化表结构"""
        async with self.get_conn() as conn:
            # 创建基础表结构
            await conn.execute(INIT_SQL)
            
            # 初始化默认设置 (PostgreSQL ON CONFLICT)
            settings_to_init = [
                ('microsoft_client_id', ''),
                ('microsoft_client_secret', ''),
                ('microsoft_redirect_uri', 'http://localhost:8000/microsoft/callback'),
                ('fallback_strategy', 'serial'),
                ('enable_skin_library', 'true'),
                ('email_verify_enabled', 'false'),
                ('enable_strong_password_check', 'false'),
                ('email_verify_ttl', '300'),
                ('smtp_host', 'smtp.example.com'),
                ('smtp_port', '465'),
                ('smtp_user', 'user@example.com'),
                ('smtp_password', 'password'),
                ('smtp_ssl', 'true'),
                ('smtp_sender', 'SkinServer <no-reply@example.com>')
            ]
            for key, val in settings_to_init:
                await conn.execute(
                    "INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO NOTHING",
                    key, val
                )

        # 初始化模块缓存
        await self.setting.init()
        await self.fallback.init()
