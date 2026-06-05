import pytest
import asyncio
import string
import time
from unittest.mock import AsyncMock, patch
from fastapi import HTTPException
from backends.site_backend import SiteBackend, is_valid_email
from routes_reference import texture_storage
from utils.password_utils import verify_password, hash_password
from utils.typing import PlayerProfile, InviteCode, User
from utils.uuid_utils import get_offline_uuid, generate_random_uuid

@pytest.mark.asyncio
async def test_site_auth_flow(db_session, test_config):
    """测试完整的注册、登录及密码修改流程"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    
    # 1. 注册 (首个用户应为管理员)
    email = "admin@example.com"
    password = "StrongPassword123!"
    username = "SuperAdmin"
    
    uid = await backend.register(email, password, username)
    assert uid is not None
    
    user_info = await backend.get_user_info(uid)
    assert user_info["is_admin"] is True
    assert user_info["display_name"] == username
    assert "profile_count" in user_info
    assert "profiles" not in user_info
    
    # 2. 登录
    login_res = await backend.login(email, password)
    assert login_res["user_id"] == uid
    assert "access_token" in login_res
    assert "refresh_token" in login_res
    assert login_res["is_admin"] is True
    
    # 3. 修改密码
    new_password = "NewStrongPassword456!"
    await backend.change_password(uid, password, new_password)
    
    # 验证新密码是否生效
    user_row = await db_session.user.get_by_id(uid)
    assert verify_password(new_password, user_row.password) is True
    
    # 4. 验证登录失败 (使用旧密码)
    with pytest.raises(HTTPException) as exc:
        await backend.login(email, password)
    assert exc.value.status_code == 401

@pytest.mark.asyncio
async def test_verification_code_flow(db_session, test_config):
    """测试邮箱验证码发送与校验流程"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    email = "verify@test.com"
    
    # 启用邮件验证
    await db_session.setting.set("email_verify_enabled", "true")
    
    # 使用 Mock 模拟邮件发送
    with patch.object(backend.email_sender, 'send_verification_code', new_callable=AsyncMock) as mock_send:
        mock_send.return_value = True
        
        # 发送注册验证码
        res = await backend.send_verification_code(email, "register")
        assert res["ok"] is True
        mock_send.assert_called_once()
        
        # 获取存储的验证码并验证
        record = await db_session.verification.get_code(email, "register")
        code = record[0]
        
        assert await backend.verify_code(email, code, "register") is True
        assert await backend.verify_code(email, "WRONG", "register") is False

@pytest.mark.asyncio
async def test_profile_and_texture_application(db_session, test_config, user_factory):
    """测试角色创建及材质应用逻辑"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()
    
    # 1. 创建角色
    profile_data = await backend.create_profile(user.id, "MyPlayer", "default")
    pid = profile_data["id"]
    assert profile_data["name"] == "MyPlayer"
    
    # 2. 准备材质 (模拟已在库中)
    tex_hash = "some_skin_hash"
    await db_session.texture.add_to_library(user.id, tex_hash, "skin", is_public=False)
    
    # 3. 应用材质到角色
    await backend.apply_texture_to_profile(user.id, pid, tex_hash, "skin")
    
    # 验证
    updated_p = await db_session.user.get_profile_by_id(pid)
    assert updated_p.skin_hash == tex_hash
    
    # 4. 清除材质
    await backend.clear_profile_texture(user.id, pid, "skin")
    cleared_p = await db_session.user.get_profile_by_id(pid)
    assert cleared_p.skin_hash is None

@pytest.mark.asyncio
async def test_registration_restrictions(db_session, test_config, user_factory):
    """测试注册限制逻辑：邀请码、注册开关、用户名重复"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    
    # 1. 禁用注册
    await db_session.setting.set("allow_register", "false")
    with pytest.raises(HTTPException) as exc:
        await backend.register("t@t.com", "p", "u")
    assert exc.value.status_code == 403
    
    await db_session.setting.set("allow_register", "true")
    
    # 2. 强制邀请码
    await db_session.setting.set("require_invite", "true")
    with pytest.raises(HTTPException) as exc:
        await backend.register("t@t.com", "p", "u")
    assert "invite code required" in exc.value.detail
    
    # 使用无效邀请码
    with pytest.raises(HTTPException) as exc:
        await backend.register("t@t.com", "p", "u", invite_code="INVALID")
    assert "invalid invite code" in exc.value.detail
    
    # 使用有效邀请码
    from utils.typing import InviteCode
    import time
    await db_session.user.create_invite(InviteCode("VALID_CODE", int(time.time()*1000), total_uses=1))
    uid = await backend.register("t@t.com", "Pass123!", "UniqueUser", invite_code="VALID_CODE")
    assert uid is not None
    
    # 3. 用户名占用
    with pytest.raises(HTTPException) as exc:
        await backend.register("t2@t.com", "p", "UniqueUser")
    assert "Username already exists" in exc.value.detail


@pytest.mark.asyncio
async def test_create_profile_uses_offline_uuid_when_enabled(db_session, test_config, user_factory):
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()
    await db_session.setting.set("profile_uuid_mode", "offline")

    created = await backend.create_profile(user.id, "OfflinePlayerA", "default")
    assert created["id"] == get_offline_uuid("OfflinePlayerA")


@pytest.mark.asyncio
async def test_create_profile_rejects_uuid_conflict(db_session, test_config, user_factory):
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()

    conflict_id = "abcdabcdabcdabcdabcdabcdabcdabcd"
    await db_session.user.create_profile(PlayerProfile(conflict_id, user.id, "TakenRole", "default"))

    with patch("backends.site_backend.generate_random_uuid", return_value=conflict_id):
        with pytest.raises(HTTPException) as exc:
            await backend.create_profile(user.id, "BrandNewRole", "default")

    assert exc.value.status_code == 400
    assert exc.value.detail == "角色 UUID 冲突，无法新建角色"


# ========== Phase 4: orchestration methods moved from router ==========


@pytest.mark.asyncio
async def test_get_public_skin_library_aggregates_uploader_name(db_session, test_config, user_factory):
    """皮肤库聚合：每条 item 带正确的 uploader_name，且翻页跟随游标无重叠"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    uploader = await user_factory(username="LibOwner")
    hashes = [chr(ord("a") + i) * 64 for i in range(3)]
    for i, h in enumerate(hashes):
        await db_session.texture.add_to_library(uploader.id, h, "skin", note=f"L{i}", is_public=True)

    seen = []
    cursor = None
    for _ in range(10):
        page = await backend.get_public_skin_library(cursor, 2, None)
        for item in page["items"]:
            assert item["uploader_name"] == "LibOwner"
        seen.extend(item["hash"] for item in page["items"])
        if not page["has_next"]:
            break
        cursor = page["next_cursor"]
        assert isinstance(cursor, str) and cursor

    assert set(hashes).issubset(set(seen))
    assert len(seen) == len(set(seen))


@pytest.mark.asyncio
async def test_get_public_skin_library_disabled(db_session, test_config):
    backend = SiteBackend(db_session, test_config, texture_storage)
    await db_session.setting.set("enable_skin_library", "false")
    with pytest.raises(HTTPException) as exc:
        await backend.get_public_skin_library(None, 20, None)
    assert exc.value.status_code == 403


@pytest.mark.asyncio
async def test_get_public_skin_library_invalid_cursor(db_session, test_config):
    backend = SiteBackend(db_session, test_config, texture_storage)
    with pytest.raises(HTTPException) as exc:
        await backend.get_public_skin_library("garbage!!", 20, None)
    assert exc.value.status_code == 400


@pytest.mark.asyncio
async def test_get_public_skin_library_search_matches_name_hash_uploader(db_session, test_config, user_factory):
    """搜索功能：分别可按名称、hash 子串、上传者 display_name 命中，并排除非匹配项。

    这是一个三向区分测试：每条材质的 name / hash / 上传者名互不重叠，
    保证当某一列的过滤被遗漏时，对应查询会返回 0 条而不是 1 条；当 q
    完全被忽略时则会返回 3 条。
    """
    backend = SiteBackend(db_session, test_config, texture_storage)
    alice = await user_factory(username="AliceWonder")
    bob = await user_factory(username="BobBuilder")
    charlie = await user_factory(username="CharlieBrown")

    hash_a = "a" * 64  # alice / MagicSword
    hash_b = "b" * 64  # bob / DragonShield
    hash_c = "c" * 64  # charlie / HolyArmor

    await db_session.texture.add_to_library(alice.id, hash_a, "skin", note="MagicSword", is_public=True)
    await db_session.texture.add_to_library(bob.id, hash_b, "skin", note="DragonShield", is_public=True)
    await db_session.texture.add_to_library(charlie.id, hash_c, "skin", note="HolyArmor", is_public=True)

    # 1. 按名称匹配：仅 A
    page = await backend.get_public_skin_library(None, 20, None, query="MagicSword")
    assert [it["hash"] for it in page["items"]] == [hash_a]
    assert page["items"][0]["name"] == "MagicSword"

    # 2. 按 hash 子串匹配：仅 B（"bbb" 不会匹配名字或任一上传者名）
    page = await backend.get_public_skin_library(None, 20, None, query="bbb")
    assert [it["hash"] for it in page["items"]] == [hash_b]

    # 3. 按上传者 display_name 匹配：仅 C
    page = await backend.get_public_skin_library(None, 20, None, query="CharlieBrown")
    assert [it["hash"] for it in page["items"]] == [hash_c]
    assert page["items"][0]["uploader_name"] == "CharlieBrown"

    # 4. 大小写不敏感（ILIKE）
    page = await backend.get_public_skin_library(None, 20, None, query="magicsword")
    assert [it["hash"] for it in page["items"]] == [hash_a]

    # 5. 不命中：返回空
    page = await backend.get_public_skin_library(None, 20, None, query="ZZZ_no_such_token")
    assert page["items"] == []
    assert page["has_next"] is False

    # 6. 不带 query：三条都在
    page = await backend.get_public_skin_library(None, 20, None)
    returned = {it["hash"] for it in page["items"]}
    assert {hash_a, hash_b, hash_c}.issubset(returned)


@pytest.mark.asyncio
async def test_get_public_skin_library_search_combined_with_type_filter(db_session, test_config, user_factory):
    """搜索 + texture_type 过滤可联合使用：同名 cape 不会被 skin 类型搜索命中。"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory(username="DualUploader")
    skin_hash = "1" * 64
    cape_hash = "2" * 64
    # 故意同名，确保差异只在 texture_type
    await db_session.texture.add_to_library(user.id, skin_hash, "skin", note="SharedName", is_public=True)
    await db_session.texture.add_to_library(user.id, cape_hash, "cape", note="SharedName", is_public=True)

    page = await backend.get_public_skin_library(None, 20, "skin", query="SharedName")
    assert [it["hash"] for it in page["items"]] == [skin_hash]

    page = await backend.get_public_skin_library(None, 20, "cape", query="SharedName")
    assert [it["hash"] for it in page["items"]] == [cape_hash]


@pytest.mark.asyncio
async def test_get_public_skin_library_search_excludes_private(db_session, test_config, user_factory):
    """搜索结果仍受 only_public=True 约束：私有材质即便命中也不返回。"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory(username="PrivOwner")
    pub_hash = "e" * 64
    priv_hash = "f" * 64
    await db_session.texture.add_to_library(user.id, pub_hash, "skin", note="UniquePublicTex", is_public=True)
    await db_session.texture.add_to_library(user.id, priv_hash, "skin", note="UniquePrivateTex", is_public=False)

    page = await backend.get_public_skin_library(None, 20, None, query="UniquePublicTex")
    assert [it["hash"] for it in page["items"]] == [pub_hash]

    page = await backend.get_public_skin_library(None, 20, None, query="UniquePrivateTex")
    assert page["items"] == []


@pytest.mark.asyncio
async def test_update_my_texture_field_branches(db_session, test_config, user_factory):
    """note/model/is_public 三个分支独立生效，返回最新 info"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()
    h = "f" * 64
    await db_session.texture.add_to_library(user.id, h, "skin", note="orig", is_public=True, model="default")

    res = await backend.update_my_texture(user.id, h, "skin", {"note": "renamed"})
    assert res["ok"] is True
    assert res["note"] == "renamed"

    res = await backend.update_my_texture(user.id, h, "skin", {"model": "slim"})
    assert res["model"] == "slim"

    res = await backend.update_my_texture(user.id, h, "skin", {"is_public": False})
    assert res["is_public"] == 0


@pytest.mark.asyncio
async def test_upload_and_apply_texture_three_steps(db_session, test_config, user_factory):
    """上传→应用→更新模型 三步串联，皮肤 hash 与 slim 模型落到角色上"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()
    profile = await backend.create_profile(user.id, "ApplyTarget", "default")
    pid = profile["id"]

    with patch.object(texture_storage, "process_and_save", return_value="applied_hash"):
        result = await backend.upload_and_apply_texture(
            user.id, pid, b"fake-png-bytes", "skin", model="slim", is_public=False
        )
    assert result["ok"] is True

    updated = await db_session.user.get_profile_by_id(pid)
    assert updated.skin_hash == "applied_hash"
    assert updated.texture_model == "slim"


@pytest.mark.asyncio
async def test_add_texture_to_wardrobe_missing(db_session, test_config, user_factory):
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()
    with pytest.raises(HTTPException) as exc:
        await backend.add_texture_to_wardrobe(user.id, "nonexistent_hash")
    assert exc.value.status_code == 404


@pytest.mark.asyncio
async def test_get_my_texture_detail_missing(db_session, test_config, user_factory):
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()
    with pytest.raises(HTTPException) as exc:
        await backend.get_my_texture_detail(user.id, "nope", "skin")
    assert exc.value.status_code == 404


# ========== Phase 3: 邮箱校验 / 随机码 / 注册原子化 / 邀请超额 ==========


def test_is_valid_email_rejects_crlf_and_malformed():
    """阶段3：邮箱校验用 fullmatch + 拒绝 CRLF，挡住头注入与畸形地址。"""
    from backends.site_backend import is_valid_email

    # 合法
    assert is_valid_email("a@b.com") is True
    assert is_valid_email("user.name+tag@example.co.uk") is True

    # 头注入载荷（CRLF）
    assert is_valid_email("a@x.com\r\nBcc: x@y.com") is False
    assert is_valid_email("a@x.com\n") is False
    assert is_valid_email("a@x.com\r") is False

    # 畸形
    assert is_valid_email("a@@b") is False
    assert is_valid_email("a@b") is False  # 无顶级域
    assert is_valid_email("plainstring") is False
    assert is_valid_email("") is False


@pytest.mark.asyncio
async def test_register_rejects_invalid_email(db_session, test_config):
    """阶段3：注册入口拒绝畸形/CRLF 邮箱，返回 400。"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    for bad in ("a@b", "a@x.com\r\nBcc: x@y.com", "notanemail"):
        with pytest.raises(HTTPException) as exc:
            await backend.register(bad, "Password123!", "SomeUser")
        assert exc.value.status_code == 400
        assert "Invalid email format" in exc.value.detail


@pytest.mark.asyncio
async def test_verification_code_uses_secure_charset(db_session, test_config):
    """阶段3：验证码长度为 8、字符集为大写字母+数字（密码学安全源 secrets）。"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    await db_session.setting.set("email_verify_enabled", "true")

    import string as _string
    allowed = set(_string.ascii_uppercase + _string.digits)

    with patch.object(backend.email_sender, "send_verification_code", new_callable=AsyncMock) as mock_send:
        mock_send.return_value = True
        await backend.send_verification_code("codeuser@test.com", "register")

    record = await db_session.verification.get_code("codeuser@test.com", "register")
    code = record[0]
    assert len(code) == 8
    assert set(code).issubset(allowed)


@pytest.mark.asyncio
async def test_register_atomic_no_orphan_user_on_profile_conflict(db_session, test_config):
    """阶段3：注册建号过程中角色名冲突 → 整笔回滚，无孤儿 user，邮箱仍可注册。"""
    backend = SiteBackend(db_session, test_config, texture_storage)

    email = "atomic@test.com"

    # 让 create_user_with_profile 内部的 profile 插入冲突：预占该用户将生成的角色名。
    # base_name 取邮箱 @ 前缀清洗后的串；这里直接 patch 生成的角色名为一个已占用名。
    taken_name = "TakenProfileName"
    owner = User(generate_random_uuid(), "owner@test.com", hash_password("x"), False, "zh_CN", "Owner")
    await db_session.user.create(owner)
    await db_session.user.create_profile(PlayerProfile(generate_random_uuid(), owner.id, taken_name, "default"))

    with patch("backends.site_backend.generate_unique_profile_name", new_callable=AsyncMock) as mock_gen:
        mock_gen.return_value = taken_name
        with pytest.raises(HTTPException):
            await backend.register(email, "Password123!", "AtomicUser")

    # 无孤儿 user：该邮箱不应残留
    assert await db_session.user.get_by_email(email) is None


@pytest.mark.asyncio
async def test_register_invite_overuse_blocked(db_session, test_config):
    """阶段3：total_uses=1 的邀请码，第二次注册被拒（条件 UPDATE + 行数判定兜底）。"""
    from utils.typing import InviteCode
    import time as _time

    backend = SiteBackend(db_session, test_config, texture_storage)
    await db_session.setting.set("require_invite", "true")
    await db_session.user.create_invite(
        InviteCode("ONCE_ONLY", int(_time.time() * 1000), total_uses=1)
    )

    uid1 = await backend.register("first@test.com", "Password123!", "FirstUser", invite_code="ONCE_ONLY")
    assert uid1 is not None

    with pytest.raises(HTTPException) as exc:
        await backend.register("second@test.com", "Password123!", "SecondUser", invite_code="ONCE_ONLY")
    assert exc.value.status_code == 400
    # 第二个用户不应被创建
    assert await db_session.user.get_by_email("second@test.com") is None


@pytest.mark.asyncio
async def test_user_self_delete_profile_cascades_tokens(db_session, test_config, user_factory):
    """阶段4：用户自助删 profile 时，其 Yggdrasil 游戏 token 一并消失（统一走级联）。"""
    from utils.typing import Token

    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()
    pid = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "SelfDel", "default", None, None))
    await db_session.user.add_token(Token("self-acc-tok", "self-cli-tok", user.id, pid, int(time.time() * 1000)))
    assert await db_session.user.get_token("self-acc-tok") is not None

    await backend.delete_profile(user.id, pid)
    assert await db_session.user.get_profile_by_id(pid) is None
    assert await db_session.user.get_token("self-acc-tok") is None


@pytest.mark.asyncio
async def test_user_self_delete_profile_not_owner(db_session, test_config, user_factory):
    """阶段4：非属主删 profile 仍被拒（403），级联改动不放松鉴权。"""
    backend = SiteBackend(db_session, test_config, texture_storage)
    owner = await user_factory()
    other = await user_factory()
    pid = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, owner.id, "OwnedProf", "default", None, None))

    with pytest.raises(HTTPException) as exc:
        await backend.delete_profile(other.id, pid)
    assert exc.value.status_code == 403
    # 角色未被删除
    assert await db_session.user.get_profile_by_id(pid) is not None
