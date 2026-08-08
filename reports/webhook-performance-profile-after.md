# Webhook 性能架构改进复测报告

## 1. 结论

这次改进针对 profile 已确认的线性放大和并发领取问题，没有把本机参数当成生产最优值：

1. 1000 个事件的生产 Worker SQL 从 `18,040` 次降至 `80` 次，即从 `18.04` 次/event 降至 `0.08` 次/event，调用放大减少 `99.6%`。
2. 展开、投递完成都改为批量提交；订阅和授权按批次内等价键合并。投递授权仍在每个领取批次重新检查，1000 次投递对应 20 个领取批次和 20 次授权查询，不跨批复用授权结论。
3. 紧循环组合处理 1000 个事件由 `1.03s` 降至 `249.1ms`，本机零延迟样本约为原来的 `4.1` 倍。这个数字只证明本次实现相对基线的方向，不是生产吞吐承诺。
4. event 展开和 delivery 投递均使用带随机 token 的可过期 lease；`FOR UPDATE SKIP LOCKED` 允许多个 Worker 安全领取，过期 Worker 的完成结果会因 token 不匹配被拒绝。
5. Worker 保持独立进程和独立数据库连接池。连接预算与有工作批次间隔是配置项；仓库提供 `2` 个连接和 `3000ms` 的保守建议值，生产应按共享数据库容量、积压目标和第三方延迟调整。

## 2. 架构变化

| 路径 | 基线 | 改进后 |
| --- | --- | --- |
| event 领取 | 无展开 lease，多实例可能重复计算 | lease token + `SKIP LOCKED` 批量领取 |
| endpoint 查询 | 每 event 一次 | 同批 `(event_type, target_client_id)` 一次 |
| 展开授权 | 每 event/endpoint 重算完整权限 | 同批等价授权主体一次，权限位图复用 Redis 有效权限缓存 |
| 展开完成 | 每 event 一个事务 | 每批一个 SQL，同时插入 deliveries 并完成 events |
| delivery 授权 | 每 delivery 一次授权 SQL | 每个领取批次内等价授权主体一次；下一批重新检查 |
| delivery 完成 | 每 delivery 一次更新 | 每批一个带 lease token 条件的批量更新 |
| 调度 | 固定 `500ms` tick 决定吞吐 | 有工作按可配置预算间隔继续排空；空闲使用 `500ms` 轮询 |
| 资源隔离 | Worker 独立进程，连接数写死 | Worker 独立进程，连接池和活动间隔可配置 |

Webhook 仍是至少一次投递。lease 解决并发状态提交冲突和崩溃恢复，不试图把外部 HTTP 变成恰好一次；接收方仍须用稳定的 `Webhook-Id` 幂等。

## 3. SQL 复杂度证据

相同的 1000 event、单 endpoint、零延迟 `204` 接收端：

| 阶段 | 基线 SQL | 改进后 SQL | 基线 SQL/event | 改进后 SQL/event |
| --- | ---: | ---: | ---: | ---: |
| 紧循环展开 | 11,005 | 22 | 11.01 | 0.02 |
| 紧循环投递 | 7,020 | 60 | 7.02 | 0.06 |
| 生产循环端到端 | 18,040 | 80 | 18.04 | 0.08 |

改进后生产循环的 80 次 SQL 由以下调用精确组成：

| SQL | 调用 |
| --- | ---: |
| event lease 领取 | 5 |
| endpoint 列表 | 5 |
| 批量完成展开 | 5 |
| delivery lease 领取 | 20 |
| 批次授权状态 | 25（展开 5 + 投递 20） |
| 批量完成投递 | 20 |
| 合计 | 80 |

这项结果与 CPU 频率、磁盘型号无关，证明数据库往返复杂度已经从按事件多次调用，变为按批次及批次内不同授权组合调用。多用户、多应用场景的授权调用数取决于批次内不同授权组合数，不会永远固定为 25，因此生产容量仍需用真实分布复测。

完整逐 SQL 数据见 [`webhook-worker-sql-profile.md`](webhook-worker-sql-profile.md) 和 [`webhook-worker-sql-profile-after.md`](webhook-worker-sql-profile-after.md)。

## 4. 吞吐与主站隔离样本

未开启 profiler 的相同本机样本：

| 紧循环阶段 | 基线 | 改进后 | 相对倍率 |
| --- | ---: | ---: | ---: |
| outbox 展开 1000 event | 816.2ms | 83.3ms | 9.8x |
| HTTP 投递并完成 1000 delivery | 213.8ms | 165.8ms | 1.3x |
| 展开加投递 | 1.03s | 249.1ms | 4.1x |

建议预算样本使用主站 20 连接、Worker 独立 2 连接和 `3000ms` 活动间隔。`worker-running` 相对相邻的 `enqueue-only` 配对吞吐变化中位数为 `+2.4%`，四轮都没有请求失败；短样本没有观察到 Worker 造成的额外负向吞吐，但正值属于整机波动，不能解释为 Worker 提升了主站性能。

生产循环的 `82.0 events/s` 包含刻意设置的 `3000ms` 协作等待，表示这个建议资源预算下的行为，不表示架构上限，也不应与基线 `500ms` 调度策略的 `104.7 events/s` 直接比较。紧循环结果用于观察实现上限，预算循环用于观察资源隔离，两者用途不同。

完整轮次见 [`webhook-load-test.md`](webhook-load-test.md) 和 [`webhook-load-test-after.md`](webhook-load-test-after.md)。

## 5. CPU、block 与 mutex 复测

在正式四模式压测上同时采集 CPU、block 和 mutex profile。采样会影响绝对吞吐，因此只用于确认热点是否转移：

- 聚焦 `internal/service/webhook` 的 CPU flat 样本为 `0.05s`，约占整个并发压测 CPU 的 `0.014%`；没有出现新的 Worker CPU 热点。
- Worker block 中约 `12.01s` 位于 `waitAfterWork`，与 4 次 `3s` 建议预算等待吻合，是主动资源整形，不是锁或数据库阻塞。
- 聚焦 Worker 的 mutex 累计等待约 `28.6ms`；新增批内授权合并本身约 `4.4ms`，没有形成锁竞争瓶颈。

复现命令：

```powershell
$env:WEBHOOK_LOADTEST_ENABLE='1'
$env:WEBHOOK_LOADTEST_WORKER_DB_MAX_CONNECTIONS='2'
$env:WEBHOOK_LOADTEST_WORKER_ACTIVE_INTERVAL='3s'
go test ./cmd/loadtest -run TestWebhookLoadImpact -count=1 -v `
  -cpuprofile=cpu.pprof -blockprofile=block.pprof -mutexprofile=mutex.pprof
```

逐 SQL profile：

```powershell
$env:WEBHOOK_SQL_PROFILE_ENABLE='1'
$env:WEBHOOK_LOADTEST_WORKER_DB_MAX_CONNECTIONS='2'
$env:WEBHOOK_LOADTEST_WORKER_ACTIVE_INTERVAL='3s'
go test ./cmd/loadtest -run TestWebhookWorkerSQLProfile -count=1 -v
```

## 6. 配置建议与生产校准

建议起点：

```yaml
webhook_worker:
  max_database_connections: 2
  active_interval_ms: 3000
```

这两个值表达的是主站优先的初始资源预算，而不是本机 profile 算出的最优参数。生产校准应同时观察：主站数据库池等待、PostgreSQL CPU/I/O、Webhook backlog 年龄、领取批次耗时、投递成功延迟、第三方 P95/P99 响应时间和重试率。若积压延迟超出目标，先在数据库余量允许范围内降低活动间隔；确认连接池等待仍可接受后再增加 Worker 连接或实例。任何调整都应在接近生产的网络与授权主体分布下复测。

本轮没有修改业务事务内的订阅存在性检查。其约 `7.6%` 的本机相对成本仍需 PostgreSQL `EXPLAIN (ANALYZE, BUFFERS, WAL)` 和长时间 soak test 证据，不能从 Worker profile 推断优化方式。

## 7. 测试与覆盖率

- `go vet ./...`：通过。
- `go test -race -p 2 ./internal/database/webhook ./internal/service/webhook`：通过。
- 全仓 `go test ./...` 所有包按 database、service、httpapi、cmd/app/integration/基础包四组执行：全部通过。分组是为了绕开桌面命令的 10 分钟外层限制，包范围与全仓命令一致。
- 本次变更相关包合并 statement coverage：`82.1%`。

| 包 | statement coverage |
| --- | ---: |
| `internal/config` | 92.0% |
| `internal/database/oauth` | 67.8% |
| `internal/database/webhook` | 83.2% |
| `internal/service/webhook` | 85.1% |
| `cmd/loadtest` | 97.1% |

覆盖率来自真实业务、错误、lease 过期、陈旧 token、并发领取、撤权重检和批量完成路径，没有添加只为提高数字的空断言测试。
