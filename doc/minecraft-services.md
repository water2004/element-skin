# Minecraft Services 查询与角色名称

## 目标与名称策略

Issue #18 选择全站角色名称忽略大小写唯一。`CasePlayer` 与 `caseplayer`
表示同一个名称，不能由两个角色占用。数据库保留用户选择的显示大小写，
查询返回数据库中的名称，不返回请求中的大小写。

这是客户端兼容性选择，不是从 Yggdrasil 规范的“全局唯一”直接推导出的要求。
[Mojang authlib 7.0.61 源码](https://libraries.minecraft.net/com/mojang/authlib/7.0.61/authlib-7.0.61-sources.jar)
中的 `YggdrasilGameProfileRepository` 在单名、批量请求前转小写，并按小写名称匹配结果；
批量请求每页两个名称。服务端无法从归一化请求恢复原始大小写。

本次不新增 UUID 查询端点，不重构账号登录流程，不重新计算现有角色 UUID，
仅启动迁移自动处理既有大小写冲突，不合并或删除角色。无需新增前端界面；日常名称冲突提示继续使用原错误分类。

## 数据模型与写入

`profiles.name` 保持文本列，唯一索引 `profiles_name_key` 改为
`UNIQUE (lower(name))`。名称查询也使用 `lower(name)`，覆盖站点占用检查、
协议单名/批量查询和 `/v2/minecraft` 查询。

Yggdrasil 凭据验证中的角色名查找分支也沿用这一匹配规则；邮箱优先查找、
归属解析和密码校验流程保持原样。

数据库索引约束所有写入路径，包括注册、创建、用户/管理员改名、远程导入和正版角色同步，
并防止先查询后写入的并发竞争。原有名称冲突错误分类继续识别 `profiles_name_key`。
同一角色只修改显示大小写可以成功，角色 UUID、所有者和皮肤不变。
离线 UUID 模式只影响新角色生成，升级不重算已有 UUID。

## 升级与既有冲突处理

升级需要维护窗口。先备份数据库，并停止站点、worker 和其他可能写入角色的进程。
在部署前执行以下只读检查；结果必须保留在管理员控制的环境内：

```sql
SELECT lower(name) AS normalized_name,
       array_agg(id ORDER BY id) AS profile_ids,
       array_agg(name ORDER BY id) AS display_names,
       array_agg(user_id ORDER BY id) AS owner_ids
FROM profiles
GROUP BY lower(name)
HAVING count(*) > 1
ORDER BY lower(name);
```

启动迁移会自动解决冲突：

1. 每组 `lower(name)` 相同的角色按 UUID 的 C 排序保留第一项原名。该顺序只用于使迁移可重复，不代表角色创建时间或归属优先级。
2. 其余角色同样按 UUID 固定顺序处理，直接复用导入角色使用的 `util.ProfileNameCandidate`。
3. 按 `_1`、`_2`……`_99` 尝试名称；截短原名以使总长不超过 16 字符。按忽略大小写检查全表，跳过既有占用及本次迁移刚分配的名称。
4. 只更新 `profiles.name`，保留 UUID、归属、皮肤、披风、绑定关系和其他属性。
5. 在结构化表 `profile_name_case_migrations` 保存 `profile_id`、`old_name`、`new_name`、`migrated_at`。重复启动不会再次改名或覆盖记录。日常改名不写入此迁移表。

例如存在 `CasePlayer`、`caseplayer` 和已占用的 `CASEPLAYER_1`，如果前者 UUID 排序靠前，则保留 `CasePlayer`，将后者改为 `caseplayer_2`。

初始化 SQL、自动改名、迁移记录和唯一索引在同一事务中提交；改名期间持有数据库写锁，避免并发写入抢占候选名称。已有 profile 更新 webhook 触发器仍正常工作，其事件也在同一事务内。若 `_1` 到 `_99` 全部占用或任何数据库操作失败，所有改动回滚，站点启动失败，不会以半迁移状态提供查询或向外站回退。管理员按错误中给出的 UUID 处理名称后可重新启动；不增加第二套运行时兼容逻辑。

升级后可查询改名记录，并通知受影响的角色所有者：

```sql
SELECT profile_id, old_name, new_name, migrated_at
FROM profile_name_case_migrations
ORDER BY profile_id;
```

请预留索引构建和改名时间；该迁移不是在线无锁迁移。现有账号不被合并，客户端需要使用迁移后的显示名称；基于 UUID 的角色数据继续沿用。若希望人工决定保留者，可在升级前通过旧站点先处理冲突。

## HTTP 协议

| 请求 | 本地/回退未找到 | 上游来源 |
| --- | --- | --- |
| `GET /minecraftservices/publickeys`（也支持结尾 `/`） | 不适用 | 复用现有公钥服务 |
| `GET /minecraftservices/minecraft/profile/lookup/name/{playerName}` | 404 | `services_url` |
| `POST /minecraftservices/minecraft/profile/lookup/bulk/byname` | 200，忽略未找到项 | `services_url` |
| `GET /api/minecraft/profile/lookup/name/{playerName}` | 404 | `services_url` |
| `POST /api/minecraft/profile/lookup/bulk/byname` | 200，忽略未找到项 | `services_url` |
| `GET /minecraft/profile/lookup/name/{playerName}` | 404 | `services_url` |
| `POST /minecraft/profile/lookup/bulk/byname` | 200，忽略未找到项 | `services_url` |
| 现有 `/api/users/profiles/minecraft/{playerName}`、`/users/profiles/minecraft/{playerName}`、`/api/profiles/minecraft/{playerName}` | 204，无响应体 | `account_url` |
| `POST /api/profiles/minecraft` | 200，忽略未找到项 | `account_url` |

这些是 authlib/authlib-injector 使用的协议入口，复用同一处理器和业务实现。
Services 单名未找到返回 `{"error":"NOT_FOUND","errorMessage":"Profile not found."}`。
成功单名返回 `{id,name}`，批量返回该对象的数组。

Services 批量每次 1–10 项，旧 account 批量最多 100 项；按原始数组长度检查，
再忽略大小写去重。旧 account 空数组返回 `[]`，Services 空数组返回 400；
`null`、空名称、非字符串项、非数组、
多个 JSON 值及超限请求返回 400 Yggdrasil 错误格式（`IllegalArgumentException`）。
保留共享 JSON 请求体大小限制。重复名称只查询、返回一次，本地命中不会进入外站回退。
仅缺失名称进入回退，Services 批量请求使用上游
`/minecraft/profile/lookup/bulk/byname`，旧协议使用 `/profiles/minecraft`。

公钥入口沿用 `site_public.read.public` 权限包装、原公钥合并服务和缓存；
名称入口沿用已有协议访问规则。回退继续使用现有端点开关、调度策略及请求去重 store，
不引入新的本地名称缓存。

## 验证

数据库测试覆盖大小写查询、显示大小写修改、并发创建冲突、改名失败不改变数据、
旧库冲突自动改名、已占用后缀、16 字符截断、候选耗尽时整体回滚、处理后重试与重复启动。
HTTP 测试覆盖公钥权限及响应一致性、两种代理路径、单名和批量、
重复名称、未找到、错误响应、数量边界及 account/services 回退源。

运行后端全量测试与跨包覆盖率：

```text
go test -p 2 -count=1 ./... '-coverpkg=./...' '-covermode=atomic' '-coverprofile=coverage.out'
go tool cover '-func=coverage.out'
```

查询使用与唯一索引一致的表达式，新增查询没有逐名称数据库循环。
真实站点升级仍需使用其数据量评估维护窗口；真实 Minecraft 与
SkinsRestorer 联调属于部署验收，HTTP 模拟不能替代它。
