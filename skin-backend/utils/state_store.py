"""一次性、带过期的 state 存储抽象。

用于 OAuth state / 临时 token 等「写入一次、限时取用一次」的场景。
默认实现为带 TTL 主动清理的内存版：修复了裸 dict「过期项仅在被再次命中时才删除」
导致的缓慢内存泄漏，同时保持零外部依赖。

注意：内存实现仅适用于单实例部署。多 worker / 多实例需替换为 Redis 等
跨进程实现——路由层只依赖 put/pop 接口，切换实现无需改动调用方。
"""

import time
from typing import Any, Optional


class StateStore:
    """state 存储接口。实现需保证 pop 取出即删（一次性语义）。"""

    def put(self, key: str, value: Any, ttl_seconds: int) -> None:
        raise NotImplementedError

    def pop(self, key: str) -> Optional[Any]:
        raise NotImplementedError


class InMemoryStateStore(StateStore):
    """带 TTL 的内存 state 存储；put 时周期性清理过期项，pop 取出即删。

    仅适用于单实例部署。多实例需替换为 Redis 实现。
    """

    def __init__(self) -> None:
        self._data: dict[str, tuple[float, Any]] = {}

    def put(self, key: str, value: Any, ttl_seconds: int) -> None:
        self._data[key] = (time.time() + ttl_seconds, value)
        self._sweep()

    def pop(self, key: str) -> Optional[Any]:
        item = self._data.pop(key, None)
        if not item:
            return None
        expires_at, value = item
        if time.time() > expires_at:
            return None
        return value

    def _sweep(self) -> None:
        now = time.time()
        expired = [k for k, (exp, _) in self._data.items() if now > exp]
        for k in expired:
            self._data.pop(k, None)
