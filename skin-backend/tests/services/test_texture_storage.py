import os
from io import BytesIO

import pytest
from PIL import Image

from services import TextureStorage


def _png_bytes(width=64, height=64, color=(255, 0, 0, 255)):
    buf = BytesIO()
    Image.new("RGBA", size=(width, height), color=color).save(buf, "png")
    buf.seek(0)
    return buf.read()


@pytest.fixture
def storage(tmp_path):
    return TextureStorage(str(tmp_path / "textures"))


def test_init_creates_directory(tmp_path):
    target = tmp_path / "nested" / "textures"
    assert not target.exists()
    TextureStorage(str(target))
    assert target.is_dir()


def test_process_and_save_valid_skin_returns_hash_and_writes_file(storage):
    tex_hash = storage.process_and_save(_png_bytes(64, 64), "skin")

    assert isinstance(tex_hash, str) and len(tex_hash) == 64  # sha256 hex
    saved = os.path.join(storage.textures_dir, f"{tex_hash}.png")
    assert os.path.exists(saved)


def test_process_and_save_hash_is_stable_for_same_pixels(storage):
    # Same pixel content must produce the same hash (content-addressed)
    h1 = storage.process_and_save(_png_bytes(64, 64, (10, 20, 30, 255)), "skin")
    h2 = storage.process_and_save(_png_bytes(64, 64, (10, 20, 30, 255)), "skin")
    assert h1 == h2

    # Different pixels must produce a different hash
    h3 = storage.process_and_save(_png_bytes(64, 64, (200, 100, 50, 255)), "skin")
    assert h3 != h1


def test_process_and_save_valid_cape(storage):
    tex_hash = storage.process_and_save(_png_bytes(64, 32), "cape")
    assert os.path.exists(os.path.join(storage.textures_dir, f"{tex_hash}.png"))


def test_process_and_save_invalid_skin_dimensions_raises(storage):
    # 63x63 is not a multiple of 64 → invalid skin dimensions
    with pytest.raises(ValueError):
        storage.process_and_save(_png_bytes(63, 63), "skin")

    # 100x100 is square but not a multiple of 64 → also invalid
    with pytest.raises(ValueError):
        storage.process_and_save(_png_bytes(100, 100), "skin")


def test_process_and_save_non_png_bytes_raises(storage):
    with pytest.raises(ValueError):
        storage.process_and_save(b"this is definitely not a png", "skin")


def test_process_and_save_jpeg_bytes_raises(storage):
    # A real image but wrong format (JPEG) must be rejected by normalize_png
    buf = BytesIO()
    Image.new("RGB", (64, 64), (0, 0, 0)).save(buf, "jpeg")
    buf.seek(0)
    with pytest.raises(ValueError):
        storage.process_and_save(buf.read(), "skin")


def test_delete_file_is_idempotent(storage):
    tex_hash = storage.process_and_save(_png_bytes(64, 64), "skin")
    path = os.path.join(storage.textures_dir, f"{tex_hash}.png")
    assert os.path.exists(path)

    storage.delete_file(tex_hash)
    assert not os.path.exists(path)
    # Deleting again must not raise
    storage.delete_file(tex_hash)


# ========== DoS hardening (Phase 2) ==========


def test_oversize_dimensions_rejected(storage):
    # 2048x2048 is a valid multiple of 64 but exceeds MAX_TEXTURE_DIMENSION (1024)
    with pytest.raises(ValueError):
        storage.process_and_save(_png_bytes(2048, 2048), "skin")


def test_hash_unchanged_after_alpha_zero_handling(storage):
    # Alpha=0 pixels must have RGB zeroed before hashing (spec). Two images that
    # differ only in the RGB of fully-transparent pixels must hash identically.
    h1 = storage.process_and_save(_png_bytes(64, 64, (10, 20, 30, 0)), "skin")
    h2 = storage.process_and_save(_png_bytes(64, 64, (200, 100, 50, 0)), "skin")
    assert h1 == h2


@pytest.mark.asyncio
async def test_process_and_save_async_matches_sync(storage):
    sync_hash = storage.process_and_save(_png_bytes(64, 64, (7, 8, 9, 255)), "skin")
    async_hash = await storage.process_and_save_async(
        _png_bytes(64, 64, (7, 8, 9, 255)), "skin"
    )
    assert sync_hash == async_hash


class _FakeSetting:
    def __init__(self, value):
        self._value = value

    async def get(self, key, default=None):
        return self._value


class _FakeDB:
    def __init__(self, max_kb):
        self.setting = _FakeSetting(str(max_kb))


@pytest.mark.asyncio
async def test_assert_texture_size_enforced():
    from services import assert_texture_size

    db = _FakeDB(max_kb=1)  # 1 KB limit
    # under the limit: passes
    await assert_texture_size(db, b"x" * 500)
    # over the limit: raises
    with pytest.raises(ValueError):
        await assert_texture_size(db, b"x" * (2 * 1024))
