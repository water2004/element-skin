"""
配置文件加载器
支持 YAML 配置文件，并提供环境变量覆盖
"""

import os
import yaml
from typing import Any, Dict


class Config:
    def __init__(self, config_path: str = "config.yaml"):
        self.config_path = config_path
        self._data = {}
        self.load()

    def load(self):
        """加载配置文件"""
        if os.path.exists(self.config_path):
            with open(self.config_path, "r", encoding="utf-8") as f:
                self._data = yaml.safe_load(f) or {}
        else:
            print(f"Warning: Config file {self.config_path} not found, using defaults")
            self._data = self._get_defaults()

    def _get_defaults(self) -> Dict[str, Any]:
        """默认配置"""
        return {
            "jwt": {
                "secret": "dev-secret-please-change-in-production",
                "expire_days": 7,
            },
            "rate_limit": {
                "enabled": True,
                "auth_attempts": 5,
                "auth_window_minutes": 15,
                "general_requests": 100,
                "general_window_seconds": 60,
            },
            "database": {"path": "yggdrasil.db"},
            "textures": {"directory": "textures", "max_size_kb": 1024},
            "server": {"host": "0.0.0.0", "port": 8000},
        }

    def get(self, key: str, default: Any = None) -> Any:
        """
        获取配置值，支持点号分隔的嵌套键
        例如: config.get('jwt.secret')
        环境变量优先级更高，格式：KEY__SUBKEY（双下划线）
        例如: JWT__SECRET 会覆盖 jwt.secret
        """
        # 检查环境变量（转换为大写，点号改为双下划线）
        env_key = key.upper().replace(".", "__")
        env_value = os.getenv(env_key)
        if env_value is not None:
            # 尝试转换类型
            if env_value.lower() in ("true", "false"):
                return env_value.lower() == "true"
            try:
                return int(env_value)
            except ValueError:
                try:
                    return float(env_value)
                except ValueError:
                    return env_value

        # 从配置文件读取
        keys = key.split(".")
        value = self._data
        for k in keys:
            if isinstance(value, dict):
                value = value.get(k)
            else:
                return default
            if value is None:
                return default
        return value if value is not None else default


# 全局配置实例
config = Config()
