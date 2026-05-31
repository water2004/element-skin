import pytest
import time
from utils.typing import User, PlayerProfile, Token, Session, InviteCode
from utils.uuid_utils import generate_random_uuid

@pytest.mark.asyncio
async def test_user_management(db_session, user_factory):
    """测试核心用户 CRUD、更新、封禁等逻辑"""
    # Create
    user = await user_factory(email="test@user.com", username="Tester", is_admin=False)
    assert await db_session.user.count() == 1
    
    # Get
    fetched_by_email = await db_session.user.get_by_email("test@user.com")
    assert fetched_by_email.display_name == "Tester"
    
    fetched_by_id = await db_session.user.get_by_id(user.id)
    assert fetched_by_id.email == "test@user.com"
    
    # Update Password/Email/Display Name/Language
    new_pw = "new_hash"
    await db_session.user.update_password(user.id, new_pw)
    assert (await db_session.user.get_by_id(user.id)).password == new_pw
    
    await db_session.user.update_email(user.id, "new@user.com")
    assert (await db_session.user.get_by_id(user.id)).email == "new@user.com"
    
    await db_session.user.update_display_name(user.id, "NewTester")
    assert (await db_session.user.get_by_id(user.id)).display_name == "NewTester"
    
    await db_session.user.update_preferred_language(user.id, "en_US")
    assert (await db_session.user.get_by_id(user.id)).preferred_language == "en_US"
    
    # Display Name Taken Check
    assert await db_session.user.is_display_name_taken("NewTester") is True
    assert await db_session.user.is_display_name_taken("NewTester", exclude_user_id=user.id) is False
    
    # Ban/Unban/IsBanned
    banned_until = int((time.time() + 3600) * 1000)
    await db_session.user.ban(user.id, banned_until)
    assert await db_session.user.is_banned(user.id) is True
    
    await db_session.user.unban(user.id)
    assert await db_session.user.is_banned(user.id) is False
    
    # Toggle Admin
    status = await db_session.user.toggle_admin(user.id)
    assert status == 1
    assert (await db_session.user.get_by_id(user.id)).is_admin is True
    
    # List & Delete
    await user_factory(email="second@test.com")
    users_page = await db_session.user.list_users_cursor(limit=10)
    assert len(users_page["items"]) == 2
    
    await db_session.user.delete(user.id)
    assert await db_session.user.count() == 1

@pytest.mark.asyncio
async def test_profile_management(db_session, user_factory):
    """测试角色(Profile)相关接口"""
    user = await user_factory()
    pid = generate_random_uuid()
    profile = PlayerProfile(pid, user.id, "Player1", "default", None, None)
    
    # Create
    await db_session.user.create_profile(profile)
    assert await db_session.user.count_profiles_by_user(user.id) == 1
    
    # Get (paginated)
    profiles = await db_session.user.get_profiles_by_user(user.id, limit=10)
    assert len(profiles) == 1
    assert profiles[0].name == "Player1"
    
    p2 = await db_session.user.get_profile_by_name("Player1")
    assert p2.id == pid
    
    # Update Skin/Cape/Model/Name
    await db_session.user.update_profile_skin(pid, "skin_hash")
    await db_session.user.update_profile_cape(pid, "cape_hash")
    await db_session.user.update_profile_texture_model(pid, "slim")
    await db_session.user.update_profile_name(pid, "NewName")
    
    updated = await db_session.user.get_profile_by_id(pid)
    assert updated.skin_hash == "skin_hash"
    assert updated.cape_hash == "cape_hash"
    assert updated.texture_model == "slim"
    assert updated.name == "NewName"
    
    # Search & Bulk display name
    await db_session.user.create_profile(PlayerProfile(generate_random_uuid(), user.id, "Player2", "default", None, None))
    results = await db_session.user.search_profiles_by_names(["Player1", "Player2"])
    # The count should be 2 because we created Player1 and Player2.
    # Wait, did we delete it earlier? Yes, `await db_session.user.delete_profile(pid)`
    # So we should create a new one to be sure.
    # Ah, I see: `assert await db_session.user.count_profiles_by_user(user.id) == 1`
    # That was the count after deleting Player1.
    # So currently we have 1 profile (Player1 was deleted).
    # Creating Player2 makes it 2.
    assert await db_session.user.count_profiles_by_user(user.id) == 2
    
    results = await db_session.user.search_profiles_by_names(["Player1", "Player2"])
    # Player1 was deleted. So only Player2 exists.
    assert len(results) == 1
    
    names = await db_session.user.get_display_names_by_ids([user.id])
    assert names[user.id] == user.display_name
    
    # Ownership
    assert await db_session.user.verify_profile_ownership(user.id, pid) is True
    
    # Delete
    await db_session.user.delete_profile(pid)
    assert await db_session.user.count_profiles_by_user(user.id) == 1

@pytest.mark.asyncio
async def test_token_and_session(db_session, user_factory):
    """测试 Token 和 Session 接口，包含过期清理逻辑"""
    user = await user_factory()
    
    # 1. Tokens 基础操作
    token_str = "acc_token"
    token = Token(token_str, "cli_token", user.id, None, int(time.time() * 1000))
    await db_session.user.add_token(token)
    assert (await db_session.user.get_token(token_str)) is not None
    
    # 2. 删除用户所有令牌
    await db_session.user.delete_tokens_by_user(user.id)
    assert (await db_session.user.get_token(token_str)) is None
    
    # 3. 过期令牌清理 (delete_expired_tokens)
    old_token_str = "old_token"
    old_ts = int((time.time() - 10000) * 1000) # 很久以前
    await db_session.user.add_token(Token(old_token_str, "cli", user.id, None, old_ts))
    
    cutoff = int((time.time() - 5000) * 1000)
    await db_session.user.delete_expired_tokens(user.id, cutoff)
    assert (await db_session.user.get_token(old_token_str)) is None
    
    # 4. 冗余令牌清理 (delete_surplus_tokens)
    for i in range(10):
        await db_session.user.add_token(Token(f"t{i}", "cli", user.id, None, int(time.time() * 1000) + i))
    
    await db_session.user.delete_surplus_tokens(user.id, keep=5)
    # 应该只剩下最后 5 个
    assert (await db_session.user.get_token("t9")) is not None
    assert (await db_session.user.get_token("t0")) is None
    
    # 5. Sessions
    session = Session("server_id", "acc_token", "127.0.0.1", int(time.time() * 1000))
    await db_session.user.add_session(session)
    assert (await db_session.user.get_session("server_id")) is not None
    
    await db_session.user.delete_session("server_id")
    assert (await db_session.user.get_session("server_id")) is None

@pytest.mark.asyncio
async def test_list_and_search_users_cursor_field_mapping(db_session, user_factory):
    """游标列表/搜索的 User 字段映射必须精确：每字段取独立值，错位即变红"""
    user = await user_factory(email="mapper@test.com", username="MapName", is_admin=True)
    await db_session.user.update_preferred_language(user.id, "en_US")
    banned_until = int((time.time() + 3600) * 1000)
    await db_session.user.ban(user.id, banned_until)

    def _find(items):
        return next(u for u in items if u.id == user.id)

    listed = _find((await db_session.user.list_users_cursor(limit=50))["items"])
    assert listed.email == "mapper@test.com"
    assert listed.display_name == "MapName"
    assert listed.is_admin is True
    assert listed.preferred_language == "en_US"
    assert listed.banned_until == banned_until
    assert listed.password == ""

    searched = _find((await db_session.user.search_users_cursor(query="MapName", limit=50))["items"])
    assert searched.email == "mapper@test.com"
    assert searched.display_name == "MapName"
    assert searched.is_admin is True
    assert searched.preferred_language == "en_US"
    assert searched.banned_until == banned_until
    assert searched.password == ""


@pytest.mark.asyncio
async def test_user_edge_cases(db_session):
    """测试用户模块的边界情况"""
    # 查询不存在的用户
    assert await db_session.user.get_by_id("non-existent") is None
    assert await db_session.user.get_by_email("none@none.com") is None
    
    # 解封未被封禁的用户
    await db_session.user.unban("non-existent") # 不应报错
    
    # 切换不存在用户的管理员状态
    res = await db_session.user.toggle_admin("non-existent")
    assert res == -1

@pytest.mark.asyncio
async def test_invite_management(db_session):
    """测试邀请码逻辑"""
    code_str = "INVITE_CODE"
    invite = InviteCode(code_str, int(time.time() * 1000), used_by=None, total_uses=5, used_count=0, note="test")
    
    await db_session.user.create_invite(invite)
    
    fetched = await db_session.user.get_invite(code_str)
    assert fetched.total_uses == 5
    
    await db_session.user.use_invite(code_str, "used@test.com")
    updated = await db_session.user.get_invite(code_str)
    assert updated.used_count == 1
    assert updated.used_by == "used@test.com"
    
    invites_page = await db_session.user.list_invites_cursor()
    assert len(invites_page["items"]) == 1
    
    await db_session.user.delete_invite(code_str)
    assert (await db_session.user.get_invite(code_str)) is None


@pytest.mark.asyncio
async def test_list_all_profiles_cursor(db_session, user_factory):
    """测试管理端：游标分页列出所有角色（含搜索）"""
    user1 = await user_factory(email="user1@test.com")
    user2 = await user_factory(email="user2@test.com")

    # Create 2 profiles for user1, 1 for user2
    pid1 = generate_random_uuid()
    pid2 = generate_random_uuid()
    pid3 = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid1, user1.id, "Player1", "default", None, None))
    await db_session.user.create_profile(PlayerProfile(pid2, user1.id, "Player2", "slim", None, None))
    await db_session.user.create_profile(PlayerProfile(pid3, user2.id, "OtherPlayer", "default", None, None))

    # 1. List all with ample limit
    page = await db_session.user.list_all_profiles_cursor(limit=10)
    assert len(page["items"]) == 3
    assert page["has_next"] is False
    assert page["next_key"] is None

    # 2. Pagination: limit=1 (to get proper cursor progression)
    page1 = await db_session.user.list_all_profiles_cursor(limit=1)
    assert len(page1["items"]) == 1
    assert page1["has_next"] is True
    assert page1["next_key"] is not None

    # Follow raw next_key into next page
    cursor_data = page1["next_key"]
    page2 = await db_session.user.list_all_profiles_cursor(limit=10, after_id=cursor_data["last_id"])
    assert len(page2["items"]) >= 1
    assert page2["has_next"] is False
    # 全量覆盖且无重叠：两页合计 3 个角色且不重复
    seen_ids = [p["id"] for p in page1["items"]] + [p["id"] for p in page2["items"]]
    assert len(seen_ids) == 3
    assert len(set(seen_ids)) == 3

    # 3. Search by profile name
    search_page = await db_session.user.list_all_profiles_cursor(limit=10, query="Player1")
    assert len(search_page["items"]) == 1
    assert search_page["items"][0]["name"] == "Player1"

    # 4. Search by owner email
    search_page = await db_session.user.list_all_profiles_cursor(limit=10, query="user1@")
    assert len(search_page["items"]) == 2
    for item in search_page["items"]:
        assert item["owner_email"] == "user1@test.com"
