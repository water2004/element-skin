"""角色名校验与唯一名生成的纯函数测试"""

import pytest
from utils.profile_naming import is_valid_profile_name, generate_unique_profile_name


def test_is_valid_profile_name_accepts_normal():
    assert is_valid_profile_name("Player1") is True
    assert is_valid_profile_name("a") is True
    assert is_valid_profile_name("A_b_3") is True
    assert is_valid_profile_name("x" * 16) is True


def test_is_valid_profile_name_rejects_empty():
    assert is_valid_profile_name("") is False
    assert is_valid_profile_name(None) is False


def test_is_valid_profile_name_rejects_too_long():
    assert is_valid_profile_name("x" * 17) is False


def test_is_valid_profile_name_rejects_illegal_chars():
    assert is_valid_profile_name("has space") is False
    assert is_valid_profile_name("emoji😀") is False
    assert is_valid_profile_name("dash-name") is False
    assert is_valid_profile_name("dot.name") is False


@pytest.mark.asyncio
async def test_generate_unique_profile_name_base_available():
    async def exists(n):
        return False

    assert await generate_unique_profile_name("Steve", exists) == "Steve"


@pytest.mark.asyncio
async def test_generate_unique_profile_name_adds_suffix():
    taken = {"Steve", "Steve_1", "Steve_2"}

    async def exists(n):
        return n in taken

    assert await generate_unique_profile_name("Steve", exists) == "Steve_3"


@pytest.mark.asyncio
async def test_generate_unique_profile_name_exceeds_max_attempts():
    # everything is taken → must raise after max_attempts
    async def exists(n):
        return True

    with pytest.raises(ValueError):
        await generate_unique_profile_name("Steve", exists, max_attempts=5)
