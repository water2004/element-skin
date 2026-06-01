# 部署约束：单实例运行

## 摘要

当前后端在**进程内**维护多份内存状态，这些状态**不跨进程共享**。因此后端
**只能以单进程 / 单实例运行**（uvicorn 默认单 worker，未指定 `--workers >1`）。
若要水平扩展或多 worker，必须先把这些状态外置到共享存储（如 Redis）。

## 进程内状态清单

| 位置 | 用途 | 外置方案 |
| --- | --- | --- |
| `routers/microsoft_routes.py` 中的 `InMemoryStateStore` | 微软 OAuth `state` / 临时 token（一次性、带 TTL） | 已抽象 `StateStore` 接口（`utils/state_store.py`），换 Redis 实现即可，路由逻辑不变 |
| `database_module` 中 `SettingModule._cache` | 站点设置缓存 | 尚未外置 |
| `FallbackModule` 的多份缓存 | fallback 验证服务缓存 | 尚未外置 |
| `RateLimiter._attempts`（如为内存实现） | 限流计数 | 尚未外置 |

## 影响

- **多 worker（`uvicorn --workers N`，N>1）**：OAuth 回调可能落到与发起 `auth-url`
  不同的 worker，导致 `state` 找不到、流程失败；限流计数、settings 缓存各 worker
  不一致。
- **多实例（多容器 / 多 Pod）**：同上，且问题更明显。

## 当前配置确认

- `Dockerfile` 的 `CMD` **未**指定 `--workers`，默认 1 个 worker，符合单实例约束。
- 本地 `routes_reference.py:__main__` 的 `uvicorn.run` 同样默认单进程。

## 扩展路线（未来）

1. 先将 `StateStore` 切到 Redis 实现（改动面最小，接口已就绪）。
2. 再把 settings / fallback / 限流缓存改为 Redis 或带失效广播的共享缓存。
3. 完成后方可安全启用多 worker / 多实例。
