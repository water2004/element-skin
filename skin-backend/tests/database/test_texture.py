import pytest
import os
from io import BytesIO
from PIL import Image
from utils.uuid_utils import generate_random_uuid

def create_test_image(width=64, height=64):
    """创建一个测试用的 PNG 字节流"""
    file = BytesIO()
    image = Image.new('RGBA', size=(width, height), color=(255, 0, 0, 255))
    image.save(file, 'png')
    file.name = 'test.png'
    file.seek(0)
    return file.read()

@pytest.mark.asyncio
async def test_texture_upload_and_library(db_session, user_factory):
    """测试材质上传及皮肤库接口"""
    user = await user_factory()
    image_bytes = create_test_image(64, 64) # 标准 64x64 皮肤
    
    # 1. Upload
    tex_hash, tex_type = await db_session.texture.upload(
        user.id, image_bytes, "skin", note="MySkin", is_public=True, model="default"
    )
    assert tex_hash is not None
    assert tex_type == "skin"
    
    # 验证文件是否保存
    assert os.path.exists(os.path.join(db_session.texture.textures_dir, f"{tex_hash}.png"))
    
    # 2. Get for user
    user_textures = await db_session.texture.get_for_user(user.id)
    assert len(user_textures) == 1
    assert user_textures[0][0] == tex_hash
    
    # 3. Get texture info
    info = await db_session.texture.get_texture_info(user.id, tex_hash, "skin")
    assert info["note"] == "MySkin"
    assert info["is_public"] == 1
    
    # 4. Verify ownership
    assert await db_session.texture.verify_ownership(user.id, tex_hash, "skin") is True
    
    # 5. Library actions
    lib_items = await db_session.texture.get_from_library(only_public=True)
    assert len(lib_items) == 1
    assert lib_items[0][0] == tex_hash
    
    count = await db_session.texture.count_library(only_public=True)
    assert count == 1
    
    # 6. Update actions
    await db_session.texture.update_note(user.id, tex_hash, "skin", "NewNote")
    await db_session.texture.update_model(user.id, tex_hash, "skin", "slim")
    await db_session.texture.update_is_public(user.id, tex_hash, "skin", False)
    
    updated_info = await db_session.texture.get_texture_info(user.id, tex_hash, "skin")
    assert updated_info["note"] == "NewNote"
    assert updated_info["model"] == "slim"
    assert updated_info["is_public"] == 0
    
    # 7. Add to wardrobe (from library)
    user2 = await user_factory()
    success = await db_session.texture.add_to_user_wardrobe(user2.id, tex_hash)
    assert success is True
    assert len(await db_session.texture.get_for_user(user2.id)) == 1
    
    # 8. Delete
    await db_session.texture.delete_from_library(user.id, tex_hash, "skin")
    assert len(await db_session.texture.get_for_user(user.id)) == 0

@pytest.mark.asyncio
async def test_texture_model_cascade_update(db_session, user_factory):
    """测试更新皮肤模型时，自动同步更新所有使用该皮肤的角色的模型"""
    user = await user_factory()
    image_bytes = create_test_image(64, 64)
    
    # 1. 上传皮肤
    tex_hash, _ = await db_session.texture.upload(user.id, image_bytes, "skin", model="default")
    
    # 2. 创建角色并应用该皮肤
    from utils.typing import PlayerProfile
    pid = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "ModelTester", "default"))
    await db_session.user.update_profile_skin(pid, tex_hash)
    
    # 3. 更新材质模型为 slim
    await db_session.texture.update_model(user.id, tex_hash, "skin", "slim")
    
    # 4. 验证级联更新：角色的 texture_model 应该也变成了 slim
    profile = await db_session.user.get_profile_by_id(pid)
    assert profile.texture_model == "slim"
    
    # 5. 验证非上传者更新 (不应报错，但也不会影响全局库)
    user2 = await user_factory()
    await db_session.texture.add_to_user_wardrobe(user2.id, tex_hash)
    await db_session.texture.update_model(user2.id, tex_hash, "skin", "default")
    
    # 角色1的模型不应被 user2 的操作改变 (因为 user2 不是上传者)
    assert (await db_session.user.get_profile_by_id(pid)).texture_model == "slim"

@pytest.mark.asyncio
async def test_texture_edge_cases(db_session, user_factory):
    """测试材质模块的边界情况"""
    user = await user_factory()
    
    # 删除不存在的材质
    res = await db_session.texture.delete_from_library(user.id, "non-existent", "skin")
    assert res is False
    
    # 验证不存在材质的所有权
    assert await db_session.texture.verify_ownership(user.id, "none", "skin") is False
    
    # 获取不存在的材质信息
    assert await db_session.texture.get_texture_info(user.id, "none", "skin") is None
