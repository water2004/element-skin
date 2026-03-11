from .core import BaseDB
from .modules.user import UserModule
from .modules.setting import SettingModule
from .modules.texture import TextureModule
from .modules.verification import VerificationModule
from .modules.fallback import FallbackModule
from config_loader import config
import asyncio

class Database(BaseDB):
    def __init__(self, dsn: str, max_connections: int = 10):
        super().__init__(dsn, max_connections)
        self.user = UserModule(self)
        self.setting = SettingModule(self)
        self.texture = TextureModule(self)
        self.verification = VerificationModule(self)
        self.fallback = FallbackModule(self)

    async def init(self):
        """初始化模块缓存 (表结构已由 Docker init.sql 创建)"""
        # 如果不是在 Docker 环境运行，或者想保留自动建表作为回退方案，
        # 也可以在这里保留建表 SQL。但为了符合“由容器初始化”的要求，
        # 这里仅触发各模块的内部初始化逻辑（如缓存加载）。
        
        await self.setting.init()
        await self.fallback.init()
