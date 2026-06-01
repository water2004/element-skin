# 阶段 6：运维硬化（OAuth state 存储 + 日志化 + 依赖锁定 + 单实例约束）

## 目标

收尾性的可维护性与运维硬化，无对外行为变更：

1. 微软 OAuth `state` 不再裸用模块级 dict（内存泄漏 + 多进程失效）。
2. 用 `logging` 取代散落的 `print`。
3. 依赖锁定与清理（移除未用、锁版本）。
4. 明确「当前架构仅支持单实例」的约束，写进文档。

## 问题证据

- `routers/microsoft_routes.py:21` `oauth_states = {}`：
  - 过期项**仅在被再次访问时**清理（`time.time() > expires_at` 才 `del`），未被回调命中的 state 永久驻留 → 缓慢内存泄漏。
  - 多 worker / 重启即丢失，OAuth 流程跨进程失败。代码注释已自认「生产应使用 Redis」。
- 多处 `print(...)` 当日志：
  - `config_loader.py:23`、`database_module/main.py:26`（阶段 5 已改为抛错）、`backends/yggdrasil_backend.py:364`、`backends/microsoft_backend.py:348`、`backends/profile_import_backend.py:67,78`、`backends/yggdrasil_client.py:103`、`utils/email_utils.py:30,86`、`routes_reference.py:121-124`。
  - `FallbackBackend` 已用 `logging.getLogger("yggdrasil.fallback")`——是好范例，其余应对齐。
- `requirements.txt`：
  - 含 `requests>=2.31.0`，但出站请求已全用 `aiohttp`（grep 确认无 `import requests` 于运行路径）——死依赖。
  - 全部用 `>=` 下限，无上限/锁定，构建不可复现。
- 多份内存缓存（`SettingModule._cache`、`FallbackModule` 三个 cache、`RateLimiter._attempts`、`oauth_states`）均进程内，无跨进程同步——**当前实现隐含「单实例」假设**，但未在任何文档中言明。

## 设计决策

- OAuth state：**抽象出一个 `StateStore` 接口**，默认实现为「带 TTL 主动清理的内存版」（修复泄漏、保持零外部依赖），并预留 Redis 实现位。不强制上 Redis（与单实例现状一致），但让未来切换无需改路由逻辑。
- 日志：引入一个集中的 logging 配置，模块各取 `logging.getLogger(__name__)`。日志级别由 `server.debug` 驱动（已有该配置项）。
- 依赖：删 `requests`；对直接依赖加**兼容上限**或在文档中说明用 lockfile（如 `pip-compile` 生成 `requirements.lock`）。最小改动版：删死依赖 + 给关键库加上限。

## 改造清单

### 6.1 OAuth state 存储

新增 `utils/state_store.py`：

```python
import time
from typing import Any, Optional

class InMemoryStateStore:
    """带 TTL 的内存 state 存储；惰性 + 周期性清理。
    注意：仅适用于单实例部署。多实例需替换为 Redis 实现。"""
    def __init__(self):
        self._data: dict[str, tuple[float, Any]] = {}

    def put(self, key: str, value: Any, ttl_seconds: int) -> None:
        self._data[key] = (time.time() + ttl_seconds, value)
        self._sweep()

    def pop(self, key: str) -> Optional[Any]:
        item = self._data.pop(key, None)
        if not item:
            return None
        expires_at, value = item
        if time.time() > expires_at:
            return None
        return value

    def _sweep(self) -> None:
        now = time.time()
        expired = [k for k, (exp, _) in self._data.items() if now > exp]
        for k in expired:
            self._data.pop(k, None)
```

`routers/microsoft_routes.py` 用它替换裸 dict：
- `oauth_states[state] = {...}` → `store.put(state, {...}, ttl_seconds=600)`
- 取用处 `if state not in oauth_states / 过期判断 / del` → `data = store.pop(state); if not data: raise 400`
- 同理替换 `temp_token` 与 `ms_token` 的存取（它们也用同一 store）。

> 语义保持不变：一次性、带过期。`pop` 取出即删，天然满足「使用后立即删除」。

### 6.2 日志化

新增集中配置（`routes_reference.py` 启动段或新建 `utils/logging_config.py`）：

```python
import logging

def setup_logging(debug: bool):
    logging.basicConfig(
        level=logging.DEBUG if debug else logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )
```

`routes_reference.py` 在初始化早期调用 `setup_logging(config.get("server.debug", False))`。

各模块把 `print(...)` 改为：

```python
import logging
logger = logging.getLogger(__name__)
# ...
logger.warning("Texture processing error: %s", e)     # 替换 print
```

替换点：`backends/yggdrasil_backend.py`、`backends/microsoft_backend.py`、`backends/profile_import_backend.py`、`backends/yggdrasil_client.py`、`utils/email_utils.py`、`config_loader.py`。`routes_reference.py:__main__` 的启动 print 可保留或改 logger（无伤大雅）。

> 注意：异常详情进日志（服务端可见），不外泄给客户端——与阶段 3.4 一致。

### 6.3 依赖锁定与清理

`requirements.txt`：
- 删除 `requests>=2.31.0`（确认运行路径无 `import requests`；测试若用到 httpx 而非 requests，可一并核对）。
- 如阶段 2 采用 numpy 方案，加 `numpy`。
- 给关键库补兼容上限或改用锁文件。最小改动示例：
  ```
  fastapi>=0.95.0,<1.0
  uvicorn[standard]>=0.22.0,<1.0
  asyncpg>=0.29.0,<0.31
  Pillow>=10.0.0,<12.0
  PyJWT>=2.8.0,<3.0
  cryptography>=41.0.0
  bcrypt>=4.0.0,<5.0
  ...
  ```
  推荐做法：保留 `requirements.txt` 为顶层声明，另用 `pip-compile` 产出 `requirements.lock` 供镜像构建使用（`Dockerfile` 改装 lock）。本阶段可先做「删死依赖 + 加上限」，lockfile 作为可选增强。

### 6.4 单实例约束文档

在 `docs/security/README.md` 或新建 `docs/deployment-notes.md` 注明：

> 当前后端在进程内维护多份缓存（settings、fallback、限流计数、OAuth state）。这些缓存**不跨进程共享**，因此后端**只能以单进程/单实例运行**（uvicorn 单 worker）。若需水平扩展或多 worker，必须先将这些状态外置（Redis 等），相关接口已通过 `StateStore` 抽象预留扩展点；限流与 settings 缓存的外置改造尚未完成。

并据此确认 `Dockerfile` CMD 未指定 `--workers >1`（当前未指定，默认 1，符合约束）。

## 影响文件

- 新增：`utils/state_store.py`、（可选）`utils/logging_config.py`、（可选）`docs/deployment-notes.md`
- 修改：`routers/microsoft_routes.py`（用 StateStore）、各含 `print` 的模块、`requirements.txt`、`routes_reference.py`（setup_logging）
- 文档：`docs/security/README.md` 补单实例约束

## 测试与验证

- StateStore 单测：put/pop 正常；过期后 pop 返回 None；`_sweep` 清理过期项（put 多个短 TTL 后再 put，断言过期项被移除）。
- OAuth 流程回归：`tests/api/test_microsoft_import_api.py` 仍通过；state 一次性、过期失效行为不变。
- 日志：本地以 `debug:true` / `false` 启动，确认日志级别与格式生效，原 `print` 信息出现在 logger 输出。
- 依赖：在干净虚拟环境 `pip install -r requirements.txt` 成功；`grep -rn "import requests" skin-backend --include=*.py` 无运行路径命中。
- `pytest -q` 全绿。

## 风险与回滚

低风险，纯内部重构与配置整理，无对外契约变化。StateStore 替换需逐一核对 microsoft_routes 的三处用法（state / temp_token / ms_token）语义一致。各子项独立可回滚。
