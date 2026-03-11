import asyncpg
import asyncio

class BaseDB:
    def __init__(self, dsn: str, max_connections: int = 10):
        self.dsn = dsn
        self._pool = None
        self._max_connections = max_connections
        self._initialized = False
        self._init_lock = asyncio.Lock()

    async def connect(self):
        """初始化连接池"""
        async with self._init_lock:
            if self._initialized:
                return
            self._pool = await asyncpg.create_pool(
                dsn=self.dsn,
                min_size=2,
                max_size=self._max_connections
            )
            self._initialized = True

    async def close(self):
        """关闭连接池"""
        async with self._init_lock:
            if not self._initialized:
                return
            await self._pool.close()
            self._pool = None
            self._initialized = False

    async def ensure_conn(self):
        """确保连接池已初始化"""
        if not self._initialized:
            await self.connect()

    def get_conn(self):
        """
        获取数据库连接的上下文管理器。
        用法：async with db.get_conn() as conn:
        """
        return PoolContext(self)

    async def execute(self, query: str, *args):
        """直接执行 SQL"""
        async with self.get_conn() as conn:
            return await conn.execute(query, *args)

    async def fetch(self, query: str, *args):
        """查询多行"""
        async with self.get_conn() as conn:
            return await conn.fetch(query, *args)

    async def fetchrow(self, query: str, *args):
        """查询单行"""
        async with self.get_conn() as conn:
            return await conn.fetchrow(query, *args)

    async def fetchval(self, query: str, *args, column: int = 0):
        """查询单个值"""
        async with self.get_conn() as conn:
            return await conn.fetchval(query, *args, column=column)

class PoolContext:
    def __init__(self, db: BaseDB):
        self.db = db
        self.conn = None

    async def __aenter__(self):
        if not self.db._initialized:
            await self.db.connect()
        self.conn = await self.db._pool.acquire()
        return self.conn

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        if self.conn:
            await self.db._pool.release(self.conn)
            self.conn = None
