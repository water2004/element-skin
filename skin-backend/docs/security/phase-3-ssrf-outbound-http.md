# 阶段 3：SSRF / 出站 HTTP 加固

## 目标

阻止认证用户借「导入材质/远程角色」让服务器去请求**任意 URL**（内网、云元数据、本地端口）。并停止把原始异常细节回显到前端 URL。

## 问题证据

- `utils/http.py:download_texture(url)`：对传入 URL 无任何限制，直接 `aiohttp GET`。
- 用户可控的 URL 来源：
  - `routers/site_routes.py:123` `/remote-ygg/import-profile`：`body.get("api_url")` → `ProfileImportBackend` → `YggdrasilClient(api_url)`（`backends/yggdrasil_client.py`，对 `api_base_url` 直接拼接并请求），以及随后 `download_texture(skin_url)`，而 `skin_url` 来自远程站返回，间接可控。
  - `routers/microsoft_routes.py:119-136` `/microsoft/import-profile`：`data.get("skin_url")` / `data.get("cape_url")` **完全前端可控**，直接进 `microsoft_backend._import_texture` → `download_texture`。
- 攻击面：`http://169.254.169.254/...`（云元数据/凭证）、`http://127.0.0.1:port`、`http://10.x/192.168.x/172.16-31.x`（内网探测）、`file://` 等。无协议白名单、无内网封禁、无重定向限制。
- `routers/microsoft_routes.py:78-80`：`error_msg = urllib.parse.quote(str(e)...)` 把原始异常塞进重定向 URL，可能泄露内部细节。

## 设计决策

集中式 URL 守卫，**所有出站请求前**统一校验。新增 `utils/url_guard.py`，对外暴露：
- `validate_outbound_url(url) -> str`：校验协议、解析主机、解析 DNS 到 IP 后逐个判断是否私有/保留地址；不通过抛 `ValueError`。
- 一个「安全的 fetch」封装，禁用或限制重定向（避免「先返回公网 302、再跳内网」绕过）。

> 关于域名白名单 vs 内网黑名单：Microsoft 导入的 `skin_url` 实际来自 `textures.minecraft.net` 等已知域，可叠加可选白名单；但远程 Yggdrasil 导入面向任意自建站，无法预知域名。因此**以「内网/保留 IP 黑名单 + 仅 https/http + 限制重定向」为基线**，对 Microsoft 路径额外可选收紧到已知皮肤域名。

## 改造清单

### 3.1 新增 URL 守卫

```python
# utils/url_guard.py
import ipaddress
import socket
from urllib.parse import urlparse

_ALLOWED_SCHEMES = {"http", "https"}

def _is_blocked_ip(ip: str) -> bool:
    addr = ipaddress.ip_address(ip)
    return (
        addr.is_private or addr.is_loopback or addr.is_link_local
        or addr.is_reserved or addr.is_multicast or addr.is_unspecified
    )

def validate_outbound_url(url: str) -> str:
    """校验出站 URL：仅 http(s)，且主机解析出的所有 IP 均非私有/保留。
    不通过抛 ValueError。"""
    parsed = urlparse(url)
    if parsed.scheme not in _ALLOWED_SCHEMES:
        raise ValueError("URL scheme not allowed")
    host = parsed.hostname
    if not host:
        raise ValueError("URL host missing")
    try:
        infos = socket.getaddrinfo(host, parsed.port or (443 if parsed.scheme == "https" else 80))
    except socket.gaierror:
        raise ValueError("URL host cannot be resolved")
    for info in infos:
        ip = info[4][0]
        if _is_blocked_ip(ip):
            raise ValueError("URL resolves to a blocked address")
    return url
```

> 说明：这是「解析时校验」。严格的 DNS-rebinding 防护需在连接时绑定已校验 IP（aiohttp 可通过自定义 connector/resolver 实现）。基线版先用 getaddrinfo 校验 + 禁重定向，能挡住绝大多数实战利用；如需更强，记为后续增强项。

### 3.2 接入 download_texture

`utils/http.py`：

```python
from utils.url_guard import validate_outbound_url

async def download_texture(url: str) -> bytes:
    validate_outbound_url(url)
    timeout = aiohttp.ClientTimeout(total=15)
    async with aiohttp.ClientSession(timeout=timeout) as session:
        # 禁止自动重定向，避免公网→内网跳转绕过校验
        async with session.get(url, allow_redirects=False) as resp:
            if resp.status == 200:
                return await resp.read()
            raise Exception(f"Failed to download texture from {url} (status {resp.status})")
```

> 注意：禁用重定向后，若目标站确实依赖 3xx（少见于直链材质），需改为「手动跟随有限次且每跳重新校验」。先用禁用版，遇到实际需要再放宽为受控跟随。

### 3.3 接入 api_url（远程 Yggdrasil 导入）

`backends/yggdrasil_client.py` 构造时校验 base URL，且其内部 `authenticate` / `get_profile_with_textures` 的 `session.get/post` 也应禁用重定向：

```python
from utils.url_guard import validate_outbound_url

class YggdrasilClient:
    def __init__(self, api_base_url: str):
        validate_outbound_url(api_base_url)
        self.api_base_url = api_base_url.rstrip("/") + "/"
        ...
```

`profile_import_backend` 在用 `api_url` 前已隐式经此构造校验；若有更早使用点，同样前置校验。

### 3.4 收敛异常回显

`routers/microsoft_routes.py` 回调异常分支：不再把 `str(e)` 原样回显。改为记录到服务端日志（阶段 6 统一日志化），对前端只给通用错误码/文案：

```python
except Exception as e:
    logger.warning("Microsoft auth flow failed: %s", e)   # 内部细节进日志
    location = f"{frontend_url}/dashboard/roles?error=auth_failed"
```

前端按 `error=auth_failed` 展示固定的友好提示。

## 影响文件

- 新增：`utils/url_guard.py`
- 修改：`utils/http.py`（download_texture 校验 + 禁重定向）
- 修改：`backends/yggdrasil_client.py`（构造校验 + 禁重定向）
- 修改：`routers/microsoft_routes.py`（异常不回显原文）

## 测试与验证

- 单测 `validate_outbound_url`：
  - 放行：`https://textures.minecraft.net/...`、普通公网域名。
  - 拒绝：`http://127.0.0.1`、`http://169.254.169.254`、`http://10.0.0.1`、`http://192.168.1.1`、`http://[::1]`、`ftp://...`、`file:///etc/passwd`、无主机 URL、解析失败的域名。
- 集成：`/microsoft/import-profile` 传 `skin_url=http://169.254.169.254/...` → 该材质导入失败（返回 None/不写入），不发起内网请求。
- `/remote-ygg/import-profile` 传 `api_url=http://127.0.0.1:5432` → 400，不探测本地端口。
- 重定向：mock 一个返回 302→内网 的 URL，确认 `download_texture` 不跟随。
- 回调异常：触发 Microsoft 流程异常，确认前端 URL 只含 `error=auth_failed`，无堆栈/内部信息。
- `pytest -q` 全绿。

## 风险与回滚

中风险。主要副作用：
- **禁用重定向**可能影响依赖 3xx 的合法直链（实测罕见）。若线上有正常材质走重定向，改为「受控有限跟随 + 每跳校验」。
- `getaddrinfo` 增加一次 DNS 解析开销（可接受，导入是低频操作）。

`url_guard` 是新增模块，接入点少而集中，回滚只需移除各调用点的 `validate_outbound_url` 调用。
