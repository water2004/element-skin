"""角色名校验与唯一名生成（纯函数，无状态）"""

import re
from typing import Awaitable, Callable

PROFILE_NAME_RE = re.compile(r"^[a-zA-Z0-9_]{1,16}$")
PROFILE_NAME_MAX_LEN = 16


def is_valid_profile_name(name: str) -> bool:
    return bool(name) and bool(PROFILE_NAME_RE.match(name))


def profile_name_candidate(base: str, attempt: int) -> str:
    """根据基名与重试序号生成不超过 16 字符的候选名（与 go 实现保持一致）。

    第 0 次直接使用 base（截断到 16）；之后追加 _N 后缀，必要时截断 base 段腾出空间。
    """
    suffix = "" if attempt <= 0 else f"_{attempt}"
    if len(suffix) >= PROFILE_NAME_MAX_LEN:
        return suffix[-PROFILE_NAME_MAX_LEN:]
    keep = PROFILE_NAME_MAX_LEN - len(suffix)
    return base[:keep] + suffix


async def generate_unique_profile_name(
    base: str,
    exists: Callable[[str], Awaitable[bool]],
    max_attempts: int = 100,
) -> str:
    """base 被占用时尝试 base_1, base_2 ...；超出 max_attempts 抛 ValueError。

    候选名始终被截断到 16 字符内，避免把不合法的过长名传给后续校验。
    exists: async 谓词，传入候选名返回该名是否已存在。
    """
    for attempt in range(max_attempts):
        candidate = profile_name_candidate(base, attempt)
        if not await exists(candidate):
            return candidate
    raise ValueError("无法生成唯一的角色名称")
