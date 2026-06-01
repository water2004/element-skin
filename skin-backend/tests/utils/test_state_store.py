"""InMemoryStateStore 单元测试。"""

import time

from utils.state_store import InMemoryStateStore


def test_put_then_pop_returns_value():
    store = InMemoryStateStore()
    store.put("k", {"user_id": "u1"}, ttl_seconds=600)
    assert store.pop("k") == {"user_id": "u1"}


def test_pop_is_one_shot():
    store = InMemoryStateStore()
    store.put("k", "v", ttl_seconds=600)
    assert store.pop("k") == "v"
    # 取出即删，再次 pop 返回 None
    assert store.pop("k") is None


def test_pop_missing_key_returns_none():
    store = InMemoryStateStore()
    assert store.pop("nope") is None


def test_expired_item_pop_returns_none():
    store = InMemoryStateStore()
    store.put("k", "v", ttl_seconds=0)
    time.sleep(0.01)
    assert store.pop("k") is None


def test_sweep_removes_expired_on_put():
    store = InMemoryStateStore()
    store.put("old", "v", ttl_seconds=0)
    time.sleep(0.01)
    # 再 put 一个新项会触发 _sweep，清理过期的 "old"
    store.put("new", "v2", ttl_seconds=600)
    assert "old" not in store._data
    assert store.pop("new") == "v2"
