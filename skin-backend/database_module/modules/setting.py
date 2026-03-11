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

    async def get_all(self) -> dict:
        """Return a copy of the cache"""
        return self._cache.copy()
