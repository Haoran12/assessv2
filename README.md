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

## 自定义脚本（glang-expr）使用说明

项目中的“自定义脚本”基于 `github.com/expr-lang/expr`（常被称为 Golang Expr / glang-expr）。

### 1. 使用位置

- 分数模块：`calculationMethod = custom_script`，脚本字段为 `customScript`，**返回值必须是数字**。
- 等第规则：启用 `extraConditionEnabled` 后，脚本字段为 `extraConditionScript`，**返回值必须是布尔值**。

### 2. 语法要点

- 支持常见表达式语法：`+ - * /`、比较（`> >= < <= == !=`）、逻辑（`&& || !`）、括号。
- 字符串请使用双引号，如：`"Q1"`、`"department"`。
- 可通过 `moduleScores["module_key"]` 读取模块分。

### 3. 模块脚本可用变量（`customScript`）

- 基础变量：`periodCode`、`objectId`、`groupCode`、`objectType`、`targetId`、`targetType`、`parentObjectId`、`extraAdjust`
- 分数映射：
  - `moduleScores`：当前对象已计算模块分（按模块顺序逐步产生）
  - `rawModuleScores`：原始录入模块分
- 若模块 `moduleKey` 是合法标识符，也可直接用同名变量（例如 `base_performance`）访问分数。

### 4. 等第脚本可用变量（`extraConditionScript`）

- 基础变量：`periodCode`、`objectId`、`groupKey`、`objectType`、`targetId`、`targetType`、`parentObjectId`
- 评分变量：`totalScore`、`rank`、`extraAdjust`、`moduleScores`
- 同样支持通过模块 `moduleKey`（合法标识符时）直接访问模块分。

### 5. 可调用函数

- `score(periodCode, objectId)`：读取总分，找不到返回 `0`
- `rank(periodCode, objectId)`：读取排名，找不到返回 `0`
- `grade(periodCode, objectId)`：读取等第，找不到返回 `""`
- `moduleScore(periodCode, objectId, moduleKey)`：读取模块分，找不到返回 `0`
- `targetScore(periodCode, targetType, targetId)`：按业务目标读取总分，找不到返回 `0`
- `hasScore(periodCode, objectId)`：是否已有总分

### 6. 示例

模块脚本示例（返回 number）：

```expr
base_performance * 0.7 + moduleScore("Q1", objectId, "peer_review") * 0.3
```

等第额外条件示例（返回 bool）：

```expr
totalScore >= 90 && hasScore(periodCode, objectId) && grade("Q1", objectId) != "C"
```

### 7. 注意事项

- `periodCode` 在系统内会标准化为大写，建议脚本中也使用大写周期码（如 `Q1`）。
- 模块脚本在保存时不会做强校验；运行失败时该模块按 `0` 处理。
- 等第额外脚本在启用后会进行布尔表达式校验；若脚本运行时报错，计算接口会返回业务错误。

## 文档

- 文档总览：[`docs/README.md`](./docs/README.md)
- 路由入口：`backend/internal/api/router/router.go`
