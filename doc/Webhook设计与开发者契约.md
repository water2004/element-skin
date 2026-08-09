# Webhook 设计与开发者契约

## 1. 背景

Element Skin 面向公开应用和机密应用提供同一套第三方开发能力。Webhook 用于提示应用“某个资源发生了变化”，而不是替代 `/v2` API 传输完整资源。应用收到事件后，应使用自己的有效 OAuth access token 调用 API 获取当前状态。

Webhook 完全可选。应用不需要事件通知时不配置 endpoint，站点不会为该应用创建投递任务。OAuth `redirect_uri` 与 Webhook endpoint 也是两个独立概念：只使用 Device Code 或 Client Credentials 的应用可以不配置 OAuth 回调地址。

## 2. 目标

- 站点业务事务不执行第三方 HTTP 请求，只写入轻量、可恢复的 outbox 事件。
- 公开应用、机密应用、用户委托和 Client Credentials 使用同一套事件名称、信封、签名与重试契约。
- endpoint 可订阅事件必须受应用申请权限约束，投递前再次按当前有效权限检查。
- 事件只携带稳定标识和基础上下文，避免生成昂贵载荷和泄露不必要数据。
- 提供至少一次投递、幂等标识、超时、退避重试、SSRF 防护和有界资源占用。

## 3. 非目标

- 不保证恰好一次投递；接收方必须自行去重。
- 不保证不同事件或同一资源事件的全局投递顺序。
- 不把 Webhook 作为资源快照、审计日志或消息队列消费接口。
- 不允许 endpoint 指向内网、回环、链路本地地址，也不跟随 HTTP 重定向。
- 不为事件名加入 `.v1` 或额外的 aspect 层级。

## 4. 事件目录与命名

事件名固定使用 `<resource>.<action>` 两段格式。resource 与权限目录中的资源概念保持一致；action 使用过去式状态变化。事件目录可以新增类型，但已发布类型不会通过版本后缀改变语义。

当前目录如下：

| 事件 | 基础数据 | 可申请权限 |
| --- | --- | --- |
| `account.created` | `user_id` | `account.read.any` |
| `account.updated` | `user_id` | `account.read.self` 或 `account.read.any` |
| `account.deleted` | `user_id` | `account.read.any` |
| `oauth_grant.created` | `grant_id`、`user_id` | `oauth_grant.read.owned` |
| `oauth_grant.updated` | `grant_id`、`user_id` | `oauth_grant.read.owned` |
| `oauth_grant.revoked` | `grant_id`、`user_id` | `oauth_grant.read.owned` |
| `official_whitelist.added` | `username`、`endpoint_id` | `official_whitelist.read.any` |
| `official_whitelist.removed` | `username`、`endpoint_id` | `official_whitelist.read.any` |
| `permission.updated` | `user_id` | `permission.read.any` |
| `profile.created` | `profile_id`、`user_id` | `profile.read.owned` 或 `profile.read.any` |
| `profile.updated` | `profile_id`、`user_id` | `profile.read.owned` 或 `profile.read.any` |
| `profile.deleted` | `profile_id`、`user_id` | `profile.read.owned` 或 `profile.read.any` |
| `texture.created` | `texture_hash`、`texture_type`、`user_id` | `texture.read.owned` 或 `texture.read.any` |
| `texture.updated` | `texture_hash`、`texture_type`、`user_id` | `texture.read.owned` 或 `texture.read.any` |
| `texture.deleted` | `texture_hash`、`texture_type`、`user_id` | `texture.read.owned` 或 `texture.read.any` |

`user_id` 是站点用户 UUID。grant 事件同时提供 `grant_id`，便于应用关联自己的授权记录；公开应用和机密应用收到的字段一致。`account.created`、`account.deleted`、`permission.updated` 和 `official_whitelist.*` 是管理型事件，只能由机密应用使用对应的 app-only 权限订阅。普通用户委托应用以 `oauth_grant.revoked` 作为用户授权终止信号，不订阅 `account.deleted`。

事件目录 API：

```http
GET /v2/oauth/webhook-events
```

```json
{
  "events": [
    {
      "type": "profile.updated",
      "description": "用户角色发生变化",
      "required_permissions": ["profile.read.any", "profile.read.owned"],
      "delegated_permission": "profile.read.owned",
      "application_permission": "profile.read.any"
    }
  ]
}
```

`delegated_permission` 表示公开应用和机密应用均可通过用户 grant 使用的权限；`application_permission` 表示只有机密应用才能以 app-only actor 使用的权限。事件不支持某种模式时对应字段省略。`required_permissions` 是两种模式所需权限的有序并集，便于通用目录展示。

前端应使用该目录、应用类型和应用当前申请的权限动态计算可选事件，不维护另一份硬编码目录。

## 5. 应用与 endpoint API

创建或修改应用时，`webhook_endpoints` 是完整替换语义，最多包含 5 项。传入空数组或不传该字段表示应用不配置 Webhook。

```json
{
  "name": "Example integration",
  "client_type": "confidential",
  "redirect_uri": "",
  "permissions": ["profile.read.any"],
  "webhook_endpoints": [
    {
      "url": "https://hooks.example.com/element-skin",
      "enabled": true,
      "events": ["profile.created", "profile.updated", "profile.deleted"]
    }
  ]
}
```

修改已有 endpoint 时必须回传其 `id`；不在数组中的旧 endpoint 会被删除。URL 必须是公网 HTTPS 地址，同一应用内不能重复。每个 endpoint 至少选择一个当前应用权限允许的事件。

新建 endpoint 的响应会额外包含仅展示一次的 `signing_secret`：

```json
{
  "id": "wh_...",
  "url": "https://hooks.example.com/element-skin",
  "status": "active",
  "enabled": true,
  "events": ["profile.updated"],
  "created_at": 1786118400000,
  "updated_at": 1786118400000,
  "signing_secret": "..."
}
```

后续读取应用不会再次返回明文密钥。服务端只保存加密密文。删除 endpoint 后重新创建可生成新密钥。

## 6. 权限与授权语义

endpoint 的事件选择与应用申请权限是同一个上限：

- 创建或修改应用时，如果事件所需权限不在 `permissions` 中，请求返回 `400`。
- 应用权限被移除时，前端同步移除已经不再可选的事件；后端仍会拒绝越权配置。
- 应用未处于 `active`、endpoint 被停用或权限不再有效时，不执行投递。
- `*.read.owned` 资源事件要求事件所属用户对该应用存在有效 grant，且当前委托 actor 仍拥有对应读取权限。
- `*.read.any` 资源事件只适用于机密应用的有效 app-only actor，并同时受应用申请权限和管理员配置的 client 权限上限约束。
- `oauth_grant.*` 只投递给 grant 所属应用，不广播给其他应用。
- `account.created`、`account.deleted`、`permission.updated` 与 `official_whitelist.*` 只走机密应用的 app-only 权限路径；`account.updated` 同时支持用户委托和 app-only 路径。
- `permission.updated` 表示用户角色或直接权限覆盖发生有效变化，不在事件体中展开权限集合；接收方应读取用户权限 API 获取当前结果。

同一用户对同一应用最多存在一个 active grant。再次授权会更新这条逻辑授权，不会并存多个有效 grant；撤销后原 grant 保留用于历史和排错。

账号删除会把该用户仍处于 active 的 grant 作为 `oauth_grant.revoked` 定向发送给对应应用。后续清理已经 revoked 的历史 grant 不重复发送事件。因此用户委托应用不依赖无法读取的 `account.deleted` 也能可靠感知授权终止。

投递展开和真正发出 HTTP 请求前都会重新检查权限。因此，从产生事件到投递之间发生 grant 撤销、应用停用或权限收窄时，旧任务会停止而不是继续泄露事件。

## 7. HTTP 投递契约

站点使用 `POST` 发送 JSON，任意 `2xx` 都视为成功。请求最长等待 10 秒，不跟随重定向。

```http
POST /element-skin HTTP/1.1
Content-Type: application/json
Webhook-Id: evt_...
Webhook-Delivery: whd_...
Webhook-Timestamp: 1786118400123
Webhook-Signature: v1=...
```

```json
{
  "id": "evt_...",
  "type": "profile.updated",
  "created_at": 1786118399000,
  "data": {
    "user_id": "...",
    "profile_id": "..."
  }
}
```

字段含义：

- `id` / `Webhook-Id`：稳定事件 ID，接收方应以它作为业务幂等键。
- `Webhook-Delivery`：本 endpoint 的投递任务 ID，用于排错；重试时保持不变。
- `created_at`、`Webhook-Timestamp`：毫秒时间戳。
- `data`：只包含定位资源所需的基础字段，不保证资源此时仍然存在。

## 8. 签名验证

签名输入是 `Webhook-Timestamp + "." + 原始请求体字节`。服务端计算：

```text
v1=hex(HMAC-SHA256(signing_secret, timestamp + "." + raw_body))
```

接收方应：

1. 读取原始 body，不要先解析再重新序列化。
2. 拒绝与当前时间差距过大的时间戳，建议允许 5 分钟时钟偏差。
3. 使用恒定时间比较验证签名。
4. 在验证成功后按 `Webhook-Id` 幂等处理。
5. 尽快返回 `2xx`，耗时业务应进入接收方自己的异步队列。

Python 接收方可使用 `python-sdk` 的 `WebhookVerifier` 完成原始 body 验签、时间戳和事件结构校验。`ReplayGuard` 允许生产应用把已认证事件原子写入 durable inbox；`MemoryReplayGuard` 只用于测试和单进程示例。具体 API、FastAPI 与 Flask 示例见 `python-sdk/doc/Webhook接收.md`。

## 9. 重试、性能与保留

站点采用至少一次投递。网络错误、超时或非 `2xx` 响应使用指数退避重试：首次约 30 秒，单次间隔最多 6 小时，最多尝试 12 次且总投递年龄不超过 72 小时。接收方可能在“已处理请求但成功响应丢失”时收到重复事件。

站点业务事务只执行一次带索引的订阅存在性检查；没有任何有效订阅时不会创建 event。存在订阅时，事务只写一条不可变 outbox 快照，不按 endpoint 放大写入。独立 worker 再批量领取、展开和投递，因此慢 endpoint 不占用站点请求线程或主站数据库连接池。endpoint 查询和授权结论只在当前领取批次内按等价主体合并；下一投递批次仍会重新检查授权，不把撤权安全依赖于长期缓存。

Worker 使用独立数据库连接池，并提供 `webhook_worker.max_database_connections` 与 `webhook_worker.active_interval_ms` 两项资源预算。仓库建议起点为 2 个连接和有工作批次间 `3000ms`，目的是优先保护主站，不是特定机器上测出的生产最优值。持续积压时应结合数据库池等待、PostgreSQL CPU/I/O、backlog 年龄和第三方响应延迟调整，不能直接照搬本地吞吐数字。

成功和最终失败的事件保留 7 天后由 worker 分批清理。待投递和处理中任务不参与清理。Webhook 表不作为长期审计存储。

优化前 profile 确认 1000 个事件会执行 18,040 次 Worker SQL，并有逐事件短事务和固定调度等待。改为 lease 领取、批量完成以及批次内订阅/授权合并后，相同生产流程降至 80 次 SQL，即从 18.04 次/event 降到 0.08 次/event。未采样的本机零延迟样本中，紧循环展开加投递从 1.03 秒降至 249.1 毫秒，约 4.1 倍；该相对样本用于验证实现方向，不作为生产 SLA。

建议预算复测使用 50 并发、主站 20 连接、Worker 独立 2 连接和 `3000ms` 活动间隔。`worker-running` 相对相邻的 `enqueue-only` 配对吞吐变化中位数为 `+2.4%`，未观察到额外负向吞吐；正值属于短窗口波动，不能解释为 Worker 提升主站性能。优化前后完整证据分别见 `reports/webhook-performance-profile.md`、`reports/webhook-performance-profile-after.md`、`reports/webhook-load-test-after.md` 和 `reports/webhook-worker-sql-profile-after.md`。

## 10. 数据模型

- `webhook_endpoints`：endpoint URL、加密签名密钥、状态和所属 client。
- `webhook_endpoint_events`：endpoint 与事件类型的结构化订阅关系。
- `webhook_events`：事务内写入的不可变事件快照；`target_client_id`、`subject_user_id` 为结构化列，`data` 只保存不可变协议载荷；展开 lease 的到期时间和随机 token 用于并发领取与拒绝过期完成。
- `webhook_deliveries`：每个 event/endpoint 唯一任务、带随机 token 的 lease、尝试次数、下次时间和最后结果。

业务表触发器与业务变更处于同一 PostgreSQL 事务，事务回滚时 outbox 事件也回滚。账号事件忽略密码哈希变化和没有实际字段变化的 UPDATE；权限覆盖写入相同 effect、重复角色授予和重复白名单添加也不会生成事件。worker 通过 `FOR UPDATE SKIP LOCKED` lease 领取任务，支持多个 worker 实例并行运行和进程崩溃后的重新领取。

## 11. 前端交互

第三方应用列表只负责展示和导航。创建应用和修改应用分别进入独立页面，不使用弹窗。页面同时维护基本信息、应用类型、权限与可选 Webhook endpoints，并在权限或应用类型改变时立即收窄可选项。

`client_secret` 与 Webhook `signing_secret` 都必须在一次性响应区明确提示并支持复制，离开页面后不能假设可以再次读取。

## 12. 测试计划

- 事件目录名称、权限引用和资源/动作两段格式的精确测试。
- endpoint 创建、更新、停用、删除、密钥只显示一次和事务失败不留脏数据测试。
- 无订阅不写 event、outbox 展开幂等、lease 重领、完成、失败和清理测试。
- 用户委托、机密应用 app-only、grant 撤销及权限收窄后的投递重检测试。
- HMAC 头、原始载荷、`2xx`、非 `2xx`、超时与确定性退避测试。
- 私网地址、localhost、HTTP、重定向和 DNS 解析结果的 SSRF 防护测试。
- 前端 API exact request 测试、TypeScript 类型检查、生产构建以及桌面端和移动端表单检查。
- 独立压测同一 profile 写入在关闭触发器、无订阅、仅写 outbox、worker 同时运行四种模式下的吞吐与延迟；短预热后默认四轮轮换执行顺序。
- 以固定事件数和本机零延迟接收端分别测量排除等待的紧循环架构能力，以及包含可配置活动间隔和独立连接预算的生产 Worker 行为；优化前后报告分开保存，不能把建议预算吞吐当成生产最优值。

## 13. 待确认问题

当前契约没有阻塞实现的待确认项。后续若增加批量事件、endpoint 独立密钥轮换或开发者投递日志查询，应新增明确 API，不改变现有事件语义。
