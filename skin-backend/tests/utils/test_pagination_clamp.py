"""分页 limit 收敛工具测试（Phase 8）"""

import pytest

from utils.pagination import clamp_limit, DEFAULT_LIMIT, MAX_LIMIT


def test_clamp_negative_to_one():
    assert clamp_limit(-1) == 1


def test_clamp_zero_to_one():
    assert clamp_limit(0) == 1


def test_clamp_huge_to_max():
    assert clamp_limit(10_000) == MAX_LIMIT
    assert clamp_limit(99_999_999) == MAX_LIMIT


def test_clamp_none_to_default():
    assert clamp_limit(None) == DEFAULT_LIMIT


def test_clamp_none_custom_default():
    assert clamp_limit(None, default=15) == 15


def test_clamp_unparseable_to_default():
    assert clamp_limit("abc") == DEFAULT_LIMIT
    assert clamp_limit("abc", default=15) == 15


def test_clamp_numeric_string_parsed():
    assert clamp_limit("50") == 50
    assert clamp_limit("500") == MAX_LIMIT
    assert clamp_limit("-3") == 1


def test_clamp_normal_passthrough():
    assert clamp_limit(1) == 1
    assert clamp_limit(20) == 20
    assert clamp_limit(MAX_LIMIT) == MAX_LIMIT


def test_clamp_boundary():
    assert clamp_limit(MAX_LIMIT + 1) == MAX_LIMIT
    assert clamp_limit(MAX_LIMIT - 1) == MAX_LIMIT - 1
