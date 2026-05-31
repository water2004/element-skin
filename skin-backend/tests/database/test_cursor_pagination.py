"""游标分页测试"""

import pytest
from utils.typing import User, PlayerProfile, InviteCode
from utils.pagination import CursorEncoder


@pytest.mark.asyncio
async def test_list_users_cursor_first_page(db_session):
    """测试用户列表游标分页 - 首页"""
    from utils.uuid_utils import generate_random_uuid

    # 创建5个用户
    for i in range(5):
        uid = generate_random_uuid()
        user = User(uid, f"user{i}@test.com", "hash_pwd", False, "en_US", f"User {i}", None)
        await db_session.user.create(user)

    # 获取首页（limit=2）
    result = await db_session.user.list_users_cursor(limit=2)
    
    assert len(result["items"]) == 2
    assert result["has_next"] is True
    assert result["next_key"] is not None
    assert result["page_size"] == 2


@pytest.mark.asyncio
async def test_list_users_cursor_pagination(db_session):
    """测试用户列表游标分页 - 翻页：全量覆盖且无重叠"""
    from utils.uuid_utils import generate_random_uuid

    # 创建8个用户
    user_ids = []
    for i in range(8):
        uid = generate_random_uuid()
        user = User(uid, f"user{i:02d}@test.com", "hash_pwd", False, "en_US", f"User {i}", None)
        await db_session.user.create(user)
        user_ids.append(uid)

    # 逐页跟随游标，收集所有 id
    seen = []
    last_id = None
    for _ in range(20):  # 安全上限
        page = await db_session.user.list_users_cursor(limit=3, last_id=last_id)
        seen.extend(u.id for u in page["items"])
        if not page["has_next"]:
            break
        last_id = page["next_key"]["last_id"]

    # 8 个用户全部出现，无重复
    assert set(seen) == set(user_ids)
    assert len(seen) == 8


@pytest.mark.asyncio
async def test_get_profiles_by_user_cursor(db_session, user_factory):
    """测试用户角色列表游标分页"""
    user = await user_factory()
    
    # 创建5个角色
    for i in range(5):
        profile = PlayerProfile(f"pid_{i}", user.id, f"Player{i}", "default")
        await db_session.user.create_profile(profile)

    # 获取首页
    result = await db_session.user.get_profiles_by_user_cursor(user.id, limit=2)
    assert len(result["items"]) == 2
    assert result["has_next"] is True
    assert result["page_size"] == 2


@pytest.mark.asyncio
async def test_list_invites_cursor(db_session):
    """测试邀请码列表游标分页（复合游标）"""
    from utils.typing import InviteCode
    import time

    # 创建5个邀请码，不同时间戳
    base_time = int(time.time() * 1000)
    codes = []
    for i in range(5):
        code = InviteCode(
            f"CODE_{i}", 
            base_time - i * 1000,  # 递减时间
            total_uses=1
        )
        await db_session.user.create_invite(code)
        codes.append(code)

    # 获取首页
    result = await db_session.user.list_invites_cursor(limit=2)
    assert len(result["items"]) == 2
    assert result["has_next"] is True

    # 验证按时间排序（DESC）
    assert result["items"][0].created_at >= result["items"][1].created_at


@pytest.mark.asyncio
async def test_list_invites_cursor_pagination(db_session):
    """测试邀请码游标分页翻页：全量覆盖且无重叠"""
    from utils.typing import InviteCode
    import time

    # 创建6个邀请码
    base_time = int(time.time() * 1000)
    codes = []
    for i in range(6):
        code = f"CODE_{i:02d}"
        await db_session.user.create_invite(InviteCode(code, base_time - i * 1000, total_uses=1))
        codes.append(code)

    # 逐页跟随游标
    seen = []
    last_created_at = None
    last_code = None
    for _ in range(20):  # 安全上限
        page = await db_session.user.list_invites_cursor(
            limit=2, last_created_at=last_created_at, last_code=last_code
        )
        seen.extend(c.code for c in page["items"])
        if not page["has_next"]:
            break
        cursor_data = page["next_key"]
        last_created_at = cursor_data["last_created_at"]
        last_code = cursor_data["last_code"]

    assert set(seen) == set(codes)
    assert len(seen) == 6


@pytest.mark.asyncio
async def test_get_for_user_cursor(db_session, user_factory):
    """测试用户材质列表游标分页：全量覆盖且无重叠"""
    user = await user_factory()

    # 创建5个材质（注意：同毫秒内 created_at 相同，靠 hash 做次级排序键）
    hashes = []
    for i in range(5):
        hash_val = f"hash_{i}"
        await db_session.texture.add_to_library(
            user.id, hash_val, "skin", note=f"Skin {i}", is_public=False
        )
        hashes.append(hash_val)

    seen = []
    last_created_at = None
    last_hash = None
    for _ in range(20):  # 安全上限
        page = await db_session.texture.get_for_user_cursor(
            user.id, limit=2, last_created_at=last_created_at, last_hash=last_hash
        )
        seen.extend(item["hash"] for item in page["items"])
        if not page["has_next"]:
            break
        cursor_data = page["next_key"]
        last_created_at = cursor_data["last_created_at"]
        last_hash = cursor_data["last_hash"]

    assert set(seen) == set(hashes)
    assert len(seen) == 5


@pytest.mark.asyncio
async def test_get_for_user_cursor_with_type_filter(db_session, user_factory):
    """测试用户材质游标分页 - 带纹理类型过滤"""
    user = await user_factory()
    
    # 创建3个skin和2个cape
    for i in range(3):
        await db_session.texture.add_to_library(
            user.id,
            f"skin_{i}",
            "skin",
            note=f"Skin {i}",
            is_public=False
        )
    
    for i in range(2):
        await db_session.texture.add_to_library(
            user.id,
            f"cape_{i}",
            "cape",
            note=f"Cape {i}",
            is_public=False
        )

    # 只获取skin
    result = await db_session.texture.get_for_user_cursor(
        user.id,
        texture_type="skin",
        limit=2
    )
    assert len(result["items"]) == 2
    assert all(item["type"] == "skin" for item in result["items"])
    assert result["has_next"] is True


@pytest.mark.asyncio
async def test_get_from_library_cursor(db_session, user_factory):
    """测试公开皮肤库游标分页：全量覆盖且无重叠"""
    uploader = await user_factory()

    # 创建5个公开材质
    hashes = []
    for i in range(5):
        hash_val = f"public_hash_{i}"
        await db_session.texture.add_to_library(
            uploader.id, hash_val, "skin", note=f"Public Skin {i}", is_public=True
        )
        hashes.append(hash_val)

    seen = []
    last_created_at = None
    last_skin_hash = None
    for _ in range(20):  # 安全上限
        page = await db_session.texture.get_from_library_cursor(
            limit=2, last_created_at=last_created_at, last_skin_hash=last_skin_hash
        )
        assert all(item["is_public"] is True for item in page["items"])
        seen.extend(item["hash"] for item in page["items"])
        if not page["has_next"]:
            break
        cursor_data = page["next_key"]
        last_created_at = cursor_data["last_created_at"]
        last_skin_hash = cursor_data["last_skin_hash"]

    assert set(seen) == set(hashes)
    assert len(seen) == 5


@pytest.mark.asyncio
async def test_cursor_encoder_decode(db_session):
    """测试游标编码/解码"""
    # 测试简单游标
    data1 = {"last_id": "user-123"}
    cursor1 = CursorEncoder.encode(data1)
    decoded1 = CursorEncoder.decode(cursor1)
    assert decoded1 == data1

    # 测试复合游标
    data2 = {"last_created_at": 1701000000, "last_code": "ABC123"}
    cursor2 = CursorEncoder.encode(data2)
    decoded2 = CursorEncoder.decode(cursor2)
    assert decoded2 == data2

    # 测试无效游标
    invalid_decoded = CursorEncoder.decode("invalid==cursor")
    assert invalid_decoded is None
