"""Token persistence abstractions."""

from __future__ import annotations

import json
import os
from abc import ABC, abstractmethod
from pathlib import Path

from ..models import TokenSet


class TokenStore(ABC):
    @abstractmethod
    def load(self) -> TokenSet | None:
        raise NotImplementedError

    @abstractmethod
    def save(self, tokens: TokenSet) -> None:
        raise NotImplementedError

    @abstractmethod
    def clear(self) -> None:
        raise NotImplementedError


class MemoryTokenStore(TokenStore):
    def __init__(self, tokens: TokenSet | None = None):
        self.tokens = tokens

    def load(self) -> TokenSet | None:
        return self.tokens

    def save(self, tokens: TokenSet) -> None:
        self.tokens = tokens

    def clear(self) -> None:
        self.tokens = None


class FileTokenStore(TokenStore):
    def __init__(self, path: str | os.PathLike[str], *, mode: int = 0o600):
        self.path = Path(path)
        self.mode = mode

    def load(self) -> TokenSet | None:
        if not self.path.exists():
            return None
        data = json.loads(self.path.read_text(encoding="utf-8"))
        return TokenSet.from_mapping(data)

    def save(self, tokens: TokenSet) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        payload = {
            "access_token": tokens.access_token,
            "token_type": tokens.token_type,
            "expires_in": tokens.expires_in,
            "scope": tokens.scope,
            "refresh_token": tokens.refresh_token,
            "permissions": list(tokens.permissions),
            "id_token": tokens.id_token,
        }
        self.path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
        try:
            os.chmod(self.path, self.mode)
        except OSError:
            pass

    def clear(self) -> None:
        try:
            self.path.unlink()
        except FileNotFoundError:
            pass
