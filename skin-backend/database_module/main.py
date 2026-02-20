from .core import BaseDB
from .modules.user import UserModule
from .modules.setting import SettingModule
from .modules.texture import TextureModule
from .modules.verification import VerificationModule

INIT_SQL = """
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    preferred_language TEXT DEFAULT 'zh_CN',
    display_name TEXT DEFAULT '',
    is_admin INTEGER DEFAULT 0,
    banned_until INTEGER DEFAULT NULL
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
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    server_id TEXT PRIMARY KEY,
    access_token TEXT NOT NULL,
    ip TEXT,
    created_at INTEGER NOT NULL
);
 
CREATE TABLE IF NOT EXISTS invites (
    code TEXT PRIMARY KEY,
    created_by TEXT,
    used_by TEXT,
    total_uses INTEGER DEFAULT 1,
    used_count INTEGER DEFAULT 0,
    created_at INTEGER,
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
    created_at INTEGER NOT NULL,
    PRIMARY KEY(user_id, hash, texture_type),
    FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS official_whitelist (
    username TEXT PRIMARY KEY,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS verification_codes (
    email TEXT,
    code TEXT NOT NULL,
    type TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    PRIMARY KEY(email, type)
);
"""

class Database(BaseDB):
    def __init__(self, db_path="yggdrasil.db", max_connections: int = 10):
        super().__init__(db_path, max_connections)
        self.user = UserModule(self)
        self.setting = SettingModule(self)
        self.texture = TextureModule(self)
        self.verification = VerificationModule(self)

    async def init(self):
        """初始化表结构及执行迁移"""
        async with self.get_conn() as conn:
            # 创建基础表结构
            await conn.executescript(INIT_SQL)
            await conn.commit()

            # 兼容旧库新增列
            cursor = await conn.execute("PRAGMA table_info(invites)")
            columns = [row[1] for row in await cursor.fetchall()]
            if "note" not in columns:
                await conn.execute(
                    "ALTER TABLE invites ADD COLUMN note TEXT DEFAULT ''"
                )
                await conn.commit()
            
            # 初始化默认设置
            await conn.execute(
                "INSERT OR IGNORE INTO settings (key, value) VALUES ('microsoft_client_id', '')"
            )
            await conn.execute(
                "INSERT OR IGNORE INTO settings (key, value) VALUES ('microsoft_client_secret', '')"
            )
            await conn.execute(
                "INSERT OR IGNORE INTO settings (key, value) VALUES ('microsoft_redirect_uri', 'http://localhost:8000/microsoft/callback')"
            )
            await conn.execute(
                "INSERT OR IGNORE INTO settings (key, value) VALUES ('fallback_mojang_profile', 'false')"
            )
            await conn.execute(
                "INSERT OR IGNORE INTO settings (key, value) VALUES ('fallback_mojang_hasjoined', 'false')"
            )
            await conn.execute(
                "INSERT OR IGNORE INTO settings (key, value) VALUES ('enable_official_whitelist', 'false')"
            )
            # await conn.execute(
            #     "INSERT OR IGNORE INTO settings (key, value) VALUES ('password_strength_enabled', 'false')"
            # )
            
            # SMTP Default Settings
            await conn.execute(
                "INSERT OR IGNORE INTO settings (key, value) VALUES ('email_verify_enabled', 'false')"
            )
            await conn.execute(
                "INSERT OR IGNORE INTO settings (key, value) VALUES ('enable_strong_password_check', 'false')"
            )
            await conn.execute(
                "INSERT OR IGNORE INTO settings (key, value) VALUES ('email_verify_ttl', '300')"
            )
            await conn.execute(
                "INSERT OR IGNORE INTO settings (key, value) VALUES ('smtp_host', 'smtp.example.com')"
            )
            await conn.execute(
                "INSERT OR IGNORE INTO settings (key, value) VALUES ('smtp_port', '465')"
            )
            await conn.execute(
                "INSERT OR IGNORE INTO settings (key, value) VALUES ('smtp_user', 'user@example.com')"
            )
            await conn.execute(
                "INSERT OR IGNORE INTO settings (key, value) VALUES ('smtp_password', 'password')"
            )
            await conn.execute(
                "INSERT OR IGNORE INTO settings (key, value) VALUES ('smtp_ssl', 'true')"
            )
            await conn.execute(
                "INSERT OR IGNORE INTO settings (key, value) VALUES ('smtp_sender', 'SkinServer <no-reply@example.com>')"
            )
            
            await conn.commit()

    # Proxy methods for backward compatibility or direct access if needed
    # But strictly speaking, the user asked for db.user.xxx
    # We will only expose raw connection via get_conn()
