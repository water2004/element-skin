# Webhook Worker SQL Profile

- 生成时间：`2026-08-24T01:21:50+08:00`
- 命令：`WEBHOOK_SQL_PROFILE_ENABLE=1 go test ./cmd/loadtest -run TestWebhookWorkerSQLProfile -count=1 -v`
- 事件：`1000`；Worker 独立数据库连接池：`2`；接收端：进程内零延迟 `204`
- 本次 Redis 环境：本机没有 Docker 或 Redis 服务，使用独立的 `miniredis` Redis 协议进程；权限缓存的绝对延迟不作为生产 Redis SLA
- 紧循环展开：`135.5ms`；紧循环投递：`150.7ms`；生产轮询端到端：`12.25s`
- 说明：累计 SQL 耗时按调用求和，并发查询可能重叠，因此不能直接等同于墙钟时间。

| 阶段 | SQL | 调用 | 累计 | 平均 | P95 | 最大 | 错误 |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `production-loop` | `expansions.complete_batch` | 5 | 33.3ms | 6.7ms | 7.1ms | 7.1ms | 0 |
| `production-loop` | `deliveries.claim_due` | 20 | 30.9ms | 1.5ms | 2.0ms | 2.4ms | 0 |
| `production-loop` | `events.claim_pending` | 5 | 28.0ms | 5.6ms | 10.3ms | 10.3ms | 0 |
| `production-loop` | `deliveries.complete_batch` | 20 | 23.0ms | 1.1ms | 1.6ms | 1.8ms | 0 |
| `production-loop` | `grants.authorization_permission_state` | 25 | 3.6ms | 0.1ms | 0.5ms | 1.5ms | 0 |
| `production-loop` | `endpoints.list_subscribed` | 5 | 1.6ms | 0.3ms | 0.6ms | 0.6ms | 0 |
| `tight-delivery` | `deliveries.claim_due` | 20 | 38.2ms | 1.9ms | 3.0ms | 3.4ms | 0 |
| `tight-delivery` | `deliveries.complete_batch` | 20 | 23.2ms | 1.2ms | 2.0ms | 2.2ms | 0 |
| `tight-delivery` | `grants.authorization_permission_state` | 20 | 1.8ms | 0.1ms | 0.6ms | 0.7ms | 0 |
| `tight-dispatch` | `expansions.complete_batch` | 5 | 37.7ms | 7.5ms | 9.7ms | 9.7ms | 0 |
| `tight-dispatch` | `events.claim_pending` | 5 | 19.8ms | 4.0ms | 5.4ms | 5.4ms | 0 |
| `tight-dispatch` | `endpoints.list_subscribed` | 5 | 3.2ms | 0.7ms | 1.6ms | 1.6ms | 0 |
| `tight-dispatch` | `permissions.subject_roles` | 1 | 2.0ms | 2.0ms | 2.0ms | 2.0ms | 0 |
| `tight-dispatch` | `grants.authorization_permission_state` | 5 | 1.0ms | 0.2ms | 0.5ms | 0.5ms | 0 |
| `tight-dispatch` | `permissions.subject_overrides` | 1 | - | - | - | - | 0 |
