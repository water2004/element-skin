import pytest
from unittest.mock import AsyncMock, patch
from fastapi import HTTPException
from backends.profile_import_backend import ProfileImportBackend
from routes_reference import texture_storage
from utils.typing import PlayerProfile

@pytest.mark.asyncio
async def test_import_ygg_profile_success(db_session, test_config):
    backend = ProfileImportBackend(db_session, texture_storage)
    user_id = "test_user_id"
    api_url = "http://example.com/api/yggdrasil"
    profile_id = "test_profile_id"
    profile_name = "TestPlayer"

    # Mock YggdrasilClient and download_texture
    with patch("backends.profile_import_backend.YggdrasilClient") as MockClient, \
         patch("backends.profile_import_backend.download_texture", new_callable=AsyncMock) as mock_download:

        mock_instance = MockClient.return_value
        mock_instance.get_profile_with_textures = AsyncMock(return_value={
            "id": profile_id,
            "name": profile_name,
            "skins": [{"url": "http://skin.url", "variant": "classic"}],
            "capes": []
        })
        mock_download.return_value = b"skin_bytes"

        # Create a dummy user
        await db_session.execute(
            "INSERT INTO users (id, email, password, is_admin, display_name) VALUES ($1, $2, $3, $4, $5)",
            user_id, "test@example.com", "hash", False, "TestUser"
        )
        result = await backend.import_ygg_profile(user_id, api_url, profile_id, profile_name)

        assert result["id"] == profile_id
        assert result["name"] == profile_name

        # Verify profile created in DB
        profile = await db_session.user.get_profile_by_id(profile_id)
        assert profile is not None
        assert profile.name == profile_name
        assert profile.user_id == user_id

@pytest.mark.asyncio
async def test_import_ygg_profile_uuid_conflict(db_session, test_config):
    backend = ProfileImportBackend(db_session, texture_storage)
    user_id = "test_user_id_2"
    api_url = "http://example.com/api/yggdrasil"
    profile_id = "conflict_profile_id"
    profile_name = "ConflictPlayer"

    await db_session.execute(
        "INSERT INTO users (id, email, password, is_admin, display_name) VALUES ($1, $2, $3, $4, $5)",
        user_id, "test2@example.com", "hash", False, "TestUser2"
    )

    # Pre-create a profile with the same ID
    await db_session.user.create_profile(
        PlayerProfile(profile_id, user_id, "ExistingPlayer", "default")
    )

    with patch("backends.profile_import_backend.YggdrasilClient") as MockClient:
        mock_instance = MockClient.return_value
        mock_instance.get_profile_with_textures = AsyncMock(return_value={
            "id": profile_id,
            "name": profile_name,
            "skins": [],
            "capes": []
        })

        with pytest.raises(HTTPException) as exc:
            await backend.import_ygg_profile(user_id, api_url, profile_id, profile_name)

        assert exc.value.status_code == 400
        assert "UUID" in exc.value.detail

@pytest.mark.asyncio
async def test_import_ygg_profile_name_conflict(db_session, test_config):
    backend = ProfileImportBackend(db_session, texture_storage)
    user_id = "test_user_id_3"
    api_url = "http://example.com/api/yggdrasil"
    profile_id = "new_profile_id"
    profile_name = "TakenName"

    await db_session.execute(
        "INSERT INTO users (id, email, password, is_admin, display_name) VALUES ($1, $2, $3, $4, $5)",
        user_id, "test3@example.com", "hash", False, "TestUser3"
    )

    # Pre-create a profile with the same Name but different ID
    await db_session.user.create_profile(
        PlayerProfile("other_id", user_id, profile_name, "default")
    )

    with patch("backends.profile_import_backend.YggdrasilClient") as MockClient, \
         patch("backends.profile_import_backend.download_texture", new_callable=AsyncMock):

        mock_instance = MockClient.return_value
        mock_instance.get_profile_with_textures = AsyncMock(return_value={
            "id": profile_id,
            "name": profile_name,
            "skins": [],
            "capes": []
        })

        result = await backend.import_ygg_profile(user_id, api_url, profile_id, profile_name)

        assert result["id"] == profile_id
        assert result["name"] == f"{profile_name}_1"

        profile = await db_session.user.get_profile_by_id(profile_id)
        assert profile.name == f"{profile_name}_1"


@pytest.mark.asyncio
async def test_import_ygg_profiles_batch_success_and_partial_failure(db_session, test_config):
    backend = ProfileImportBackend(db_session, texture_storage)
    user_id = "batch_user_id"
    api_url = "http://example.com/api/yggdrasil"

    await db_session.execute(
        "INSERT INTO users (id, email, password, is_admin, display_name) VALUES ($1, $2, $3, $4, $5)",
        user_id, "batch@example.com", "hash", False, "BatchUser"
    )

    profiles = [
        {"profile_id": "batch_profile_1", "profile_name": "BatchPlayer1"},
        {"profile_id": "batch_profile_2", "profile_name": "BatchPlayer2"},
        {"profile_id": "", "profile_name": "BrokenProfile"},
    ]

    with patch("backends.profile_import_backend.YggdrasilClient") as MockClient, \
         patch("backends.profile_import_backend.download_texture", new_callable=AsyncMock) as mock_download:
        mock_instance = MockClient.return_value

        async def get_profile_with_textures(profile_id):
            return {
                "id": profile_id,
                "name": profile_id.replace("batch_", ""),
                "skins": [{"url": f"http://skin.url/{profile_id}", "variant": "classic"}],
                "capes": [],
            }

        mock_instance.get_profile_with_textures = AsyncMock(side_effect=get_profile_with_textures)
        mock_download.return_value = b"skin_bytes"

        result = await backend.import_ygg_profiles(user_id, api_url, profiles)

    assert result["success_count"] == 2
    assert result["failure_count"] == 1
    assert len(result["items"]) == 2
    assert len(result["failed"]) == 1
    assert result["failed"][0]["detail"] == "profile_id and profile_name are required"

    assert await db_session.user.get_profile_by_id("batch_profile_1") is not None
    assert await db_session.user.get_profile_by_id("batch_profile_2") is not None


@pytest.mark.asyncio
async def test_get_ygg_profiles_internal_error_is_converged(db_session, test_config):
    """远端连接/内部异常不应回显底层文本（含 URL/连接细节），返回通用文案。"""
    backend = ProfileImportBackend(db_session, texture_storage)
    secret = "http://secret-internal-host:9999/leak?token=abc123"

    with patch("backends.profile_import_backend.YggdrasilClient") as MockClient:
        mock_instance = MockClient.return_value
        mock_instance.authenticate = AsyncMock(
            side_effect=ConnectionError(f"connection refused to {secret}")
        )

        with pytest.raises(HTTPException) as exc:
            await backend.get_ygg_profiles("http://api.example.com", "u", "p")

    assert exc.value.status_code == 400
    # 通用文案，且不泄露底层异常中的敏感串
    assert secret not in exc.value.detail
    assert "secret-internal-host" not in exc.value.detail
    assert "token=abc123" not in exc.value.detail


@pytest.mark.asyncio
async def test_import_ygg_profile_internal_error_is_converged(db_session, test_config):
    """单个导入遇到非业务异常时收敛为通用文案，不泄露底层错误。"""
    backend = ProfileImportBackend(db_session, texture_storage)
    user_id = "leak_user_id"
    secret = "/internal/secret/path raised parse failure"

    await db_session.execute(
        "INSERT INTO users (id, email, password, is_admin, display_name) VALUES ($1, $2, $3, $4, $5)",
        user_id, "leak@example.com", "hash", False, "LeakUser"
    )

    with patch("backends.profile_import_backend.YggdrasilClient") as MockClient:
        mock_instance = MockClient.return_value
        # 非 HTTPException 的内部异常
        mock_instance.get_profile_with_textures = AsyncMock(
            side_effect=ValueError(secret)
        )

        with pytest.raises(HTTPException) as exc:
            await backend.import_ygg_profile(
                user_id, "http://api.example.com", "leak_pid", "LeakPlayer"
            )

    assert exc.value.status_code == 400
    assert secret not in exc.value.detail
    assert "secret" not in exc.value.detail


@pytest.mark.asyncio
async def test_import_ygg_profile_business_error_passes_through(db_session, test_config):
    """业务错误（UUID 冲突）仍向用户返回可读提示，不被收敛抹掉。"""
    backend = ProfileImportBackend(db_session, texture_storage)
    user_id = "biz_user_id"
    profile_id = "biz_conflict_pid"

    await db_session.execute(
        "INSERT INTO users (id, email, password, is_admin, display_name) VALUES ($1, $2, $3, $4, $5)",
        user_id, "biz@example.com", "hash", False, "BizUser"
    )
    await db_session.user.create_profile(
        PlayerProfile(profile_id, user_id, "BizExisting", "default")
    )

    with patch("backends.profile_import_backend.YggdrasilClient") as MockClient:
        mock_instance = MockClient.return_value
        mock_instance.get_profile_with_textures = AsyncMock(return_value={
            "id": profile_id, "name": "BizPlayer", "skins": [], "capes": []
        })

        with pytest.raises(HTTPException) as exc:
            await backend.import_ygg_profile(
                user_id, "http://api.example.com", profile_id, "BizPlayer"
            )

    # 业务提示原样保留
    assert exc.value.status_code == 400
    assert "UUID" in exc.value.detail


@pytest.mark.asyncio
async def test_import_ygg_profiles_batch_converges_internal_keeps_business(db_session, test_config):
    """批量导入：内部异常项收敛为通用文案；业务错误项保留可读提示。"""
    backend = ProfileImportBackend(db_session, texture_storage)
    user_id = "batch_leak_user"
    secret = "http://leaky-host:1234/secret?token=zzz"

    await db_session.execute(
        "INSERT INTO users (id, email, password, is_admin, display_name) VALUES ($1, $2, $3, $4, $5)",
        user_id, "batchleak@example.com", "hash", False, "BatchLeakUser"
    )
    # 预置一个 UUID 冲突项（业务错误）
    await db_session.user.create_profile(
        PlayerProfile("batch_biz_pid", user_id, "BatchBizExisting", "default")
    )

    profiles = [
        {"profile_id": "batch_internal_pid", "profile_name": "Internal"},  # 触发内部异常
        {"profile_id": "batch_biz_pid", "profile_name": "Biz"},            # 触发业务错误（UUID 冲突）
    ]

    with patch("backends.profile_import_backend.YggdrasilClient") as MockClient:
        mock_instance = MockClient.return_value

        async def get_profile(pid):
            if pid == "batch_internal_pid":
                raise ConnectionError(f"connect fail {secret}")
            return {"id": pid, "name": "Biz", "skins": [], "capes": []}

        mock_instance.get_profile_with_textures = AsyncMock(side_effect=get_profile)

        result = await backend.import_ygg_profiles(user_id, "http://api.example.com", profiles)

    assert result["success_count"] == 0
    assert result["failure_count"] == 2

    by_id = {f["profile_id"]: f["detail"] for f in result["failed"]}
    # 内部异常：通用文案，不泄露
    assert by_id["batch_internal_pid"] == "导入失败"
    assert secret not in by_id["batch_internal_pid"]
    # 业务错误：保留可读提示
    assert "UUID" in by_id["batch_biz_pid"]
