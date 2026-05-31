# 后端分层重构计划

本目录是 `skin-backend` 分层重构的总计划。目标：**恢复 database / backend / router 三层的职责边界**，并拆分过载的 backend 类。

每个阶段是一个独立文件，可单独执行、单独验证、单独提交。阶段之间尽量解耦，但存在推荐顺序（见下文「执行顺序」）。

## 背景：当前的核心问题

经过对 `database_module/`、`backends/`、`routers/` 的通读，确认了三类问题：

1. **Database 层混入 Backend 逻辑**
   - `database_module/modules/texture.py::upload()` 做了图像规范化、尺寸校验、哈希计算、**写文件系统** —— 全部不属于数据访问层。
   - 游标的 base64 编码（`CursorEncoder.encode`）散落在 `user.py` / `texture.py` 的每个 `*_cursor` 方法里，属于传输层关注点。
   - 跨表业务级联（如 `texture.update_model` 顺带改 `profiles.texture_model`）埋在看似简单的 CRUD 方法里。

2. **Backend 层被反向绕过**
   - Router 直接调用 `db.*` 共约 40 处，材质上传 `db.texture.upload` 同时被 router、`site_backend`、`yggdrasil_backend` 直接调用，没有唯一权威入口。

3. **Backend 内部过于复杂**
   - `site_backend.py`（555 行）一个类塞入认证、邮箱验证、用户管理、角色管理、远程 Yggdrasil 导入、轮播图等 5+ 领域。
   - 重复代码：唯一角色名生成循环出现 3 次，角色名校验正则出现 3 处。

## 重构原则

- **每层只做自己的事**
  - `database_module/`：纯数据访问。输入输出是参数和领域对象/dict，不碰文件系统、不碰 HTTP、不编码游标、不做跨领域业务决策。
  - `backends/`：业务逻辑与编排。组合多个 DB 调用、外部服务、存储服务，做业务校验，抛 `HTTPException` / 领域异常。
  - `routers/`：HTTP 适配。解析请求、调 **一个** backend 方法、组织响应。不直接调 `db.*`，不做多步编排。
- **行为不变**：本计划是结构重构，不改变对外 API 契约和可观察行为。每阶段以「现有测试全绿」为通过标准。
- **小步提交**：一个阶段一个（或几个）PR，便于 review 和回滚。
- **测试先行/同步**：每阶段列出受影响的测试文件，移动逻辑时同步迁移测试。

## 阶段总览

| 阶段 | 文件 | 主题 | 风险 | 主要收益 |
|------|------|------|------|----------|
| 1 | [phase-1-texture-storage.md](./phase-1-texture-storage.md) | 抽出 `TextureStorage`，剥离 DB 层的图像/文件 IO | 中 | 消除最实锤的「DB 混入 backend」 |
| 2 | [phase-2-cursor-pagination.md](./phase-2-cursor-pagination.md) | 统一游标编解码到单一边界 | 中 | 消除分页逻辑的层级割裂 |
| 3 | [phase-3-split-site-backend.md](./phase-3-split-site-backend.md) | 拆分 `SiteBackend`，抽出 `ProfileImportBackend` | 中 | 降低单类复杂度，消除重复 |
| 4 | [phase-4-router-boundary.md](./phase-4-router-boundary.md) | 让 router 只调 backend，编排逻辑上移 | 中高 | 统一入口，恢复 backend 权威性 |
| 5 | [phase-5-settings-and-cleanup.md](./phase-5-settings-and-cleanup.md) | 抽 `SettingsBackend` + 修 typing/领域对象 | 低 | 收敛设置默认值，修潜在 bug |

## 执行顺序

推荐顺序：**1 → 2 → 3 → 5 → 4**。

理由：
- 阶段 1、2 处理 DB 层泄漏，是「自底向上」清理，先做能让上层重构站在干净的地基上。
- 阶段 3、5 在 backend 内部做拆分和清理，依赖 1、2 完成后的稳定接口。
- 阶段 4（router 边界）放最后，因为它需要 backend 已经提供了完整、权威的方法入口（阶段 1/3 会补齐这些入口）。提前做会反复返工。

阶段 5 风险最低，若想先积累一个「安全的」合并，也可把它提前到 1 之前作为热身。

## 通用验证

每阶段执行后均需：

```bash
cd skin-backend
pytest -q                    # 全量测试
pytest tests/database -q     # 数据层
pytest tests/backends -q     # 逻辑层
pytest tests/api -q          # 接口层（行为契约）
```

通过标准：重构前后 `pytest` 结果一致（全绿），无新增失败。涉及移动测试时，旧测试删除、新测试覆盖等价场景。
