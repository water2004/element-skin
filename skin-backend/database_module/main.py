from .core import BaseDB
from .modules.user import UserModule
from .modules.setting import SettingModule
from .modules.texture import TextureModule
from .modules.verification import VerificationModule
from .modules.fallback import FallbackModule
from config_loader import config
import asyncio

from .initsql import INIT_SQL

class Database(BaseDB):
    def __init__(self, dsn: str, max_connections: int = 10):
        super().__init__(dsn, max_connections)
        self.user = UserModule(self)
        self.setting = SettingModule(self)
        self.texture = TextureModule(self)
        self.verification = VerificationModule(self)
        self.fallback = FallbackModule(self)

    async def init(self):
        """同步数据库结构并初始化模块缓存"""
        # 1. 直接运行初始化 SQL (幂等)
        try:
            await self.ensure_conn()
            await self.execute(INIT_SQL)
        except Exception as e:
            print(f"⚠️ 数据库初始化失败: {e}")

        # 2. 触发各模块内部逻辑 (加载缓存等)
        await self.setting.init()
        await self.fallback.init()
