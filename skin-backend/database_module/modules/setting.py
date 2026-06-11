from ..core import BaseDB

class SettingModule:
    def __init__(self, db: BaseDB):
        self.db = db
        self._cache = {}

    async def init(self):
        """Initialize cache from database"""
        rows = await self.db.fetch("SELECT key, value FROM settings")
        self._cache = {row[0]: row[1] for row in rows}

    async def get(self, key: str, default: str = None) -> str:
        """Get from cache with fallback to default"""
        return self._cache.get(key, default)

    async def set(self, key: str, value: str):
        """Update both DB and cache"""
        await self.db.execute(
            "INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value",
            key, value,
        )
        self._cache[key] = value

    async def set_many(self, conn, updates: dict):
        """在调用方提供的连接/事务上原子写入多个设置项；提交后由调用方刷新缓存。"""
        for key, value in updates.items():
            await conn.execute(
                "INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value",
                key, value,
            )

    def apply_cache_updates(self, updates: dict):
        """在事务成功后由调用方调用，将 set_many 写入的值同步到内存缓存。"""
        for key, value in updates.items():
            self._cache[key] = value

    async def get_all(self) -> dict:
        """Return a copy of the cache"""
        return self._cache.copy()
