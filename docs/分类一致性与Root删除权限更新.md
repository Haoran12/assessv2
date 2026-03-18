# 分类一致性与 Root 删除权限更新（2026-03-13）

## 1. 分类体系（统一口径）

本次明确并固定以下分类体系：

- 团体分类：
  - `group`（集团）
  - `group_leadership_team`（集团领导班子）
  - `group_department`（集团部门）
  - `subsidiary_company`（权属企业）
  - `subsidiary_company_leadership_team`（权属企业领导班子）
  - `subsidiary_company_department`（权属企业部门）
- 个人分类：
  - `leadership_main`（领导班子正职）
  - `leadership_deputy`（领导班子副职）
  - `department_main`（部门正职）
  - `department_deputy`（部门副职）
  - `general_management_personnel`（一般管理人员）

## 2. 数据库调整

新增主数据表：`assessment_categories`

- 字段：
  - `category_code`（唯一编码）
  - `category_name`（展示名称）
  - `object_type`（`team` / `individual`）
  - `sort_order`、`is_system`、`status`
  - `created_at`、`updated_at`
- 迁移文件：
  - `backend/migrations/0008_assessment_categories.up.sql`
  - `backend/migrations/0008_assessment_categories.down.sql`
- 种子数据：
  - 在 `SeedBaselineData` 中新增 `seedDefaultAssessmentCategories`，写入 11 个默认分类。

说明：
- `position_levels` 继续用于“人员职级/岗位分类”管理。
- TopBar 与规则的“对象分类”不再由前端散落常量独立维护，而是通过分类主数据统一输出。

## 3. 后端调整

### 3.1 新增分类查询接口

- `GET /api/org/assessment-categories`
- 权限：`assessment:view`
- 用途：供 TopBar、规则页、上下文模块统一读取分类主数据。

### 3.2 Root 删除权限增强

新增 Root 专属删除接口：

- `DELETE /api/org/organizations/:id`
- `DELETE /api/org/departments/:id`
- `DELETE /api/org/employees/:id`

并保留：

- `DELETE /api/org/position-levels/:id`（Root）

删除规则：

- 组织删除前会校验是否仍有子组织/部门/人员。
- 部门删除前会校验是否仍有子部门/人员。
- 人员删除为软删除（设置 `deleted_at`）。
- 分类删除允许 Root 执行；若仍被引用（在用）则按业务规则拒绝。

## 4. 前端交互调整

### 4.1 TopBar 分类统一来源

- `context store` 初始化时调用 `/api/org/assessment-categories`。
- TopBar 分类选项改为使用上下文 store 提供的统一选项。
- 本地存储分类值会按当前有效分类集合进行校验与归一化。

### 4.2 规则页面分类来源统一

- `RulesView` 移除本地硬编码 `categoryMap`。
- 规则筛选/创建/模板应用的分类选项统一复用分类定义。

### 4.3 组织架构页面 Root 删除能力

- `OrganizationView` 新增 Root 可见删除按钮：
  - 组织
  - 部门
  - 人员
  - 分类（含系统分类，仍受“被引用不可删”约束）
- 组织架构四个列表（组织/部门/分类/人员）首列展示“序号”而非数据库 `id`；序号仅按当前表格数据从 1 递增

## 5. 验证

后端测试：

- `go test ./internal/api/router -run TestM2 -count=1` 通过
- 新增测试覆盖：
  - 分类列表接口返回默认 11 类及团队/个人分布
  - Root 删除组织/部门/人员成功
  - 非 Root 删除组织/部门/人员被拒绝
  - Root 可删除系统分类

前端检查：

- `npm run typecheck` 通过
