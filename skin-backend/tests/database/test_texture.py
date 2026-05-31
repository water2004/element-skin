import pytest
from utils.uuid_utils import generate_random_uuid


@pytest.mark.asyncio
async def test_texture_upload_and_library(db_session, user_factory):
    """测试材质记录写入及皮肤库接口（DB 层纯记录操作）"""
    user = await user_factory()
    tex_hash = "a" * 64

    # 1. add_to_library（DB 层纯记录；图像处理/落盘由 TextureStorage 负责，见 services 测试）
    ok = await db_session.texture.add_to_library(
        user.id, tex_hash, "skin", note="MySkin", is_public=True, model="default"
    )
    assert ok is True

    # 2. Get for user
    user_textures_page = await db_session.texture.get_for_user_cursor(user.id, limit=10)
    assert len(user_textures_page["items"]) == 1
    assert user_textures_page["items"][0]["hash"] == tex_hash

    count = await db_session.texture.count_for_user(user.id)
    assert count == 1

    # 3. Get texture info
    info = await db_session.texture.get_texture_info(user.id, tex_hash, "skin")
    assert info["note"] == "MySkin"
    assert info["is_public"] == 1

    # 4. Verify ownership
    assert await db_session.texture.verify_ownership(user.id, tex_hash, "skin") is True

    # 5. Library actions
    lib_page = await db_session.texture.get_from_library_cursor(only_public=True)
    assert len(lib_page["items"]) == 1
    assert lib_page["items"][0]["hash"] == tex_hash

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
    user2_textures_page = await db_session.texture.get_for_user_cursor(user2.id)
    assert len(user2_textures_page["items"]) == 1
    assert user2_textures_page["items"][0]["is_public"] == 2 # 状态 2 表示非上传者

    # 8. Delete
    await db_session.texture.delete_from_library(user.id, tex_hash, "skin")
    assert len((await db_session.texture.get_for_user_cursor(user.id))["items"]) == 0

@pytest.mark.asyncio
async def test_texture_model_cascade_update(db_session, user_factory):
    """测试更新皮肤模型时，自动同步更新所有使用该皮肤的角色的模型"""
    user = await user_factory()
    tex_hash = "b" * 64

    # 1. 记录皮肤
    await db_session.texture.add_to_library(user.id, tex_hash, "skin", model="default")

    # 2. 创建角色并应用该皮肤
    from utils.typing import PlayerProfile
    pid = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "ModelTester", "default", None, None))
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

@pytest.mark.asyncio
async def test_texture_uploader_deletion_and_readd(db_session, user_factory):
    """测试上传者删除材质同步删除库记录，以及从库中恢复材质的逻辑"""
    user = await user_factory()
    tex_hash = "c" * 64

    # 1. 记录材质（公开）
    await db_session.texture.add_to_library(
        user.id, tex_hash, "skin", note="PublicSkin", is_public=True
    )

    # 验证库中存在
    assert await db_session.texture.count_library(only_public=True) == 1
    
    # 2. 上传者删除材质
    await db_session.texture.delete_from_library(user.id, tex_hash, "skin")
    
    # 验证库中已删除 (修复验证)
    assert await db_session.texture.count_library(only_public=True) == 0
    
    # 3. 模拟遗留数据 (材质在库中，但不在用户衣柜中)
    # 手动插入到 skin_library
    created_at = 1234567890
    async with db_session.get_conn() as conn:
        await conn.execute(
            "INSERT INTO skin_library (skin_hash, texture_type, is_public, uploader, model, name, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
            tex_hash, "skin", 1, user.id, "default", "LegacySkin", created_at
        )
    
    # 4. 上传者重新添加 (验证兼容性修复：is_public=1)
    await db_session.texture.add_to_user_wardrobe(user.id, tex_hash)
    
    user_tex = await db_session.texture.get_texture_info(user.id, tex_hash, "skin")
    assert user_tex["is_public"] == 1
    
    # 5. 其他用户添加 (验证正常逻辑：is_public=2)
    user2 = await user_factory()
    await db_session.texture.add_to_user_wardrobe(user2.id, tex_hash)
    
    user2_tex = await db_session.texture.get_texture_info(user2.id, tex_hash, "skin")
    assert user2_tex["is_public"] == 2


@pytest.mark.asyncio
async def test_list_all_textures_cursor(db_session, user_factory):
    """测试全局管理员分页查询所有材质（list_all_textures_cursor）"""

    user = await user_factory()
    hash_skin_pub = "d" * 64
    hash_skin_priv = "e" * 64
    hash_cape = "f" * 64

    # 1. 记录两个皮肤（一公开一私有）和一个披风（公开）
    await db_session.texture.add_to_library(
        user.id, hash_skin_pub, "skin", note="PublicSkin", is_public=True
    )
    await db_session.texture.add_to_library(
        user.id, hash_skin_priv, "skin", note="PrivateSkin", is_public=False
    )
    await db_session.texture.add_to_library(
        user.id, hash_cape, "cape", note="MyCape", is_public=True
    )

    # 2. list_all_textures_cursor returns all textures (public + private)
    result = await db_session.texture.list_all_textures_cursor(limit=20)
    assert result["page_size"] >= 2  # at least the public ones

    # 3. Test type filter
    only_skins = await db_session.texture.list_all_textures_cursor(limit=20, type_filter="skin")
    assert only_skins["page_size"] >= 1
    for item in only_skins["items"]:
        assert item["type"] == "skin"

    only_capes = await db_session.texture.list_all_textures_cursor(limit=20, type_filter="cape")
    assert only_capes["page_size"] >= 1
    for item in only_capes["items"]:
        assert item["type"] == "cape"

    # 4. Test search (query filter)
    search_result = await db_session.texture.list_all_textures_cursor(limit=20, query="PublicSkin")
    assert search_result["page_size"] >= 1

    # 5. 游标翻页：逐页跟随 next_key，断言全量覆盖且无重叠
    seen = []
    last_created_at = None
    last_skin_hash = None
    for _ in range(10):  # 安全上限，防止死循环
        page = await db_session.texture.list_all_textures_cursor(
            limit=2, last_created_at=last_created_at, last_skin_hash=last_skin_hash
        )
        seen.extend(item["hash"] for item in page["items"])
        if not page["has_next"]:
            break
        last_created_at = page["next_key"]["last_created_at"]
        last_skin_hash = page["next_key"]["last_skin_hash"]

    # 三个材质全部出现，且无重复
    assert set(seen) == {hash_skin_pub, hash_skin_priv, hash_cape}
    assert len(seen) == 3

    # 6. Verify uploader info is returned
    for item in only_skins["items"]:
        if item["hash"] == hash_skin_pub:
            assert item["uploader_user_id"] == user.id
