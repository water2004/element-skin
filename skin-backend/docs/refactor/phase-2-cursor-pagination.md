# 阶段 2：统一游标分页编解码到单一边界

## 目标

把游标的 **base64 编码/解码**全部收敛到一层（传输/API 边界）。数据库层只接收/返回**排序键的原始值**，不再知道"游标"这个 base64 字符串的存在。

## 当前问题（证据）

游标逻辑被劈成两半，位置不统一：

- **编码（encode）全在 DB 层**：
  - `user.py`：`107, 152, 234, 303, 499` 行，每个 `*_cursor` 方法末尾 `CursorEncoder.encode(...)`。
  - `texture.py`：`122, 268, 357` 行同理。
- **解码（decode）位置混乱**：
  - 有时在 router：`site_routes.py:218`（`/me/textures`）、`245`（`/me/profiles`）、`330`（`/public/skin-library`）手动 `CursorEncoder.decode`。
  - 有时在 DB 层：`texture.py:294` 的 `list_all_textures_cursor` 自己 `decode(after_cursor)`。

同一个分页机制，DB 方法的签名都不一致：有的收 `last_created_at + last_hash`（已解码的键），有的收 `after_cursor`（未解码的字符串）。这是难维护的直接来源。

## 设计决策

**游标是 API 契约**：客户端拿到的不透明字符串长什么样、怎么编码，是传输层的事。DB 层应只表达「从这个排序键之后取 N 条」。

统一约定：

1. **DB 层 `*_cursor` 方法**：
   - 输入：已解码的排序键（如 `last_created_at: int | None`、`last_hash: str | None`），**不接收 base64 字符串**。
   - 输出：`{"items": [...], "has_next": bool, "next_key": dict | None, "page_size": int}`。
     - 关键改动：把 `next_cursor`（已编码字符串）改为 `next_key`（原始 dict，如 `{"last_created_at": ..., "last_hash": ...}`），**不调用 `CursorEncoder`**。
2. **边界层负责编解码**：进入时 decode 成键传给 DB，返回时把 `next_key` encode 成 `next_cursor` 给客户端。

## 边界放在哪？

两个候选：router 或 backend。**选 backend**，理由：
- 阶段 4 的方向是「router 只调 backend」。把游标编解码放 backend，能让 router 完全不碰 `CursorEncoder`。
- 现在有些分页端点（如 `/public/skin-library`）已经在做 backend 级编排（收集 uploader 名字），编解码放一起更内聚。

因此引入一个轻量 helper（已有 `utils/pagination.py`，扩展它）：

```python
# utils/pagination.py 已有 CursorEncoder，新增便捷函数
def decode_cursor(cursor: str | None, required_keys: tuple[str, ...]) -> dict | None:
    """解码并校验必需键；非法游标抛 ValueError（由 backend 转 HTTPException 400）。"""
    if not cursor:
        return None
    data = CursorEncoder.decode(cursor)
    if not data or any(k not in data for k in required_keys):
        raise ValueError("Invalid cursor")
    return data

def encode_next(next_key: dict | None) -> str | None:
    return CursorEncoder.encode(next_key) if next_key else None
```

## 改造清单

### DB 层（去掉 encode）

每个 `*_cursor` 方法，把：

```python
next_cursor = None
if has_next:
    next_cursor = CursorEncoder.encode({"last_id": rows[limit][0]})
return {..., "next_cursor": next_cursor, ...}
```

改为：

```python
next_key = None
if has_next:
    next_key = {"last_id": rows[limit][0]}
return {..., "next_key": next_key, ...}
```

涉及方法：
- `user.py`：`list_users_cursor`、`search_users_cursor`、`get_profiles_by_user_cursor`、`list_all_profiles_cursor`、`list_invites_cursor`。
- `texture.py`：`get_for_user_cursor`、`get_from_library_cursor`、`list_all_textures_cursor`。
- 特别地，`list_all_textures_cursor`（`texture.py:280-367`）当前**入参是 `after_cursor` 字符串并自己 decode**。改为入参 `last_created_at` / `last_skin_hash`，decode 移到 backend。删除其顶部的 `from utils.pagination import CursorEncoder`。

> 删干净后 `texture.py` / `user.py` 不再 import `CursorEncoder`。

### Backend 层（统一 encode/decode）

凡是返回分页结果的 backend 方法（`admin_backend.get_all_profiles` / `get_all_textures`，以及阶段 4 会新建的 `list_my_textures` / `list_my_profiles` / `get_skin_library`）：

```python
async def get_all_textures(self, limit, cursor=None, query=None, type_filter=None):
    key = decode_cursor(cursor, ("last_created_at", "last_skin_hash"))  # 可能为 None
    result = await self.db.texture.list_all_textures_cursor(
        limit, last_created_at=(key or {}).get("last_created_at"),
        last_skin_hash=(key or {}).get("last_skin_hash"),
        query=query, type_filter=type_filter)
    result["next_cursor"] = encode_next(result.pop("next_key"))
    return result
```

### Router 层（不再碰 CursorEncoder）

`site_routes.py` 三处 `from utils.pagination import CursorEncoder` 局部 import 全部删除（`211, 239, 321` 行附近）。router 只把原始 `cursor` 字符串透传给 backend。

## 影响文件

- 修改：`utils/pagination.py`（新增 `decode_cursor` / `encode_next`）
- 修改：`database_module/modules/user.py`、`database_module/modules/texture.py`（`next_cursor`→`next_key`，去 encode/decode 和 import）
- 修改：`backends/admin_backend.py`（`get_all_profiles` / `get_all_textures` 收口编解码）
- 修改：`routers/site_routes.py`、`routers/admin_routes.py`（去掉 router 内 decode）
- 关联：阶段 4 新建的 backend 分页方法直接采用本约定

## 测试

- `tests/database/test_cursor_pagination.py`：断言从 `next_cursor` 改为 `next_key`（dict），不再断言 base64 字符串。
- `tests/database/test_texture.py` / `test_user.py`：分页返回结构断言同步更新。
- `tests/backends/*`：新增/更新对 backend 编解码的用例（非法游标 → 400）。
- `tests/api/*`：API 对外仍返回 `next_cursor`（base64 字符串），契约不变 —— 这是回归保护的关键。

## 完成标准

- `grep -rn "CursorEncoder" skin-backend/database_module` 无结果。
- `grep -rn "CursorEncoder" skin-backend/routers` 无结果。
- DB 层所有 `*_cursor` 方法签名统一为「收原始键 / 返 `next_key`」。
- `pytest -q` 全绿；API 返回的 `next_cursor` 字符串与重构前逐字节一致（可加一个快照测试）。

## 风险与回滚

中等风险：改动点多但机械。建议分两个 commit —— 先改 DB+helper（含其单测），再改 backend/router。任一步出错可独立 revert。
