from __future__ import annotations

import json

import pytest

from element_skin_sdk import FileTokenStore, MemoryTokenStore
from element_skin_sdk.models import TokenSet
from element_skin_sdk.oauth.token_store import TokenStore


def test_memory_token_store_round_trips_and_clears_exact_token() -> None:
    tokens = TokenSet(
        access_token="access-token-1",
        token_type="Bearer",
        expires_in=3600,
        scope="account.read.self",
        refresh_token="refresh-token-1",
        permissions=("account.read.self",),
        id_token="id-token-1",
    )
    store = MemoryTokenStore()

    assert store.load() is None
    store.save(tokens)
    assert store.load() == tokens
    store.clear()
    assert store.load() is None


def test_file_token_store_writes_structured_json_and_clears_file(tmp_path) -> None:
    path = tmp_path / "tokens.json"
    tokens = TokenSet(
        access_token="access-token-1",
        token_type="Bearer",
        expires_in=3600,
        scope="account.read.self",
        refresh_token="refresh-token-1",
        permissions=("account.read.self",),
        id_token="id-token-1",
    )
    store = FileTokenStore(path)

    store.save(tokens)

    assert json.loads(path.read_text(encoding="utf-8")) == {
        "access_token": "access-token-1",
        "token_type": "Bearer",
        "expires_in": 3600,
        "scope": "account.read.self",
        "refresh_token": "refresh-token-1",
        "permissions": ["account.read.self"],
        "id_token": "id-token-1",
    }
    assert store.load() == tokens
    store.clear()
    assert not path.exists()


def test_file_token_store_missing_file_and_repeated_clear_are_noops(tmp_path) -> None:
    path = tmp_path / "missing.json"
    store = FileTokenStore(path)

    assert store.load() is None
    store.clear()
    store.clear()
    assert not path.exists()


def test_file_token_store_ignores_chmod_failure(monkeypatch, tmp_path) -> None:
    path = tmp_path / "tokens.json"
    tokens = TokenSet("access-token-1", "Bearer", 3600)

    def fail_chmod(path_arg, mode):
        assert path_arg == path
        assert mode == 0o600
        raise OSError("chmod unavailable")

    monkeypatch.setattr("element_skin_sdk.oauth.token_store.os.chmod", fail_chmod)

    FileTokenStore(path).save(tokens)

    assert json.loads(path.read_text(encoding="utf-8")) == {
        "access_token": "access-token-1",
        "token_type": "Bearer",
        "expires_in": 3600,
        "scope": "",
        "refresh_token": None,
        "permissions": [],
        "id_token": None,
    }


def test_token_store_abstract_methods_raise_exact_errors() -> None:
    with pytest.raises(NotImplementedError):
        TokenStore.load(object())  # type: ignore[misc]
    with pytest.raises(NotImplementedError):
        TokenStore.save(object(), TokenSet("access-token-1", "Bearer", 3600))  # type: ignore[misc]
    with pytest.raises(NotImplementedError):
        TokenStore.clear(object())  # type: ignore[misc]
