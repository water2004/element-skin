# Webhook 与 OAuth 应用编辑覆盖率报告

测试日期：2026-08-08

## 1. 覆盖范围

本报告覆盖以下本次变更：

- 单用户、单应用唯一 active OAuth grant；
- Webhook 事件目录、endpoint 配置、outbox 与 delivery store；
- 独立 Webhook worker 的权限重检、签名、重试、死信、清理和 SSRF 防护；
- OAuth 应用 API wrapper；
- 第三方应用列表以及独立创建、修改页面；
- 权限变化对可选 Webhook 事件和提交 payload 的影响。
- OAuth 创建、读取、更新、删除与 grant 读取、撤销权限的独立组合。

## 2. 后端结果

全仓命令：

```powershell
go test -p 2 ./... -coverprofile=<system-temp>/element-skin-backend-full-coverage.out
go tool cover -func=<system-temp>/element-skin-backend-full-coverage.out
```

结果：全部包通过，全仓 statement coverage 为 **82.6%**。

本次相关包在全仓覆盖中的结果：

| 包 | Statements |
| --- | ---: |
| `internal/webhook` | 100.0% |
| `internal/database/webhook` | 78.4% |
| `internal/service/webhook` | 81.8% |
| `internal/httpapi/oauth` | 87.5% |
| `internal/service/oauth` | 77.4% |

使用 `-coverpkg` 让 HTTP、service 和 database 测试共同计入本次相关实现后，目标包组合 statement
coverage 为 **81.8%**。该结果使用 atomic profile，并按源码 block 合并各测试包的覆盖数据。关键函数结果包括：

| 函数 | Statements |
| --- | ---: |
| `prepareWebhookEndpoints` | 87.5% |
| `validateWebhookEventTypes` | 100.0% |
| `WebhookEventCatalog` | 100.0% |
| `DispatchBatch` | 84.0% |
| `DeliverBatch` | 86.4% |
| `deliver` | 80.5% |
| `endpointAuthorized` | 82.1% |
| `retryOrDead` | 80.0% |
| `retryDelay` | 90.9% |
| `sign`、`truncateDetail`、`IsPublicIP` | 100.0% |

`cmd/webhook-worker` 是信号、配置和依赖装配入口，没有独立业务分支，当前显示 0%；真正的 worker
行为位于 `internal/service/webhook`，覆盖率为 81.9%。

## 3. 前端结果与门禁

仓库新增可重复命令：

```powershell
npm run test:coverage:oauth
```

该命令包含两个门禁：

1. `src/api/oauth/apps.ts` 的 statements、branches、functions、lines 必须全部达到 **100%**；
2. OAuth 应用页面与表单状态的最低阈值为 statements 80%、branches 75%、functions 80%、lines 80%。

当前页面与表单状态结果：

| 指标 | 结果 | 阈值 |
| --- | ---: | ---: |
| Statements | 84.61% | 80% |
| Branches | 81.01% | 75% |
| Functions | 83.22% | 80% |
| Lines | 88.29% | 80% |

其中 `oauthAppFormState.ts` 的 statements、functions、lines 为 100%，branches 为 83.33%。Vue
页面使用真实 Vue Router 与 Element Plus 在 jsdom 中挂载，覆盖创建 payload、一次性 secret、读取编辑
数据、endpoint 增删、secret 轮换、重新提交、删除应用、grant 撤销和编辑页跳转。
新增权限矩阵还会精确断言只加载允许的应用或 grant 区块、隐藏无权操作，以及读取加删除权限下的
只读应用表单。

## 4. 精确行为与失败路径

新增或加强的测试明确断言：

- outbox 无订阅不写入，有订阅时事件字段精确，重复展开不产生重复 delivery；
- endpoint 与应用权限不匹配时返回精确 `400`，失败后 client 和 endpoint 均不落库；
- 用户委托 grant 撤销后，已展开任务在投递前变为 dead 且不发 HTTP；
- 机密应用只有在 app-only actor 当前有效时才能收到 `*.read.any` 事件；
- endpoint 在展开后停用时不投递；
- 非 `2xx` 使用确定性退避，第 12 次失败精确进入 dead；
- HMAC body 和所有 Webhook headers 精确匹配；
- localhost、私网、链路本地、组播地址和不安全 URL 被拒绝；
- 成功、失败和清理状态字段、尝试次数、时间与 HTTP 状态均精确断言；
- 并发邮箱修改只允许一个成功，另一个请求只能返回验证码已消费或邮箱已占用这两种明确竞争结果，
  最终数据库仍必须恰好一个目标邮箱和一个原邮箱。该测试连续执行 10 次通过。

## 5. 最终验证

```text
go test -p 2 ./...                    PASS
go test -p 2 ./... -coverprofile=...  PASS, 82.6% statements
npm run test                          PASS, 24 files / 283 tests
npm run build                         PASS
npm run test:coverage:oauth           PASS
```
