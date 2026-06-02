import asyncio
import time

import pytest

from routes_reference import _refresh_cleanup_loop


@pytest.mark.asyncio
async def test_cleanup_loop_removes_expired_then_cancels(db_session, user_factory):
    """周期清理：跑一轮后过期行消失、未过期行保留；取消时干净退出。"""
    user = await user_factory()
    now = int(time.time() * 1000)
    past = now - 10_000
    future = now + 7 * 24 * 3600 * 1000

    await db_session.user.add_refresh_token("hash_old", user.id, past, now)
    await db_session.user.add_refresh_token("hash_new", user.id, future, now)

    # interval 设很短，确保第一轮 sleep 立即过去并执行一次清理
    task = asyncio.create_task(_refresh_cleanup_loop(db_session, interval_seconds=0.01))
    try:
        # 轮询等待清理生效（避免对调度时序做硬编码假设）
        for _ in range(200):
            await asyncio.sleep(0.01)
            if (await db_session.user.get_refresh_token("hash_old")) is None:
                break
    finally:
        task.cancel()
        # 取消应被 CancelledError 分支吞掉，await 不抛
        await task

    assert task.cancelled() or task.done()
    assert (await db_session.user.get_refresh_token("hash_old")) is None
    assert (await db_session.user.get_refresh_token("hash_new")) is not None


@pytest.mark.asyncio
async def test_cleanup_loop_survives_cleanup_error(db_session):
    """单轮清理抛错不应中断循环：记录后继续，直到被取消。"""

    class _FlakyUser:
        def __init__(self):
            self.calls = 0

        async def delete_expired_refresh_tokens(self, cutoff):
            self.calls += 1
            raise RuntimeError("boom")

    class _FlakyDB:
        def __init__(self):
            self.user = _FlakyUser()

    flaky = _FlakyDB()
    task = asyncio.create_task(_refresh_cleanup_loop(flaky, interval_seconds=0.01))
    try:
        # 等到至少发生两次调用，证明抛错后循环仍在继续
        for _ in range(200):
            await asyncio.sleep(0.01)
            if flaky.user.calls >= 2:
                break
    finally:
        task.cancel()
        await task

    assert flaky.user.calls >= 2
