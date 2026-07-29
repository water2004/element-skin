# Element Skin v3.0.2

> **v3.0.2 修复了 fallback 配置保存的数据一致性问题。**
>
> 本版本没有数据库 schema、配置项或公开 API 变更，可以从 v3.0.1 直接升级。

---

# 修复

* 保存 fallback 配置时不再重建全部端点，既有白名单、端点 ID 与运行状态会被正确保留。
* 修复多个 fallback 端点及新建端点的白名单差异同步，新增和删除项现在会写入对应端点。

---

**Full Changelog**: https://github.com/water2004/element-skin/compare/v3.0.1...v3.0.2
