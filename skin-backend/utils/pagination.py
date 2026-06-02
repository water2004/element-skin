"""游标分页工具模块"""

import json
import base64
from typing import Any, Dict, Optional


DEFAULT_LIMIT = 20
MAX_LIMIT = 100


def clamp_limit(limit: Optional[int], default: int = DEFAULT_LIMIT) -> int:
    """把分页 limit 收敛到 [1, MAX_LIMIT]。None 取 default。

    防御：limit=-1 触发 IndexError/500；limit=0 导致分页死循环；
    超大 limit 造成单次查询 DoS。非法/不可解析值回退 default。
    """
    if limit is None:
        return default
    try:
        limit = int(limit)
    except (TypeError, ValueError):
        return default
    return max(1, min(limit, MAX_LIMIT))


class CursorEncoder:
    """游标编码/解码工具"""

    @staticmethod
    def encode(cursor_data: Dict[str, Any]) -> str:
        """将游标数据编码为Base64字符串

        Args:
            cursor_data: 包含排序键的字典，例如 {"last_id": "user-123"}

        Returns:
            Base64编码的游标字符串
        """
        json_str = json.dumps(cursor_data, separators=(',', ':'), ensure_ascii=False)
        return base64.urlsafe_b64encode(json_str.encode()).decode().rstrip('=')

    @staticmethod
    def decode(cursor_str: str) -> Optional[Dict[str, Any]]:
        """将Base64字符串解码为游标数据

        Args:
            cursor_str: Base64编码的游标字符串

        Returns:
            解码后的游标数据字典，若解码失败返回None
        """
        try:
            # 补齐padding
            padding = 4 - (len(cursor_str) % 4)
            if padding != 4:
                cursor_str += '=' * padding
            json_str = base64.urlsafe_b64decode(cursor_str.encode()).decode()
            return json.loads(json_str)
        except Exception:
            return None


def decode_cursor(cursor: Optional[str], required_keys: tuple[str, ...]) -> Optional[Dict[str, Any]]:
    """解码并校验必需键；非法游标抛 ValueError（由 backend 转 HTTPException 400）。

    无游标（首页）返回 None。
    """
    if not cursor:
        return None
    data = CursorEncoder.decode(cursor)
    if not data or any(k not in data for k in required_keys):
        raise ValueError("Invalid cursor")
    return data


def encode_next(next_key: Optional[Dict[str, Any]]) -> Optional[str]:
    """把 DB 层返回的原始排序键 dict 编码为对外的不透明游标字符串。"""
    return CursorEncoder.encode(next_key) if next_key else None
