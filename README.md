# AssessV2

AssessV2 是一个以“考核场次（assessment session）”为核心的考核系统。

技术栈：
- 前端：Vue 3 + TypeScript + Element Plus
- 后端：Go + Gin + GORM + SQLite
- 桌面壳：Wails 2.x

## 核心数据规则（重要）

- **每个场次目录内的 `assess.db` 是该场次业务数据与规则的唯一真源**。
- 场次业务数据必须落在 `data/{assessment}/` 下，不再使用“年度目录”作为业务主存储。
- `accounts/` 属于系统级数据，允许集中存储。
- 组织树可作为通用数据源；场次创建后，场次对象快照独立保存，不随组织树后续变化而改变。
- **运行时不做自动迁移**。历史数据迁移必须通过离线命令执行。

## 数据目录约定

```text
data/
  accounts/
    accounts.db                 # 系统账号与权限数据
  {assessment}/
    assess.db                   # 场次业务数据唯一真源
    *.json                      # 该场次规则文件
```

说明：历史版本可能残留 `business_data.json` / `default_objects.json`，它们不是运行时真源。

## 启动方式

### 1. 后端

```bash
cd backend
go run ./cmd/server
```

默认监听：`127.0.0.1:8080`

### 2. 前端

```bash
cd frontend
npm install
npm run dev
```

默认访问：`http://127.0.0.1:5173`

### 3. 桌面（可选）

```bash
cd backend/desktop
wails dev
```

## 离线迁移命令

### 1. 场次业务表迁移到每个场次 `assess.db`

```bash
cd backend

# dry-run
go run ./cmd/migrate-session-business-db --db ../data/assess.db --data-root ../data

# apply
go run ./cmd/migrate-session-business-db --db ../data/assess.db --data-root ../data --apply
```

如果历史主库在别处（例如 `../data/2026/assess.db`），将 `--db` 改为实际路径。  
该命令会把旧 `default_objects.json`（若存在）导入 `assess.db` 的快照表。

### 2. 规则文件路径迁移到场次目录

```bash
cd backend

# dry-run
go run ./cmd/migrate-rule-file-paths --db ../data/assess.db --data-root ../data

# apply
go run ./cmd/migrate-rule-file-paths --db ../data/assess.db --data-root ../data --apply
```

该命令会清理旧结构痕迹：保留单一 `rule.json`，并移除旧的“基础规则/copy”冗余记录与文件。

## 文档

- 文档总览：[`docs/README.md`](./docs/README.md)
- 路由入口：`backend/internal/api/router/router.go`
