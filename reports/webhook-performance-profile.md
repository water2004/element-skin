# Webhook 性能 Profile 报告

## 1. 结论

本次只采集当前实现的基线，没有修改 Worker 行为。Profile 能证明三个主要事实：

1. 单 Worker 的首要持续吞吐限制是固定 `500ms` 调度等待。1000 个事件的生产轮询耗时 `9.55s`，block profile 中 `Worker.Run` 有 `8.42s` 累计阻塞在调度选择上。
2. Outbox 展开和投递存在确定的 SQL 调用放大。1000 个事件、一个 endpoint 的生产循环共执行 `18,040` 次 Worker SQL，即 `18.04` 次/event；其中权限与授权查询为 `12,000` 次。
3. Worker 自身的 Go mutex 不是瓶颈。聚焦 `service/webhook` 的 mutex profile 只有约 `0.15s` 累计等待，远低于调度和数据库路径。

因此，后续优化应优先处理自适应排空、展开阶段批量权限计算和批量完成事务，不应先做锁优化，也不应在未解决重复展开领取前直接增加 Worker 数量。

## 2. 采样方法

CPU、block、mutex 和 execution trace 使用正式四模式压测采集：

```powershell
$env:WEBHOOK_LOADTEST_ENABLE='1'
$env:WEBHOOK_LOADTEST_CONCURRENCY='50'
$env:WEBHOOK_LOADTEST_DURATION='3s'
$env:WEBHOOK_LOADTEST_REPEATS='4'
$env:WEBHOOK_LOADTEST_EVENTS='1000'
$env:WEBHOOK_LOADTEST_DB_MAX_CONNECTIONS='20'
go test ./cmd/loadtest -run TestWebhookLoadImpact -count=1 -v `
  -cpuprofile=cpu.pprof `
  -blockprofile=block.pprof -blockprofilerate=1 `
  -mutexprofile=mutex.pprof -mutexprofilefraction=1 `
  -trace=trace.out
```

Worker SQL 使用独立 5 连接池上的测试专用 pgx `QueryTracer` 采集：

```powershell
$env:WEBHOOK_SQL_PROFILE_ENABLE='1'
$env:WEBHOOK_SQL_PROFILE_EVENTS='1000'
go test ./cmd/loadtest -run TestWebhookWorkerSQLProfile -count=1 -v
```

逐 SQL 原始汇总见 [`webhook-worker-sql-profile.md`](webhook-worker-sql-profile.md)。

全量 trace、block 和 mutex 采样会显著影响绝对吞吐，因此这次 profile 只用于热点归因，不把采样期间的 req/s 与未采样压测直接比较。SQL 累计时间按调用求和，并发调用会重叠，也不能直接当作墙钟时间。

## 3. Go Profile 证据

### 3.1 调度等待

| 指标 | 结果 |
| --- | ---: |
| 生产轮询处理 1000 个事件 | 9.55s |
| block profile 中 `Worker.Run` 调度阻塞 | 8.42s |
| 当前投递批次/轮询间隔 | 50 / 500ms |
| 零延迟接收端持续吞吐 | 约 104.7 events/s |

实测吞吐与 `50 / 0.5s = 100 events/s` 的调度上限一致。紧循环排除固定等待后，同一实现的组合吞吐约为 `970.9 events/s`，因此“有积压时连续排空、空闲时才等待”有直接证据支持。

### 3.2 CPU 调用栈

| 调用路径 | 累计 CPU | 相对父路径 |
| --- | ---: | ---: |
| `Worker.DispatchBatch` | 3.07s | 100% |
| `DispatchBatch → endpointAuthorized` | 1.70s | 55.4% |
| `DispatchBatch → CompleteExpansion` | 1.08s | 35.2% |
| `Worker.deliver` | 1.03s | 100% |
| `deliver → endpointAuthorized` | 0.74s | 71.8% |

PostgreSQL 服务端执行时间不计入 Go CPU，所以这里不能单独用 CPU 数字判断数据库总成本；它与下面的 SQL 调用 profile 结合后，才能确认热点来自权限路径和逐事件完成事务。

### 3.3 Mutex

聚焦 Worker 调用栈的 mutex 累计等待约 `0.15s`，主要来自 pgx pool 资源释放。没有发现 Worker 自身共享 map、队列或 HTTP 并发控制上的锁热点，因此不安排锁结构优化。

## 4. SQL Profile 证据

### 4.1 阶段汇总

| 阶段 | 墙钟时间 | SQL 调用 | SQL/event | SQL 累计时间 |
| --- | ---: | ---: | ---: | ---: |
| 紧循环展开 | 929.3ms | 11,005 | 11.01 | 832.7ms |
| 紧循环投递 | 402.6ms | 7,020 | 7.02 | 961.2ms |
| 生产轮询端到端 | 9.55s | 18,040 | 18.04 | 2.19s |

紧循环展开是串行执行，SQL 累计时间占其墙钟时间约 `89.6%`。紧循环投递使用 10 个 goroutine，并发 SQL 会重叠，因此其累计时间大于墙钟时间是预期行为。

### 4.2 生产轮询中的 SQL 构成

| 类别 | 调用 | 调用占比 | 累计时间 | 时间占比 |
| --- | ---: | ---: | ---: | ---: |
| 权限与授权：client permission、active grant、subject、role、override、delegation policy | 12,000 | 66.5% | 1.20s | 54.7% |
| 展开完成事务：begin、delivery insert、event update、commit | 4,000 | 22.2% | 453.4ms | 20.7% |
| 成功投递状态更新 | 1,000 | 5.5% | 353.9ms | 16.1% |
| 查询订阅 endpoint | 1,000 | 5.5% | 111.1ms | 5.1% |
| 领取 event/delivery 批次 | 40 | 0.2% | 76.7ms | 3.5% |

权限相关的六类 SQL 在 1000 个事件中各执行 `2000` 次：展开前一次、真正投递前再一次。投递前的重检是安全契约，必须保留；展开阶段可以在批次内按 `(event_type, user_id, client_id)` 合并计算，因为投递阶段仍会阻止 grant 撤销或权限收窄后的泄露。

每个事件还单独执行一次 `begin → insert delivery → update event → commit`。这证明批量 `CompleteExpansions` 能直接减少约 4000 次数据库往返，而不是基于代码外观的猜测。

## 5. 已证明与尚未证明

已证明：

- 固定轮询是单 Worker 持续吞吐的主要上限。
- 权限路径存在按事件重复的 SQL 放大。
- 逐事件完成展开产生大量短事务。
- Worker mutex 不是主要问题。

尚未证明：

- 无订阅触发器检查内部究竟消耗在哪条 PostgreSQL 计划；目前只有约 `-7.4%` 的宏观差异，需要单独采集 `EXPLAIN (ANALYZE, BUFFERS, WAL)` 后才能修改。
- 多 Worker 的线性扩展能力。当前 event 展开没有 lease claim，先并发扩容会重复计算，必须先修正领取语义。
- 真实第三方网络、TLS 和多 endpoint fan-out 下的最终投递能力。

## 6. Profile 支持的优化顺序

1. Worker 有积压时连续运行批次，只在无工作时等待；保持连接池和 HTTP 并发硬上限。
2. 为 event 展开增加可过期 lease 和 `SKIP LOCKED` claim，消除多 Worker 重复展开。
3. 在展开批次内合并 endpoint、client permission、grant 和 delegated actor 查询；投递前继续逐条实时重检。
4. 使用一个批量事务插入 deliveries 并更新一批 event 的 `expanded_at`。
5. 用完全相同的 CPU、block、mutex、SQL profile 和负载参数复测，再决定是否需要增加 Worker 实例。

主站触发器优化不进入这一轮，等待 PostgreSQL 执行计划证据。
