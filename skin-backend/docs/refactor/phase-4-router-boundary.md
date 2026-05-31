# 阶段 4：恢复 router → backend 边界

## 目标

让 router 只做 HTTP 适配：解析请求 → 调**一个** backend 方法 → 组织响应。消除 router 直接调 `db.*`（约 40 处）和 router 内的多步业务编排。

## 当前问题（证据）

`grep` 统计 router 直接调 `db.*`：

```
routers/admin_routes.py:6
routers/site_routes.py:16
routers/yggdrasil_routes.py:5
routers/microsoft_routes.py:13
```

典型越界：

1. **材质上传**（`site_routes.py:196, 394`）：router 直接 `db.texture.upload(...)`（阶段 1 已改为调 backend，本阶段确认收口）。
2. **材质字段更新**（`site_routes.py:280-296`）：`/me/textures/{hash}/{texture_type}` 在 router 里依次判断 `note` / `model` / `is_public` 并分别调三个 `db.texture.update_*` —— "哪些字段可改、改的顺序"是业务编排，不该在 router。
3. **皮肤库聚合**（`site_routes.py:314-358`）：`/public/skin-library` 在 router 里做：查 `enable_skin_library` 开关 → decode 游标 → 调 DB → 收集 `uploader_ids` → `db.user.get_display_names_by_ids` → 拼装 `uploader_name`。这是教科书级的 backend 编排，却整段写在路由函数里。
4. **角色列表组装**（`site_routes.py:232-266`）：`/me/profiles` 在 router 里把 `PlayerProfile` 对象手动映射成响应 dict。
5. **直传应用材质**（`site_routes.py:375-416`）：`/textures/upload` 在 router 里串了「上传→应用到角色→更新模型」三步。

## 设计原则

- router 函数体目标：**≤ 5 行**，形如 `return await backend.X(...)` 或 `result = await backend.X(...); 组装 Response`。
- Cookie / `Response` / `set_cookie` 这类**纯 HTTP 机制**留在 router（如 `site_login` 设置 cookie，合理）。
- 响应 DTO 的塑形（对象 → dict）移入 backend，router 直接返回 backend 给的 dict。
- `microsoft_routes.py`（13 处 `db.*`）和 `yggdrasil_routes.py`（5 处）也在范围内，逐一评估：Yggdrasil 协议端点有些是极薄转发，可保留对 `db` 的只读查询，但写操作和多步逻辑须下沉。

> **实现备注（与文档差异）**：最终未保留任何 router 层 `db.*` 只读转发。`microsoft_routes` 的全部编排下沉到新建的 `MicrosoftBackend`，`yggdrasil_routes` 的展示逻辑（`build_profile_json` / `build_authenticate_response` / `lookup_profile_by_name` / `build_metadata`）下沉到 `YggdrasilBackend`。验收 grep `db\.(texture|user|setting|fallback|verification)\.` 在 `routers/` 下零匹配——比文档"显著下降"更彻底。router 仅保留 Cookie / `Response` 重定向 / fallback 组合（local-then-fallback）等纯 HTTP 机制。

## 改造清单（site_routes 为主）

| 端点 | 现状 | 目标 backend 方法 |
|------|------|-------------------|
| `POST /me/textures` (183) | `db.texture.upload` | `site_backend.upload_texture_to_library`（阶段 1 已建） |
| `POST /textures/upload` (375) | 三步内联编排 | `site_backend.upload_and_apply_texture(user_id, uuid, bytes, type, model, is_public)` |
| `GET /me/textures` (203) | router decode + 调 DB | `site_backend.list_my_textures(user_id, cursor, limit, type)` |
| `GET /me/profiles` (232) | router 映射对象 | `site_backend.list_my_profiles(user_id, cursor, limit)` 返回成品 dict |
| `GET /me/textures/{h}/{t}` (268) | `db.texture.get_texture_info` | `site_backend.get_my_texture_detail(...)` |
| `PATCH /me/textures/{h}/{t}` (280) | 三个 `db.texture.update_*` | `site_backend.update_my_texture(user_id, h, t, body)` |
| `DELETE /me/textures/{h}/{t}` (298) | `db.texture.delete_from_library` | `site_backend.remove_my_texture(...)` |
| `POST /me/textures/{h}/add` (305) | `db.texture.add_to_user_wardrobe` | `site_backend.add_texture_to_wardrobe(...)` |
| `GET /public/skin-library` (314) | 整段聚合编排 | `site_backend.get_public_skin_library(cursor, limit, type)` |
| `GET /public/settings` (418) | router 拼 settings dict | `site_backend.get_public_settings()`（与阶段 5 的 SettingsBackend 协调） |

新增的 backend 方法多数是把 router 现有代码**整段平移**，逻辑不变。例如皮肤库聚合：

```python
# site_backend.py
async def get_public_skin_library(self, cursor, limit, texture_type):
    if await self.db.setting.get("enable_skin_library", "true") != "true":
        raise HTTPException(403, "Skin library is disabled by administrator")
    result = await self.list_library_page(cursor, limit, texture_type)  # 内部走阶段2的编解码
    uploader_ids = list({i["uploader"] for i in result["items"] if i.get("uploader")})
    names = await self.db.user.get_display_names_by_ids(uploader_ids)
    result["items"] = [{**i, "uploader_name": names.get(i.get("uploader"), "")} for i in result["items"]]
    return result
```

## 与其它阶段的协调

- **依赖阶段 1**：上传相关方法（`upload_texture_to_library`）已在阶段 1 建好，本阶段只是让 router 改调它。
- **依赖阶段 2**：分页 backend 方法采用「backend 编解码游标」的约定，router 只透传 `cursor` 字符串。
- **与阶段 5 协调**：`/public/settings` 和 `/admin` 设置端点的 backend 化在阶段 5 落地 `SettingsBackend`，本阶段对这两个端点可先占位、留到 5 一并完成，避免重复改。

> 这是把阶段 4 排在 3、5 之后的原因：backend 的权威方法入口要先齐备，router 才能干净地改调。

## 影响文件

- 修改：`routers/site_routes.py`（主战场，端点函数瘦身）
- 修改：`routers/admin_routes.py`、`routers/microsoft_routes.py`、`routers/yggdrasil_routes.py`（下沉写操作与多步逻辑）
- 修改：`backends/site_backend.py`、`backends/admin_backend.py`、`backends/microsoft_backend.py`（接收平移过来的编排方法）

## 测试

- `tests/api/*`（全部）：这是本阶段的主验证手段 —— **API 行为契约必须逐一保持**。重点跑 `test_site_api.py`、`test_admin_api.py`、`test_microsoft_import_api.py`、`test_yggdrasil_api.py`。
- `tests/backends/*`：为新平移进 backend 的方法补单测（皮肤库聚合、`update_my_texture` 的字段分支、`upload_and_apply_texture` 的三步）。
- 移除/改写原本针对 router 内联逻辑的测试（如有）。

## 完成标准

- `grep -rn "db\.\(texture\|user\|setting\|fallback\|verification\)\." skin-backend/routers` 显著下降；剩余项须是有意保留的极薄只读转发，并在 PR 描述中逐条说明理由。
- `site_routes.py` 中不再出现 `CursorEncoder`、`get_display_names_by_ids`、对象→dict 的手动映射。
- `pytest tests/api -q` 全绿，且与重构前响应一致。

## 风险与回滚

风险中高（改动面最广、最接近对外契约）。缓解：
- 严格"逻辑平移、不改行为"，不顺手优化。
- 按 router 文件分多个 commit（site → admin → microsoft → yggdrasil），每个独立可 revert。
- 依赖 `tests/api` 作为安全网；若某端点无 API 测试，先补测试再重构。
