import aiosqlite
import time

INIT_SQL = """
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    preferred_language TEXT DEFAULT 'zh_CN',
    display_name TEXT DEFAULT '',
    is_admin INTEGER DEFAULT 0
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
    created_at INTEGER
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
"""


class Database:
    def __init__(self, db_path="yggdrasil.db"):
        self.db_path = db_path

    async def init(self):
        async with aiosqlite.connect(self.db_path) as db:
            await db.executescript(INIT_SQL)
            await db.commit()

            # 迁移：如果旧数据库的 users 表缺少 is_admin 列，添加该列
            cur = await db.execute("PRAGMA table_info(users)")
            cols = await cur.fetchall()
            col_names = [c[1] for c in cols]
            # 添加 is_admin 列
            if "is_admin" not in col_names:
                await db.execute(
                    "ALTER TABLE users ADD COLUMN is_admin INTEGER DEFAULT 0"
                )
                await db.commit()
            # 添加 display_name 列
            if "display_name" not in col_names:
                await db.execute(
                    "ALTER TABLE users ADD COLUMN display_name TEXT DEFAULT ''"
                )
                await db.commit()

            # 迁移：为 invites 表添加使用次数字段
            cur = await db.execute("PRAGMA table_info(invites)")
            invite_cols = await cur.fetchall()
            invite_col_names = [c[1] for c in invite_cols]

            # 添加 total_uses 列（总使用次数，NULL表示无限制，0表示一次性）
            if "total_uses" not in invite_col_names:
                await db.execute(
                    "ALTER TABLE invites ADD COLUMN total_uses INTEGER DEFAULT 1"
                )
                await db.commit()

            # 添加 used_count 列（已使用次数）
            if "used_count" not in invite_col_names:
                await db.execute(
                    "ALTER TABLE invites ADD COLUMN used_count INTEGER DEFAULT 0"
                )
                await db.commit()
                # 为已使用的邀请码设置 used_count = 1
                await db.execute(
                    "UPDATE invites SET used_count = 1 WHERE used_by IS NOT NULL"
                )
                await db.commit()

            # 注意：不再在每次启动时自动设置管理员
            # 第一个用户的管理员设置已在注册逻辑中处理

    def get_conn(self):
        return aiosqlite.connect(self.db_path)
