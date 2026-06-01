"""SSRF 出站 URL 校验测试。"""

import pytest

from utils.url_guard import validate_outbound_url, UnsafeURLError


@pytest.mark.parametrize(
    "url",
    [
        "http://127.0.0.1/x",
        "http://localhost/x",
        "http://169.254.169.254/latest/meta-data",  # cloud metadata
        "http://10.0.0.5/x",
        "http://192.168.1.1/x",
        "http://172.16.0.1/x",
        "http://[::1]/x",
        "http://0.0.0.0/x",
        "file:///etc/passwd",
        "ftp://internal/x",
        "",
    ],
)
async def test_blocked_urls(url):
    with pytest.raises(UnsafeURLError):
        await validate_outbound_url(url)


@pytest.mark.parametrize(
    "url",
    [
        "https://example.com/skin.png",
        "http://1.1.1.1/x",  # public IP literal
    ],
)
async def test_allowed_urls(url):
    # Should not raise (public hosts / public IP literal).
    await validate_outbound_url(url)
