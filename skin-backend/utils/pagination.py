"""游标分页工具模块"""

import json
import base64
from typing import Any, Dict, Optional, List, TypeVar, Generic
from pydantic import BaseModel

T = TypeVar('T')


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


class CursorPaginationResponse(BaseModel):
    """游标分页响应模型"""
    items: List[Any]
    has_next: bool
    has_prev: bool = False
    next_cursor: Optional[str] = None
    prev_cursor: Optional[str] = None
    page_size: int


class OffsetCursorCompatResponse(BaseModel):
    """兼容性响应：支持offset和cursor两种分页"""
    items: List[Any]
    total: Optional[int] = None  # offset方式时有值
    has_next: Optional[bool] = None  # cursor方式时有值
    next_cursor: Optional[str] = None
    page_size: int


def build_cursor_response(
    items: List[Any],
    page_size: int,
    has_next: bool,
    next_cursor_data: Optional[Dict[str, Any]] = None,
    has_prev: bool = False,
    prev_cursor_data: Optional[Dict[str, Any]] = None,
) -> Dict[str, Any]:
    """构建游标分页响应
    
    Args:
        items: 本页数据项
        page_size: 分页大小
        has_next: 是否有下一页
        next_cursor_data: 下一页游标数据（若有下一页）
        has_prev: 是否有上一页
        prev_cursor_data: 上一页游标数据（若有上一页）
    
    Returns:
        API响应字典
    """
    return {
        "items": items,
        "has_next": has_next,
        "has_prev": has_prev,
        "next_cursor": CursorEncoder.encode(next_cursor_data) if next_cursor_data else None,
        "prev_cursor": CursorEncoder.encode(prev_cursor_data) if prev_cursor_data else None,
        "page_size": page_size,
    }
