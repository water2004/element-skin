# 阶段 1：配置/密钥脱离仓库 + 启动校验 + CORS 收紧 + 修 favicon

## 目标

1. 让含敏感信息的 `config.yaml`（DB 密码、JWT 密钥）**不再进入 git 和 Docker 镜像**。
2. 启动时**校验 JWT 密钥不是默认值**且长度足够，否则拒绝启动（fail-fast）。
3. 把 `CORS allow_origins:["*"] + allow_credentials:true` 收紧为可控白名单。
4. 修复 `entrypoint.sh` 中**实测未生效**的 favicon 保护逻辑。

> 决策前提：JWT 密钥继续**只从配置文件读取**，不引入环境变量覆盖。本阶段不动读取机制。

## 问题证据

- `config.yaml` 当前被 git 跟踪（`git ls-files` 命中），内含：
  - `jwt.secret: "dev-secret-please-change-..."`（`config.yaml:7`）
  - `database.dsn: "postgresql://postgres:12345678@..."`（`config.yaml:16`）
- `Dockerfile` 末段 `COPY skin-backend/ .` 会把 `config.yaml` 一并打入镜像。
- `utils/jwt_utils.py:9` 直接取默认值，无任何校验：
  ```python
  JWT_SECRET = config.get("jwt.secret", "dev-secret-default-key-at-least-32-chars-long")
  ```
  若运营忘记改，伪造管理员 JWT 即可拿下后台。
- `routes_reference.py:61-69` + `config.yaml:37-40`：`allow_origins=["*"]` 与 `allow_credentials=True` 同时存在，Starlette 会反射任意 Origin 并允许携带凭证。
- `entrypoint.sh` favicon 段：业主已确认实测未生效（连续两次 `cp -rf` 逻辑相互覆盖，备份/还原时序错乱）。

## 改造清单

### 1.1 配置文件脱离仓库

- 新增 `skin-backend/config.example.yaml`：与现 `config.yaml` 结构一致，但**所有敏感值替换为占位说明**：
  ```yaml
  jwt:
    secret: "CHANGE_ME_TO_A_LONG_RANDOM_STRING"   # 必改，至少 32 字符
  database:
    dsn: "postgresql://USER:PASSWORD@HOST:5432/DBNAME?sslmode=disable"
  # ...其余非敏感项保留合理默认
  ```
- 从 git 移除真实 `config.yaml`，保留本地工作副本：
  ```bash
  git rm --cached skin-backend/config.yaml
  ```
- 根 `.gitignore` 增加：
  ```
  skin-backend/config.yaml
  ```
- `Dockerfile`：在 `COPY skin-backend/ .` 之前用 `.dockerignore` 排除 `config.yaml`，避免把真实配置烤进镜像。`skin-backend/.dockerignore` 增加一行：
  ```
  config.yaml
  ```
  运行时通过挂载卷提供 `config.yaml`（与现有 `private.pem`/`textures` 的挂载思路一致——二者已在 `.dockerignore` 中）。
- 在 `README` 或部署文档注明：首次部署需 `cp config.example.yaml config.yaml` 并填入真实值后挂载。

> 注意：本次提交无法消除**历史**中已泄露的密码/密钥。需提醒业主：`config.yaml` 进过仓库历史，相关 DB 密码与 JWT secret 应视为已泄露并**轮换**（改 DB 密码、换 JWT secret——后者会使存量登录态失效，属预期）。

### 1.2 启动校验 JWT 密钥

在配置加载后、应用启动前做一次性校验。放在 `config_loader.py` 末尾或 `routes_reference.py` 顶部初始化段。建议放在 `config_loader.py`，让所有引用方都受益：

```python
# config_loader.py，全局 config 实例化之后
_DEFAULT_JWT_SECRETS = {
    "dev-secret-default-key-at-least-32-chars-long",
    "dev-secret-please-change-to-a-very-long-string-in-production",
}

def _validate_critical_config(cfg: "Config") -> None:
    secret = cfg.get("jwt.secret", "")
    if not secret or secret in _DEFAULT_JWT_SECRETS or len(secret) < 32:
        raise RuntimeError(
            "jwt.secret 未配置、仍为默认值或长度不足 32 字符。"
            "请在 config.yaml 中设置一个足够长的随机字符串后再启动。"
        )

config = Config()
_validate_critical_config(config)
```

- 这样 `import config_loader` 即触发校验，开发态若想跳过可显式在本地 `config.yaml` 填一个合规随机串（开发也应如此）。
- `jwt_utils.py` 无需改动（仍 `config.get("jwt.secret", ...)`），但默认值已不可能在生产生效。

### 1.3 CORS 收紧

`config.yaml` / `config.example.yaml` 默认改为显式前端域名，不再 `["*"]`：

```yaml
cors:
  allow_origins: ["http://localhost:5173"]   # 部署时改为真实前端域名
  allow_credentials: true
```

`routes_reference.py` 增加一道保护：当 `allow_credentials` 为真且 `allow_origins` 含 `"*"` 时，要么报错要么自动降级（建议启动期报错，避免静默的错误配置）：

```python
cors_origins = config.get("cors.allow_origins", ["http://localhost:5173"])
cors_credentials = config.get("cors.allow_credentials", True)
if cors_credentials and "*" in cors_origins:
    raise RuntimeError("CORS 配置错误：allow_credentials=true 时 allow_origins 不能为 '*'。")
```

> 行为变化说明：这是**有意的安全收紧**。部署方必须在 `config.yaml` 填真实前端域名。文档需同步。

### 1.4 修复 entrypoint.sh favicon 逻辑

将 `entrypoint.sh` 中释放前端产物的段落简化为：先备份用户 favicon（若存在）→ 清理旧入口文件 → 复制新产物 → 还原用户 favicon。去掉重复 `cp -rf` 与自相矛盾的注释。参考实现：

```bash
# --- 释放前端编译产物 ---
echo "正在释放前端静态文件到 /app/frontend..."

# 1. 若用户已自定义 favicon，先备份
USER_FAVICON=""
if [ -f "/app/frontend/favicon.ico" ]; then
    USER_FAVICON="$(mktemp)"
    cp -f /app/frontend/favicon.ico "$USER_FAVICON"
    echo "检测到自定义 favicon.ico，将在释放后保留。"
fi

# 2. 清空除 static / favicon.ico 外的旧入口文件
if [ -d "/app/frontend" ]; then
    find /app/frontend -mindepth 1 -maxdepth 1 ! -name 'static' ! -name 'favicon.ico' -exec rm -rf {} +
fi

# 3. 复制新产物
cp -rf /app/frontend_dist/* /app/frontend/

# 4. 还原用户 favicon（覆盖 dist 里的默认值）
if [ -n "$USER_FAVICON" ]; then
    cp -f "$USER_FAVICON" /app/frontend/favicon.ico
    rm -f "$USER_FAVICON"
fi
```

## 影响文件

- 新增：`skin-backend/config.example.yaml`
- 修改：`.gitignore`（根）、`skin-backend/.dockerignore`、`config_loader.py`、`routes_reference.py`、`entrypoint.sh`
- git 操作：`git rm --cached skin-backend/config.yaml`

## 测试与验证

- `git status` 确认 `config.yaml` 不再被跟踪；`git ls-files | grep config.yaml` 无结果。
- 单测：给 `_validate_critical_config` 加用例——默认密钥/短密钥/空 → 抛 `RuntimeError`；合规随机串 → 通过。
- 启动校验：本地把 `jwt.secret` 改回默认值，确认进程拒绝启动并打印中文提示。
- CORS：`allow_origins:["*"]` + credentials 时启动报错；配置真实域名后，跨域预检 (`OPTIONS`) 仅对白名单 Origin 返回 `Access-Control-Allow-Origin`。
- favicon：构造「已存在自定义 favicon.ico」与「不存在」两种场景跑 `entrypoint.sh`（可在本地用临时目录模拟 `/app/frontend` 与 `/app/frontend_dist`），确认自定义 favicon 被正确保留。
- `pytest -q` 全绿（注意：测试 import `routes_reference` 会触发新校验，需确保测试用 `config.yaml` 的 `jwt.secret` 为合规随机串）。

## 风险与回滚

低风险。最大注意点：**新校验会让测试/CI 在 `jwt.secret` 不合规时无法启动**——这是预期行为，需同步更新测试环境的 `config.yaml`。各改动可独立 revert。
