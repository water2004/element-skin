import pytest
from io import BytesIO
from fastapi import HTTPException
from PIL import Image
from utils.uuid_utils import generate_random_uuid


def _create_test_image(width=64, height=64, color=(255, 0, 0, 255)):
    """创建一个测试用的 PNG 字节流"""
    file = BytesIO()
    image = Image.new('RGBA', size=(width, height), color=color)
    image.save(file, 'png')
    file.name = 'test.png'
    file.seek(0)
    return file.read()


@pytest.mark.asyncio
async def test_update_texture_public_invalid_value(admin_backend_fixture):
    """验证 is_public 枚举校验：非 0/1 值应返回 400"""
    with pytest.raises(HTTPException) as exc:
        await admin_backend_fixture.update_texture_public("somehash", is_public=2)
    assert exc.value.status_code == 400


@pytest.mark.asyncio
async def test_update_texture_public_not_found(admin_backend_fixture):
    """验证不存在的材质返回 404"""
    with pytest.raises(HTTPException) as exc:
        await admin_backend_fixture.update_texture_public("nonexistent-hash", is_public=0)
    assert exc.value.status_code == 404


@pytest.mark.asyncio
async def test_update_texture_public_success(admin_backend_fixture, db_session, user_factory):
    """验证成功更新材质的公开状态"""
    user = await user_factory()
    image_bytes = _create_test_image(64, 64)
    tex_hash, _ = await db_session.texture.upload(
        user.id, image_bytes, "skin", note="TestTexture", is_public=True, model="default"
    )

    # Toggle to 0
    result = await admin_backend_fixture.update_texture_public(tex_hash, is_public=0)
    assert result["success"] is True

    # Verify skin_library updated
    async with db_session.get_conn() as conn:
        lib_is_public = await conn.fetchval(
            "SELECT is_public FROM skin_library WHERE skin_hash = $1", tex_hash
        )
        assert lib_is_public == 0

    # Verify user_textures updated
    info = await db_session.texture.get_texture_info(user.id, tex_hash, "skin")
    assert info["is_public"] == 0


@pytest.mark.asyncio
async def test_update_texture_model_invalid_value(admin_backend_fixture):
    """验证 model 枚举校验：无效值应返回 400"""
    with pytest.raises(HTTPException) as exc:
        await admin_backend_fixture.update_texture_model("somehash", model="invalid")
    assert exc.value.status_code == 400


@pytest.mark.asyncio
async def test_update_texture_model_not_found(admin_backend_fixture):
    """验证不存在的材质返回 404"""
    with pytest.raises(HTTPException) as exc:
        await admin_backend_fixture.update_texture_model("nonexistent-hash", model="slim")
    assert exc.value.status_code == 404


@pytest.mark.asyncio
async def test_update_texture_model_success(admin_backend_fixture, db_session, user_factory):
    """验证成功更新材质模型（三表同步）"""
    user = await user_factory()
    image_bytes = _create_test_image(64, 64)
    tex_hash, _ = await db_session.texture.upload(
        user.id, image_bytes, "skin", note="TestTexture", is_public=True, model="default"
    )

    # Update model from default to slim
    result = await admin_backend_fixture.update_texture_model(tex_hash, model="slim")
    assert result["success"] is True

    # Verify skin_library updated
    async with db_session.get_conn() as conn:
        lib_model = await conn.fetchval(
            "SELECT model FROM skin_library WHERE skin_hash = $1", tex_hash
        )
        assert lib_model == "slim"

    # Verify user_textures updated
    info = await db_session.texture.get_texture_info(user.id, tex_hash, "skin")
    assert info["model"] == "slim"


@pytest.mark.asyncio
async def test_delete_texture_missing_user_id(admin_backend_fixture):
    """验证 per-user mode 需要 user_id（force=False 且无 user_id 应返回 400）"""
    with pytest.raises(HTTPException) as exc:
        await admin_backend_fixture.delete_texture("somehash", "skin", user_id=None, force=False)
    assert exc.value.status_code == 400


@pytest.mark.asyncio
async def test_delete_texture_force_mode(admin_backend_fixture, db_session, user_factory):
    """验证 force mode 删除所有引用和皮肤库记录"""
    user1 = await user_factory()
    user2 = await user_factory()
    image_bytes = _create_test_image(64, 64)
    tex_hash, tex_type = await db_session.texture.upload(
        user1.id, image_bytes, "skin", note="ForceTarget", is_public=True, model="default"
    )
    # Add same texture to another user's wardrobe
    await db_session.texture.add_to_library(
        user2.id, tex_hash, tex_type, note="Copy", is_public=True, model="default"
    )

    # Force delete
    result = await admin_backend_fixture.delete_texture(tex_hash, tex_type, force=True)
    assert result["success"] is True

    # Verify all references gone
    assert await db_session.texture.verify_ownership(user1.id, tex_hash, tex_type) is False
    assert await db_session.texture.verify_ownership(user2.id, tex_hash, tex_type) is False

    # Verify skin_library entry gone
    async with db_session.get_conn() as conn:
        lib_val = await conn.fetchval(
            "SELECT 1 FROM skin_library WHERE skin_hash = $1", tex_hash
        )
        assert lib_val is None


@pytest.mark.asyncio
async def test_delete_texture_per_user_last_ref_removes_library(admin_backend_fixture, db_session, user_factory):
    """验证 per-user mode 最后一个引用时物理删除 skin_library（无残留）"""
    user = await user_factory()
    image_bytes = _create_test_image(64, 64)
    tex_hash, tex_type = await db_session.texture.upload(
        user.id, image_bytes, "skin", note="LastRef", is_public=True, model="default"
    )

    # Only 1 user has this texture, so deleting it should also remove skin_library
    result = await admin_backend_fixture.delete_texture(tex_hash, tex_type, user_id=user.id, force=False)
    assert result["success"] is True

    # Verify user texture removed
    assert await db_session.texture.verify_ownership(user.id, tex_hash, tex_type) is False

    # Verify skin_library is also removed (no orphan residue)
    async with db_session.get_conn() as conn:
        lib_val = await conn.fetchval(
            "SELECT 1 FROM skin_library WHERE skin_hash = $1", tex_hash
        )
        assert lib_val is None


@pytest.mark.asyncio
async def test_delete_texture_per_user_success(admin_backend_fixture, db_session, user_factory):
    """验证 per-user 删除成功（多个用户收藏时只删除指定用户的）"""
    user1 = await user_factory()
    user2 = await user_factory()
    image_bytes = _create_test_image(64, 64)
    tex_hash, tex_type = await db_session.texture.upload(
        user1.id, image_bytes, "skin", note="Shared", is_public=True, model="default"
    )
    await db_session.texture.add_to_library(
        user2.id, tex_hash, tex_type, note="Shared2", is_public=True, model="default"
    )

    # Delete user1's copy
    result = await admin_backend_fixture.delete_texture(tex_hash, tex_type, user_id=user1.id, force=False)
    assert result["success"] is True

    # user1's reference should be gone, user2's should remain
    assert await db_session.texture.verify_ownership(user1.id, tex_hash, tex_type) is False
    assert await db_session.texture.verify_ownership(user2.id, tex_hash, tex_type) is True
