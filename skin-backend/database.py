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

            # 自动将编号最小的用户设为管理员
            cur = await db.execute("SELECT id FROM users ORDER BY id LIMIT 1")
            first_user = await cur.fetchone()
            if first_user:
                await db.execute(
                    "UPDATE users SET is_admin=1 WHERE id=?", (first_user[0],)
                )
                await db.commit()

    def get_conn(self):
        return aiosqlite.connect(self.db_path)
