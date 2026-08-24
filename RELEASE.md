# Element Skin v4.0.1

> **重要：v3.0.0 至 v4.0.0 生成或写入的材质可能受到哈希算法问题影响。**
>
> 升级前必须完全停止服务，将 `data/`、`frontend/`、生产 `.env` 和当前
> `docker-compose.yml` 冷备份到部署目录之外。升级后必须执行本版本提供的材质检查/修复脚本。
>
> 从 v3 升级时，自动数据库迁移只支持 **v3.0.2 → v4.0.1**。v3.0.0 和 v3.0.1
> 必须先升级到 v3.0.2 并确认运行正常。v3.0.2 升级到 v4.0.1 后，必须先完整启动一次
> v4.0.1 完成数据库迁移，再停服执行材质修复。

---

# 修复

* 材质哈希现在严格按照 Yggdrasil 规范处理带半透明像素的 PNG，与 Python 版实现及其他兼容实现保持一致。
* 修复 Microsoft 正版角色同步使用已存在材质时，角色页已经更新 `default` / `slim`、但衣柜仍显示旧模型的问题。同步只更新当前用户的材质模型，保留备注、公开状态等独立材质信息，也不会覆盖其他上传者的公共库元数据。
* 修复衣柜材质详情中的公开/私有开关无法连续切换的问题；更新成功后，弹窗与衣柜列表会立即保持一致。

---

# 材质哈希数据修复

## 影响范围

v3.0.0 至 v4.0.0 在计算 PNG 材质哈希时可能预乘半透明像素的 RGB 分量，导致文件名、数据库引用和标准
Yggdrasil 哈希不一致。完全不含半透明像素的材质通常不受影响，但只要站点曾运行上述任一版本，就应执行一次检查。

`repair_texture_hashes.py` 会读取指定材质目录，重新计算每个 PNG 的标准哈希，并检查或修复：

* 用户头像；
* 角色皮肤和披风；
* 用户衣柜材质；
* 公共材质库及其使用次数。

脚本默认只检查，不写文件或数据库；只有显式提供 `--apply` 才会执行修复。如果错误哈希和正确哈希
对应的文件或记录同时存在，脚本以原先已经使用正确哈希的文件和数据库记录为准，并将其他引用合并到正确记录。

多个用户头像、多个角色皮肤或披风引用同一哈希是正常共享，不属于冲突。脚本会保留所有用户和角色，
逐行更新它们的哈希引用；只有同一用户衣柜或公共材质库的唯一键同时出现错误哈希与正确哈希时，才会合并材质元数据记录。

## Docker Compose 部署

以下操作均在 Docker 部署根目录执行，并假定使用官方 `docker-compose.yml`、`.env` 和默认的
`./frontend/static/textures` 材质目录。

如果当前为 v3.0.0 或 v3.0.1，先不要替换任何 v4 文件。请先按 v3.0.2 的升级说明升级到 v3.0.2，
确认站点能够正常启动、登录和访问原有数据，再从下面第 1 步开始。

### 1. 停服和备份

完全停止所有服务：

~~~bash
docker compose stop
~~~

将以下内容完整备份到部署目录之外：

* `data/`；
* `frontend/`；
* 生产 `.env`；
* 当前 `docker-compose.yml`。

`data/` 已包含 PostgreSQL、Redis、Yggdrasil/OIDC 密钥等持久化数据，不需要另外执行数据库导出。

### 2. 更新部署文件和镜像

从仓库的 `v4.0.1` 标签下载以下三个文件：

* 将仓库根目录的 [`docker-compose.yml`](https://github.com/water2004/element-skin/blob/v4.0.1/docker-compose.yml)
  放到部署根目录，覆盖现有同名文件；
* 将 [`repair_texture_hashes.py`](https://github.com/water2004/element-skin/blob/v4.0.1/skin-backend/repair_texture_hashes.py)
  下载后直接放到部署根目录，文件名保持为 `repair_texture_hashes.py`；
* 将 [`requirements-maintenance.txt`](https://github.com/water2004/element-skin/blob/v4.0.1/skin-backend/requirements-maintenance.txt)
  下载后直接放到部署根目录，文件名保持为 `requirements-maintenance.txt`。

保留现有 `.env`、`frontend/` 和 `data/`，不要用示例配置或空目录覆盖生产数据。

从 v3.0.2 升级时，在执行任何新版 Compose 命令之前，先参照 v4.0.1 的
[`.env.example`](https://github.com/water2004/element-skin/blob/v4.0.1/.env.example)
补齐 v4 必需配置，但不要覆盖原有生产值。`OIDC_PRIVATE_KEY` 和 `OIDC_PUBLIC_KEY` 指向持久化目录中的文件，
文件不存在时会自动生成；`IDENTITY_ENCRYPTION_KEY` 必须生成一次并长期保持不变。

如果 `.env` 中的 `ELEMENT_SKIN_IMAGE` 固定为旧版本标签（例如 `3.0.2` 或 `4.0.0`），将它改为
`ghcr.io/water2004/element-skin:4.0.1`；使用 `latest` 时无需修改。完成 `.env` 后拉取 v4.0.1 镜像：

~~~bash
docker compose pull backend webhook-worker
~~~

### 3. 按当前版本完成数据库升级

* **当前为 v4.0.0：** 数据库已经是 v4 结构，不要启动主后端，直接进入下一节。
* **当前为 v3.0.2：** 完整启动 v4.0.1，确认站点可访问，并检查 `backend` 日志确认数据库迁移和
  Microsoft 配置迁移成功。随后再次执行
  `docker compose stop`，再进入下一节。

v3.0.2 首次启动 v4.0.1：

~~~bash
docker compose up -d
docker compose logs --tail=100 backend webhook-worker
~~~

确认迁移完成后重新停服：

~~~bash
docker compose stop
~~~

### 4. 启动 PostgreSQL

只启动 PostgreSQL，并最多等待 120 秒直到 healthcheck 通过：

~~~bash
docker compose up -d --wait --wait-timeout 120 db
~~~

如果命令以非零状态退出，不要继续修复；先执行 `docker compose logs db` 查明启动失败原因。

### 5. 执行检查

在主后端、Redis 和 Webhook Worker 均停止时，使用临时 Python 容器执行 dry-run：

~~~bash
docker run --rm \
  --network "container:$(docker compose ps -q db)" \
  --env-file .env \
  -v "$PWD/repair_texture_hashes.py:/work/repair_texture_hashes.py:ro" \
  -v "$PWD/requirements-maintenance.txt:/work/requirements-maintenance.txt:ro" \
  -v "$PWD/frontend/static/textures:/textures" \
  -w /work \
  python:3.13-slim \
  sh -c 'python -m pip install --disable-pip-version-check -r requirements-maintenance.txt && PGPORT="$DATABASE_PORT" PGDATABASE="$DATABASE_NAME" PGUSER="$DATABASE_USER" PGPASSWORD="$DATABASE_PASSWORD" PGSSLMODE="$DATABASE_SSLMODE" python repair_texture_hashes.py --textures-dir /textures --postgres-dsn "host=127.0.0.1"'
~~~

没有问题时脚本退出码为 `0`；发现需要修复的材质时退出码为 `2`，这是 dry-run 的预期结果。
先检查脚本输出的哈希映射和各表引用数量，再执行下一步。

### 6. 执行修复

使用相同目录和数据库参数执行修复：

~~~bash
docker run --rm \
  --network "container:$(docker compose ps -q db)" \
  --env-file .env \
  -v "$PWD/repair_texture_hashes.py:/work/repair_texture_hashes.py:ro" \
  -v "$PWD/requirements-maintenance.txt:/work/requirements-maintenance.txt:ro" \
  -v "$PWD/frontend/static/textures:/textures" \
  -w /work \
  python:3.13-slim \
  sh -c 'python -m pip install --disable-pip-version-check -r requirements-maintenance.txt && PGPORT="$DATABASE_PORT" PGDATABASE="$DATABASE_NAME" PGUSER="$DATABASE_USER" PGPASSWORD="$DATABASE_PASSWORD" PGSSLMODE="$DATABASE_SSLMODE" python repair_texture_hashes.py --textures-dir /textures --postgres-dsn "host=127.0.0.1" --apply'
~~~

如果 dry-run 或 `--apply` 报错，不要启动写入服务，也不要手工移动部分材质文件。保留现场；需要回滚时，
完全停止 PostgreSQL，再使用同一份冷备份中的 `data/`、`frontend/`、`.env` 和 `docker-compose.yml`
整体恢复，不要混用不同时点的文件。

### 7. 重新启动 v4.0.1

~~~bash
docker compose up -d --wait --wait-timeout 120
docker compose logs --tail=100 backend webhook-worker
~~~

验证角色材质、用户头像、衣柜和公共材质库。

## 本地或自建部署

先停止主后端与 Webhook Worker，再按实际部署方式对 PostgreSQL 持久化数据、材质目录和配置做同一时点备份。
在脚本所在目录安装独立维护依赖：

~~~bash
python -m pip install -r requirements-maintenance.txt
~~~

先执行 dry-run：

~~~bash
python repair_texture_hashes.py \
  --textures-dir /absolute/path/to/static/textures \
  --postgres-dsn 'postgresql://user:password@host:5432/elementskin?sslmode=disable'
~~~

确认报告后再执行修复：

~~~bash
python repair_texture_hashes.py \
  --textures-dir /absolute/path/to/static/textures \
  --postgres-dsn 'postgresql://user:password@host:5432/elementskin?sslmode=disable' \
  --apply
~~~

`--textures-dir` 与 `--postgres-dsn` 均为必填参数。不要在服务仍可能上传、删除或同步材质时运行 `--apply`。

---

**Full Changelog**: https://github.com/water2004/element-skin/compare/v4.0.0...v4.0.1
