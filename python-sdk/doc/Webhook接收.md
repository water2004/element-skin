# Webhook 接收与验签

Element Skin Webhook 只携带事件类型、用户 UUID 和资源 ID 等基础信息。接收方验签并持久接收事件后，应使用自己的有效 OAuth access token 调用 `/v2` API 获取资源当前状态。

## 初始化验证器

应用创建 Webhook endpoint 时会获得只展示一次的 `signing_secret`。每个 endpoint 应使用自己的验证器：

```python
from element_skin_sdk import WebhookVerifier

verifier = WebhookVerifier(
    signing_secret="endpoint-signing-secret",
    tolerance_seconds=300,
    max_body_bytes=65_536,
)
```

`tolerance_seconds` 默认是 300 秒，同时限制过旧和过度超前的请求时间戳。`max_body_bytes` 默认限制原始请求体为 64 KiB，防止接收端为异常大请求分配和计算无界资源。生产机器需要保持可靠的时钟同步，HTTP server 或反向代理也应配置相应请求体上限。

## 验证原始请求

必须传入框架读取到的原始 body 字节，不能先解析 JSON 再序列化。`request_headers` 可以是普通 dict，也可以是 FastAPI/Starlette 或 Flask/Werkzeug 提供的、具有 `.items()` 的 headers 集合：

```python
event = verifier.verify(raw_body, request_headers)

print(event.id)           # 稳定事件 ID，业务幂等键
print(event.delivery_id)  # 当前 endpoint 的投递任务 ID
print(event.type)         # 例如 profile.updated
print(event.created_at)   # 事件创建时间，毫秒
print(event.timestamp)    # 本次签名时间，毫秒
print(event.data)         # 用户 UUID、资源 ID 等基础字段
```

`verify()` 会依次验证：

- `Webhook-Id`、`Webhook-Delivery`、`Webhook-Timestamp`、`Webhook-Signature` 请求头；
- `v1=HMAC-SHA256(signing_secret, timestamp + "." + raw_body)`，并使用恒定时间比较；
- 毫秒时间戳是否处于允许窗口；
- JSON 事件结构及请求头与 body 中的事件 ID 是否一致。

验证失败会抛出 `WebhookError` 的细分异常：

| 异常 | 含义 |
| --- | --- |
| `WebhookHeaderError` | 请求头缺失、为空、重复或类型非法 |
| `WebhookSignatureError` | 签名格式或 HMAC 不正确 |
| `WebhookTimestampError` | 时间戳格式错误或超出容差 |
| `WebhookPayloadError` | 已验签 body 不符合事件结构 |
| `WebhookReplayError` | delivery 已被重放存储原子领取 |

不要在日志中记录 `signing_secret`、完整 Authorization header 或其他凭证。

## 重放与业务幂等

时间戳窗口只拒绝过旧请求，不能单独阻止窗口内重放。SDK 提供两种使用方式：

```python
# 只验签和解析，不写任何重放状态
event = verifier.verify(raw_body, headers)

# 验签后调用可插拔 ReplayGuard 原子接收事件
event = verifier.verify_and_claim(raw_body, headers, replay_guard)
```

`ReplayGuard.claim(event, expires_at_ms)` 会收到完整的已认证事件。生产实现应把 claim 与 durable inbox 写入做成同一个原子操作：首次接收返回 `True`，已有 delivery 返回 `False`。`expires_at_ms` 是签名重放键的最短保留边界；业务 inbox 和 `event.id` 幂等记录通常需要保留更久。只有在事件已经可靠入队或已处理时，接收方才应对重复请求返回 `2xx`。

`MemoryReplayGuard` 是线程安全的进程内实现，只适合测试、开发和明确的单进程场景。它不跨进程、不跨实例，也不会代替数据库或 Redis 的原子 inbox。即使使用 replay guard，业务处理仍应以 `event.id` 作为幂等键，因为 Webhook 契约是至少一次投递。

## HTTP 处理纪律

推荐接收顺序：

1. 读取原始 body。
2. 验签并检查时间戳。
3. 将已认证事件原子写入 durable inbox，或确认它已经存在。
4. 尽快返回 `2xx`。
5. 后台消费者按 `event.id` 幂等处理，并通过 `/v2` API 读取当前资源。

不要在 Webhook HTTP 请求中同步执行慢查询或长事务。非 `2xx`、网络错误和超时会触发站点指数退避重试。

## 框架示例

- FastAPI 依赖：`pip install -e '.[webhook-fastapi]'`
- Flask 依赖：`pip install -e '.[webhook-flask]'`
- [FastAPI 示例](../demo/webhook_fastapi.py)
- [Flask 示例](../demo/webhook_flask.py)

两个示例使用 `MemoryReplayGuard` 展示 API，生产环境应替换为持久化实现。完整事件目录、权限关系和投递契约见仓库根目录的《Webhook 设计与开发者契约》。
