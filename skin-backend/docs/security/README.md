# 后端安全与稳定性修复计划

本目录是 `skin-backend` 针对一次完整安全/性能/可维护性 review 的修复总计划。目标：**关闭可被利用的安全缺口、消除会随数据量恶化的性能瓶颈、补齐运维短板**，且不改变对外 API 契约。

每个阶段是一个独立文件，可单独执行、单独验证、单独提交。阶段之间尽量解耦，但存在推荐顺序（见「执行顺序」）。

## 背景：review 结论

通读 `database_module/`、`backends/`、`routers/`、`utils/`、`services/`、`Dockerfile`、`entrypoint.sh`、`config.yaml` 后，确认以下问题（已剔除经业主确认无需处理的项：删材质不删盘文件、"私有"材质语义、free-threaded 3.14 兼容性）。

按严重程度归类：

- **严重**：`config.yaml`（含 DB 密码与 JWT 默认密钥）被提交进 git 且打入镜像；Web 上传无大小限制；逐像素哈希在事件循环上同步执行且无尺寸上限 → 单请求可打挂全服；`CORS allow_origins:["*"] + allow_credentials:true`。
- **高**：用户可控 URL 导致 SSRF；封禁/降权对站点 API 不生效（JWT 无状态）；缺关键索引；反向代理下 `request.client.host` 取到代理 IP 致限流失效。
- **中/低**：改邮箱无校验易触发未捕获 500；DB 初始化失败被吞；OAuth state 用模块级 dict；弱口令策略形同虚设；异常细节回显；`print` 当日志；依赖未锁定。

> 关于 JWT 密钥：按业主决策，**继续只从配置文件读取**，不引入环境变量覆盖。因此本计划不改读取机制，只解决「默认密钥被提交进仓库/镜像」和「启动时未校验」这两点。

## 修复原则

- **行为不变优先**：除明确的安全收紧（如 CORS、封禁拦截），不改变对外 API 形状。每阶段以「现有测试全绿 + 新增针对性用例通过」为准。
- **小步提交**：一个阶段一个（或几个）PR，便于 review 和回滚。
- **失败要响亮**：能在启动期暴露的配置/环境问题，就 fail-fast，不要拖到运行期变成神秘 500。
- **默认安全**：新加的限制项给出安全的默认值，运营可在合理范围内放宽。

## 阶段总览

| 阶段 | 文件 | 主题 | 严重度 | 风险 |
|------|------|------|--------|------|
| 1 | [phase-1-config-and-secrets.md](./phase-1-config-and-secrets.md) | 配置/密钥脱离仓库、启动校验、CORS 收紧、修 favicon | 严重 | 低 |
| 2 | [phase-2-upload-and-image-dos.md](./phase-2-upload-and-image-dos.md) | 统一上传大小限制、像素上限、哈希向量化、线程池下放 | 严重 | 中 |
| 3 | [phase-3-ssrf-outbound-http.md](./phase-3-ssrf-outbound-http.md) | 出站 URL 白名单/内网封禁、收敛异常回显 | 高 | 中 |
| 4 | [phase-4-auth-and-account.md](./phase-4-auth-and-account.md) | 站点侧封禁/降权拦截、弱口令策略、改邮箱校验、降低枚举 | 高 | 中 |
| 5 | [phase-5-db-index-proxy-init.md](./phase-5-db-index-proxy-init.md) | 补索引、开启 proxy headers、DB 初始化 fail-fast | 高 | 低 |
| 6 | [phase-6-operational-hardening.md](./phase-6-operational-hardening.md) | OAuth state 存储、日志化、依赖锁定、单实例约束文档 | 中/低 | 低 |

## 执行顺序

推荐顺序：**1 → 5 → 2 → 3 → 4 → 6**。

理由：
- **阶段 1** 最高危且风险最低（多为配置与脚本改动），先做能立刻消除「公开默认密钥」这一致命项。
- **阶段 5** 紧随其后：索引与 proxy headers 是低风险高收益的基础设施修复，且 proxy headers 修好后，阶段 2/4 的限流相关验证才有意义。
- **阶段 2、3** 是面向资源耗尽与 SSRF 的核心加固，改动集中在上传/下载链路。
- **阶段 4** 涉及鉴权依赖与用户态，改动面稍广，放在基础设施稳定后做。
- **阶段 6** 是收尾的运维硬化，可随时插入。

各阶段相互独立，如需可调整顺序；唯一软依赖是「阶段 5 的 proxy headers 先于阶段 2/4 的限流验证」。

## 通用验证

每阶段执行后均需：

```bash
cd skin-backend
pytest -q                    # 全量测试
pytest tests/api -q          # 接口层（行为契约）
```

通过标准：无新增失败；每阶段额外列出的针对性用例通过。涉及安全收紧的阶段，需手动验证「攻击路径被拒、正常路径不受影响」。

## 部署约束

后端在进程内维护多份内存状态（OAuth state、settings 缓存、fallback 缓存、限流计数），**不跨进程共享**，因此当前**只能单实例 / 单 worker 运行**。详见 [../deployment-notes.md](../deployment-notes.md)。
