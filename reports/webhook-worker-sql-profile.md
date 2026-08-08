# Webhook Worker SQL Profile

- 生成时间：`2026-08-08T23:30:34+08:00`
- 命令：`WEBHOOK_SQL_PROFILE_ENABLE=1 go test ./cmd/loadtest -run TestWebhookWorkerSQLProfile -count=1 -v`
- 事件：`1000`；Worker 独立数据库连接池：`5`；接收端：进程内零延迟 `204`
- 紧循环展开：`929.3ms`；紧循环投递：`402.6ms`；生产轮询端到端：`9.55s`
- 说明：累计 SQL 耗时按调用求和，并发查询可能重叠，因此不能直接等同于墙钟时间。

| 阶段 | SQL | 调用 | 累计 | 平均 | P95 | 最大 | 错误 |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `production-loop` | `deliveries.complete` | 1000 | 353.9ms | 0.4ms | 0.7ms | 1.4ms | 0 |
| `production-loop` | `permissions.subject_roles` | 2000 | 238.4ms | 0.1ms | 0.6ms | 1.2ms | 0 |
| `production-loop` | `permissions.delegation_policy` | 2000 | 233.7ms | 0.1ms | 0.6ms | 1.2ms | 0 |
| `production-loop` | `permissions.client_requested` | 2000 | 212.9ms | 0.1ms | 0.6ms | 1.2ms | 0 |
| `production-loop` | `permissions.subject_overrides` | 2000 | 176.1ms | 0.1ms | 0.6ms | 1.4ms | 0 |
| `production-loop` | `grants.active_by_user_client` | 2000 | 173.6ms | 0.1ms | 0.6ms | 1.4ms | 0 |
| `production-loop` | `permissions.ensure_user_subject` | 2000 | 164.8ms | 0.1ms | 0.6ms | 1.5ms | 0 |
| `production-loop` | `transaction.commit` | 1000 | 162.7ms | 0.2ms | 0.6ms | 2.9ms | 0 |
| `production-loop` | `deliveries.insert` | 1000 | 137.9ms | 0.1ms | 0.6ms | 0.8ms | 0 |
| `production-loop` | `endpoints.list_subscribed` | 1000 | 111.1ms | 0.1ms | 0.6ms | 1.5ms | 0 |
| `production-loop` | `events.complete_expansion` | 1000 | 101.8ms | 0.1ms | 0.6ms | 1.2ms | 0 |
| `production-loop` | `deliveries.claim_due` | 20 | 69.0ms | 3.5ms | 7.5ms | 7.6ms | 0 |
| `production-loop` | `transaction.begin` | 1000 | 51.0ms | 0.1ms | 0.5ms | 0.9ms | 0 |
| `production-loop` | `events.list_pending` | 20 | 7.7ms | 0.4ms | 1.2ms | 1.8ms | 0 |
| `tight-delivery` | `deliveries.complete` | 1000 | 294.3ms | 0.3ms | 0.6ms | 1.3ms | 0 |
| `tight-delivery` | `permissions.delegation_policy` | 1000 | 131.1ms | 0.1ms | 0.5ms | 1.4ms | 0 |
| `tight-delivery` | `permissions.subject_roles` | 1000 | 116.4ms | 0.1ms | 0.5ms | 1.0ms | 0 |
| `tight-delivery` | `grants.active_by_user_client` | 1000 | 106.9ms | 0.1ms | 0.6ms | 1.0ms | 0 |
| `tight-delivery` | `permissions.client_requested` | 1000 | 104.1ms | 0.1ms | 0.6ms | 1.6ms | 0 |
| `tight-delivery` | `permissions.subject_overrides` | 1000 | 90.9ms | 0.1ms | 0.5ms | 0.9ms | 0 |
| `tight-delivery` | `permissions.ensure_user_subject` | 1000 | 76.8ms | 0.1ms | 0.5ms | 0.8ms | 0 |
| `tight-delivery` | `deliveries.claim_due` | 20 | 40.7ms | 2.0ms | 3.1ms | 3.2ms | 0 |
| `tight-dispatch` | `transaction.commit` | 1000 | 157.3ms | 0.2ms | 0.6ms | 0.9ms | 0 |
| `tight-dispatch` | `deliveries.insert` | 1000 | 145.3ms | 0.1ms | 0.6ms | 1.6ms | 0 |
| `tight-dispatch` | `endpoints.list_subscribed` | 1000 | 80.6ms | 0.1ms | 0.6ms | 1.7ms | 0 |
| `tight-dispatch` | `permissions.subject_roles` | 1000 | 71.2ms | 0.1ms | 0.6ms | 1.3ms | 0 |
| `tight-dispatch` | `permissions.delegation_policy` | 1000 | 70.9ms | 0.1ms | 0.6ms | 1.0ms | 0 |
| `tight-dispatch` | `events.complete_expansion` | 1000 | 70.0ms | 0.1ms | 0.5ms | 0.8ms | 0 |
| `tight-dispatch` | `permissions.client_requested` | 1000 | 51.8ms | 0.1ms | 0.5ms | 0.7ms | 0 |
| `tight-dispatch` | `permissions.subject_overrides` | 1000 | 50.8ms | 0.1ms | 0.5ms | 0.8ms | 0 |
| `tight-dispatch` | `grants.active_by_user_client` | 1000 | 49.2ms | 0.0ms | 0.5ms | 1.3ms | 0 |
| `tight-dispatch` | `permissions.ensure_user_subject` | 1000 | 48.8ms | 0.0ms | 0.5ms | 0.8ms | 0 |
| `tight-dispatch` | `transaction.begin` | 1000 | 32.5ms | 0.0ms | 0.5ms | 0.7ms | 0 |
| `tight-dispatch` | `events.list_pending` | 5 | 4.3ms | 0.9ms | 2.9ms | 2.9ms | 0 |
