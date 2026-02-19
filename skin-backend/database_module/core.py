import aiosqlite
import asyncio

class BaseDB:
    def __init__(self, db_path: str, max_connections: int = 10):
        self.db_path = db_path
        self._pool = asyncio.Queue()
        self._all_conns = []
        self._max_connections = max_connections
        self._initialized = False
        self._init_lock = asyncio.Lock()

    async def _create_connection(self):
        """创建一个开启了 WAL 模式和性能优化的连接"""
        conn = await aiosqlite.connect(self.db_path)
        # 开启 WAL 模式：允许多个并发读取和一个写入并发执行
        await conn.execute("PRAGMA journal_mode=WAL;")
        # 设置 synchronous=NORMAL：在 WAL 模式下既能保证数据安全又能显著提升写入性能
        await conn.execute("PRAGMA synchronous=NORMAL;")
        # 设置繁忙超时：在并发写入冲突时等待而不是立即报错，配合池化连接提高稳定性
        await conn.execute("PRAGMA busy_timeout=5000;")
        # 开启内存缓存：设置为 2000 页 (约 2MB) 或更多，取决于数据库大小
        await conn.execute("PRAGMA cache_size=-2000;")  # 负数表示 KB，此处为 2MB
        # 开启内存映射：减少磁盘 IO 带来的系统调用开销
        await conn.execute("PRAGMA mmap_size=268435456;") # 256MB
        await conn.commit()
        return conn

    async def connect(self):
        """初始化连接池，建立多个持久化连接"""
        async with self._init_lock:
            if self._initialized:
                return
            for _ in range(self._max_connections):
                conn = await self._create_connection()
                self._all_conns.append(conn)
                self._pool.put_nowait(conn)
            self._initialized = True

    async def close(self):
        """关闭池中所有连接"""
        async with self._init_lock:
            if not self._initialized:
                return
            for conn in self._all_conns:
                await conn.close()
            self._all_conns = []
            while not self._pool.empty():
                self._pool.get_nowait()
            self._initialized = False

    async def ensure_conn(self):
        """确保连接池已初始化（保持 API 兼容性）"""
        if not self._initialized:
            await self.connect()

    def get_conn(self):
        """
        获取数据库连接的上下文管理器。
        移除 Python 层的全局锁，允许多个协程通过池化连接并发访问数据库。
        用法：async with db.get_conn() as conn:
        """
        return PoolContext(self)

class PoolContext:
    def __init__(self, db: BaseDB):
        self.db = db
        self.conn = None

    async def __aenter__(self):
        if not self.db._initialized:
            await self.db.connect()
        # 从队列中获取一个可用连接
        self.conn = await self.db._pool.get()
        return self.conn

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        if self.conn:
            # 使用完毕后将连接归还队列
            self.db._pool.put_nowait(self.conn)
            self.conn = None
