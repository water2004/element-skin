# 阶段 5：补索引 + 开启 proxy headers + DB 初始化 fail-fast

## 目标

低风险、高收益的基础设施修复：

1. 为高频查询/排序列补**索引**，避免随数据量增长退化为全表扫描。
2. uvicorn 开启 **proxy headers**，让反向代理后能拿到真实客户端 IP（限流、会话 IP 才有意义）。
3. DB 初始化 SQL 失败时 **fail-fast**，不要带着残缺 schema 启动。

## 问题证据

### 缺索引

`database_module/initsql.py` 仅建了主键与少量 UNIQUE。以下高频访问列无索引：

- `profiles(user_id)` —— `get_profiles_by_user*`、`count_profiles_by_user`、删除用户级联。
- `tokens(user_id)` —— `delete_tokens_by_user`、`delete_expired_tokens`、清理。
- `tokens(profile_id)` —— `delete_tokens_by_profile`。
- `tokens(user_id, created_at)` —— `delete_surplus_tokens` 的子查询（**每次登录/刷新都跑**）。
- `user_textures(user_id, created_at DESC, hash DESC)` —— `get_for_user_cursor` 的游标排序。
- `skin_library(is_public, created_at DESC, skin_hash DESC)` —— 公开皮肤库分页。
- `skin_library(created_at DESC, skin_hash DESC)` —— 管理员全量材质分页。
- `whitelisted_users(endpoint_id)` —— 白名单查询（虽有缓存，list 仍直查）。
- `sessions(access_token)` —— `has_joined` 经 server_id 查 session 后再用 access_token 查 token（token 已是主键，sessions 主键是 server_id，此项视实际查询模式可选）。

> `profiles.name`、`users.email`、`users.display_name` 已有 UNIQUE 隐式索引；但 `search_users_cursor` / `list_all_profiles_cursor` 用 `ILIKE '%q%'` 前缀通配，B-tree 索引无法命中——这是模糊搜索的固有代价，**不在本阶段强行解决**（如需可后续上 `pg_trgm` GIN 索引，单列一项记入 phase-6 备选）。

### proxy headers

- `Dockerfile` CMD：`uvicorn routes_reference:app --host 0.0.0.0 --port 8000 --loop asyncio`，**无** `--proxy-headers` / `--forwarded-allow-ips`。
- 部署架构为前端 Nginx 容器在后端之前。结果 `request.client.host`（`utils/rate_limiter.py:46`、`yggdrasil_routes.py:92`）拿到的是 Nginx 容器 IP：
  - 认证限流变成「所有用户共享一个计数」——要么互相误伤、要么形同虚设。
  - `sessions.ip` 记录全是代理 IP，审计失真。

### DB 初始化吞错

- `database_module/main.py:24-26`：`INIT_SQL` 执行失败仅 `print("⚠️ 数据库初始化失败")` 后继续。应用会带着不完整的表结构启动，后续请求神秘报错。

## 改造清单

### 5.1 补索引（追加到 INIT_SQL，幂等）

在 `database_module/initsql.py` 的 `INIT_SQL` 末尾追加（全部 `IF NOT EXISTS`，幂等安全）：

```sql
-- 性能索引（幂等）
CREATE INDEX IF NOT EXISTS idx_profiles_user_id ON profiles(user_id);
CREATE INDEX IF NOT EXISTS idx_tokens_user_id ON tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_tokens_profile_id ON tokens(profile_id);
CREATE INDEX IF NOT EXISTS idx_tokens_user_created ON tokens(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_textures_user_created_hash
    ON user_textures(user_id, created_at DESC, hash DESC);
CREATE INDEX IF NOT EXISTS idx_skin_library_public_created_hash
    ON skin_library(is_public, created_at DESC, skin_hash DESC);
CREATE INDEX IF NOT EXISTS idx_skin_library_created_hash
    ON skin_library(created_at DESC, skin_hash DESC);
CREATE INDEX IF NOT EXISTS idx_whitelisted_users_endpoint ON whitelisted_users(endpoint_id);
```

> 这些是新建表后即生效的索引。对已有大表，`CREATE INDEX` 会短暂锁表；如线上数据量大，改用 `CREATE INDEX CONCURRENTLY`（但它不能在事务/单条多语句里跑，需单独执行）。当前 `INIT_SQL` 是一次性多语句执行，普通 `CREATE INDEX IF NOT EXISTS` 对中小表足够；大表迁移记为运维注意事项。

### 5.2 开启 proxy headers

`Dockerfile` CMD 增加参数：

```dockerfile
CMD ["python3", "-m", "uvicorn", "routes_reference:app", \
     "--host", "0.0.0.0", "--port", "8000", "--loop", "asyncio", \
     "--proxy-headers", "--forwarded-allow-ips", "*"]
```

- `--proxy-headers` 让 uvicorn 解析 `X-Forwarded-For` / `X-Forwarded-Proto`。
- `--forwarded-allow-ips`：理想值是 Nginx 容器的 IP/网段；容器网络 IP 常不固定，若整个后端不直接暴露公网（只能经 Nginx 访问），用 `*` 可接受。**若后端可能被直接访问，务必收敛为具体可信代理 IP**，否则客户端可伪造 `X-Forwarded-For` 绕过限流。
- 同步要求：Nginx 需正确设置 `proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;` 与 `X-Forwarded-Proto $scheme;`（属前端容器配置，记为联动项）。

> 本地 `routes_reference.py` 的 `uvicorn.run(...)`（`__main__` 分支）也可加 `proxy_headers=True, forwarded_allow_ips="*"`，保持开发与容器一致。

### 5.3 DB 初始化 fail-fast

`database_module/main.py:init`：

```python
async def init(self):
    await self.ensure_conn()
    try:
        await self.execute(INIT_SQL)
    except Exception as e:
        # 不再吞错：schema 未就绪时应阻止应用启动
        raise RuntimeError(f"数据库初始化失败，拒绝启动：{e}") from e
    await self.setting.init()
    await self.fallback.init()
```

> `INIT_SQL` 本身是幂等的（`IF NOT EXISTS` / `ON CONFLICT`），正常情况下重复执行不会出错；真出错说明权限/连接/SQL 有实质问题，正该 fail-fast。

## 影响文件

- 修改：`database_module/initsql.py`（追加索引）
- 修改：`database_module/main.py`（init fail-fast）
- 修改：`Dockerfile`（CMD 加 proxy 参数）、可选 `routes_reference.py`（`__main__` 的 uvicorn.run）
- 联动（前端仓库）：Nginx `X-Forwarded-*` 透传配置

## 测试与验证

- 索引幂等：连续两次 `db.init()` 不报错；`tests/database/test_database_init.py` 增加断言关键索引存在（查 `pg_indexes`）。
- 索引生效（可选）：对 `tokens`/`user_textures` 插入较多行后 `EXPLAIN` 关键分页/清理查询，确认走 Index Scan 而非 Seq Scan。
- proxy headers：本地用带 `X-Forwarded-For: 1.2.3.4` 的请求经 uvicorn（开 `--proxy-headers`）打到一个回显 `request.client.host` 的临时端点，确认拿到 `1.2.3.4`；不开时拿到直连 IP。限流按真实 IP 分桶。
- init fail-fast：临时把 DSN 指向无权限/不存在的库，确认 `db.init()` 抛 `RuntimeError` 且应用不进入可服务状态。
- `pytest -q` 全绿。

## 风险与回滚

低风险。索引为追加且幂等；proxy 参数与 init 改动均可独立 revert。唯一需提醒：`--forwarded-allow-ips *` 仅在「后端不直接暴露公网」前提下安全，否则需收敛为可信代理 IP——这点务必与实际网络拓扑核对。
