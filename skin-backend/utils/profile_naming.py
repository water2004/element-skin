"""角色名校验与唯一名生成（纯函数，无状态）"""

import re
from typing import Awaitable, Callable

PROFILE_NAME_RE = re.compile(r"^[a-zA-Z0-9_]{1,16}$")


def is_valid_profile_name(name: str) -> bool:
    return bool(name) and bool(PROFILE_NAME_RE.match(name))


async def generate_unique_profile_name(
    base: str,
    exists: Callable[[str], Awaitable[bool]],
    max_attempts: int = 100,
) -> str:
    """base 被占用时尝试 base_1, base_2 ...；超出 max_attempts 抛 ValueError。

    exists: async 谓词，传入候选名返回该名是否已存在。
    """
    candidate = base
    suffix = 1
    while await exists(candidate):
        candidate = f"{base}_{suffix}"
        suffix += 1
        if suffix > max_attempts:
            raise ValueError("无法生成唯一的角色名称")
    return candidate
