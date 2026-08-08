# Webhook Worker SQL Profile

- 生成时间：`2026-08-09T00:16:17+08:00`
- 命令：`WEBHOOK_SQL_PROFILE_ENABLE=1 go test ./cmd/loadtest -run TestWebhookWorkerSQLProfile -count=1 -v`
- 事件：`1000`；Worker 独立数据库连接池：`2`；接收端：进程内零延迟 `204`
- 紧循环展开：`137.9ms`；紧循环投递：`165.8ms`；生产轮询端到端：`12.25s`
- 说明：累计 SQL 耗时按调用求和，并发查询可能重叠，因此不能直接等同于墙钟时间。

| 阶段 | SQL | 调用 | 累计 | 平均 | P95 | 最大 | 错误 |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `production-loop` | `expansions.complete_batch` | 5 | 36.2ms | 7.2ms | 8.1ms | 8.1ms | 0 |
| `production-loop` | `deliveries.claim_due` | 20 | 32.1ms | 1.6ms | 2.0ms | 3.0ms | 0 |
| `production-loop` | `events.claim_pending` | 5 | 26.8ms | 5.4ms | 9.0ms | 9.0ms | 0 |
| `production-loop` | `deliveries.complete_batch` | 20 | 21.2ms | 1.1ms | 1.6ms | 1.6ms | 0 |
| `production-loop` | `grants.authorization_permission_state` | 25 | 2.9ms | 0.1ms | 0.6ms | 0.8ms | 0 |
| `production-loop` | `endpoints.list_subscribed` | 5 | - | - | - | - | 0 |
| `tight-delivery` | `deliveries.claim_due` | 20 | 40.6ms | 2.0ms | 3.2ms | 3.3ms | 0 |
| `tight-delivery` | `deliveries.complete_batch` | 20 | 24.7ms | 1.2ms | 2.0ms | 2.0ms | 0 |
| `tight-delivery` | `grants.authorization_permission_state` | 20 | 2.9ms | 0.1ms | 0.6ms | 0.7ms | 0 |
| `tight-dispatch` | `expansions.complete_batch` | 5 | 40.3ms | 8.1ms | 9.6ms | 9.6ms | 0 |
| `tight-dispatch` | `events.claim_pending` | 5 | 19.9ms | 4.0ms | 6.6ms | 6.6ms | 0 |
| `tight-dispatch` | `endpoints.list_subscribed` | 5 | 3.7ms | 0.7ms | 1.3ms | 1.3ms | 0 |
| `tight-dispatch` | `permissions.subject_roles` | 1 | 1.1ms | 1.1ms | 1.1ms | 1.1ms | 0 |
| `tight-dispatch` | `grants.authorization_permission_state` | 5 | 0.5ms | 0.1ms | 0.5ms | 0.5ms | 0 |
| `tight-dispatch` | `permissions.subject_overrides` | 1 | - | - | - | - | 0 |
