# AssessV2

根据 [`docs/00-项目交付总结.md`](./docs/00-项目交付总结.md) 搭建的 M2 基础框架，覆盖以下技术栈：

- 前端：Vue 3 + TypeScript + Element Plus + Pinia + Vue Router
- 后端：Go + Gin + GORM + SQLite
- 桌面：Wails 2.x（容器层骨架）

## 目录结构

```text
assessv2/
├── docs/              # 需求与设计文档
├── backend/           # Go 后端
├── frontend/          # Vue 前端
├── backend/desktop/   # Wails 桌面容器
└── README.md
```

## 当前已完成

- 后端分层目录与基础服务入口
- SQLite 初始化与 system_config 表自动迁移
- 统一响应格式：`{ code, message, data }`
- 认证模块基础接口：`POST /api/auth/login`
- JWT 中间件骨架
- 10 组 API 模块占位接口（`/_ping`）
- 前端登录页、主布局、模块占位页
- 前后端联调基础（Axios + 路由守卫）
- Wails 最小项目结构和配置文件

## 启动方式

### 1. 后端

```bash
cd backend
go mod tidy
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

初始化登录账户（临时）：`admin / admin123`

### 3. Wails（可选）

```bash
cd backend/desktop
wails dev
```

## 已对齐的文档要点

- 技术选型：与 `docs/00` 中前后端与 Wails 选型一致
- 架构分层：UI/API/Service/Repository/DB 分层落位
- 模块边界：`auth/org/assessment/rules/scores/votes/calc/reports/backup/system`
- 响应规范：统一 JSON 响应结构

## 后续建议开发顺序

1. 完善认证与 RBAC（用户表、会话表、权限点）
2. 建立组织架构核心模型（group/company/department/person）
3. 实现规则配置与分数模块定义
4. 接入计算引擎（表达式 + DAG 依赖处理）
5. 实现投票、报表与备份能力
