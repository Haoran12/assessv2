# 资源级权限控制使用指南

## 概述

本系统实现了类似 Linux 文件权限的资源级访问控制,支持 Owner/Group/Others 三层权限管理。

**重要:** root 角色拥有完整的最高权限,可以访问和操作所有资源,无需任何权限检查。

## 权限位说明

每个资源有一个 `permission_mode` 字段,采用类似 Linux 的权限模式:

### 权限位定义
- **R (Read)**: 读取权限 (值: 8)
- **W (Write)**: 写入权限 (值: 4)
- **D (Delete)**: 删除权限 (值: 2)
- **X (Execute)**: 执行权限 (值: 1) - 用于特殊操作如审批、提交

### 权限层级
- **Owner**: 资源创建者 (CreatedBy/InputBy)
- **Group**: 同组织/同部门用户
- **Others**: 其他用户

### 权限模式示例

| 模式 | 十进制 | 说明 |
|------|--------|------|
| 0644 | 420 | Owner(RW), Group(R), Others(R) - 默认公开资源 |
| 0600 | 384 | Owner(RW), Group(无), Others(无) - 私有资源 |
| 0640 | 416 | Owner(RW), Group(R), Others(无) - 组内可见 |
| 0754 | 492 | Owner(RWDX), Group(RX), Others(R) - Owner全权限 |
| 0700 | 448 | Owner(RWDX), Group(无), Others(无) - 完全私有 |

## 默认权限配置

| 资源类型 | 默认模式 | 说明 |
|----------|----------|------|
| 考核年度 (assessment_year) | 0644 | 所有人可读,Owner可改 |
| 考核规则 (assessment_rule) | 0644 | 所有人可读,Owner可改 |
| 规则模板 (rule_template) | 0644 | 所有人可读,Owner可改 |
| 直接评分 (direct_score) | 0600 | 仅Owner可读写 |
| 加减分 (extra_point) | 0640 | Owner可改,组内可读 |

## 在路由中使用

### 示例 1: 保护考核年度更新操作

```go
// 在 router.go 中
assessment.PUT("/years/:id/status",
    middleware.RequirePermission("assessment:update"),  // 功能权限检查
    middleware.RequireResourcePermission(              // 资源权限检查
        db,
        "write",                                       // 需要写权限
        middleware.ExtractIDFromPath("id"),            // 从路径提取ID
        middleware.LoadAssessmentYearResource,         // 加载资源信息
    ),
    assessmentHandler.UpdateYearStatus,
)
```

### 示例 2: 保护评分删除操作

```go
scores.DELETE("/direct/:id",
    middleware.RequirePermission("score:update"),
    middleware.RequireResourcePermission(
        db,
        "delete",                                      // 需要删除权限
        middleware.ExtractIDFromPath("id"),
        middleware.LoadDirectScoreResource,
    ),
    scoreHandler.DeleteDirectScore,
)
```

### 示例 3: 保护加减分审批操作

```go
scores.POST("/extra/:id/approve",
    middleware.RequirePermission("score:update"),
    middleware.RequireResourcePermission(
        db,
        "execute",                                     // 需要执行权限
        middleware.ExtractIDFromPath("id"),
        middleware.LoadExtraPointResource,
    ),
    scoreHandler.ApproveExtraPoint,
)
```

## root 特权机制

root 角色在以下层面享有完整权限:

### 1. 中间件层自动放行
```go
// 在 RequireResourcePermission 中
if auth.HasRole(claims.Roles, "root") {
    c.Next()  // root 直接通过,不检查资源权限
    return
}
```

### 2. 权限检查函数优先判断
```go
// 在 CheckResourcePermission 中
if HasRole(roles, "root") {
    return true  // root 无条件返回 true
}
```

### 3. Service 层二次保障
```go
func (s *Service) UpdateResource(userID uint, roles []string, resourceID uint) error {
    if !auth.HasRole(roles, "root") {
        // 非 root 用户才检查权限
        if !s.canModifyResource(userID, roles, resourceID) {
            return ErrPermissionDenied
        }
    }
    // 执行更新
}
```

## 权限检查流程

```
用户请求
  ↓
JWT 认证 (RequireJWT)
  ↓
组织范围验证 (RequireOrgScope)
  ↓
功能权限检查 (RequirePermission)
  ↓
资源权限检查 (RequireResourcePermission)
  ├─ root 角色? → 是 → 直接放行
  └─ 否 ↓
      加载资源信息
        ↓
      判断用户关系 (Owner/Group/Others)
        ↓
      检查对应层级权限位
        ↓
      允许/拒绝访问
```

## 自定义资源加载器

如果需要为新资源类型添加权限控制,创建资源加载器:

```go
func LoadMyResourceLoader(ctx context.Context, db *gorm.DB, resourceID uint) (*ResourceInfo, error) {
    var resource model.MyResource
    if err := db.WithContext(ctx).First(&resource, resourceID).Error; err != nil {
        return nil, err
    }

    ownerID := uint(0)
    if resource.CreatedBy != nil {
        ownerID = *resource.CreatedBy
    }

    return &ResourceInfo{
        OwnerID:        ownerID,
        PermissionMode: resource.PermissionMode,
        OrgType:        resource.OrgType,  // 如果资源属于特定组织
        OrgID:          resource.OrgID,
    }, nil
}
```

## 数据库迁移

运行迁移添加权限字段:

```bash
# 应用迁移
go run cmd/migrate/main.go up

# 回滚迁移
go run cmd/migrate/main.go down
```

## 测试建议

### 单元测试
- 测试权限位计算: `model.HasPermission`
- 测试关系判断: `auth.GetUserResourceRelation`
- 测试 root 特权: 确保 root 角色始终返回 true

### 集成测试
1. root 用户访问任意资源 → 成功
2. Owner 用户修改自己创建的资源 → 成功
3. 普通用户修改他人资源(权限不足) → 403
4. 同组用户读取组内资源(权限允许) → 成功
5. 修改资源的 permission_mode → 验证权限变化生效

## 注意事项

1. **root 角色优先**: 所有权限检查必须首先判断 root 角色
2. **默认权限**: 新建资源时使用模型中定义的默认权限模式
3. **系统资源**: IsSystem=true 的资源建议设置更严格的权限
4. **性能考虑**: 资源加载器会查询数据库,注意缓存优化
5. **向后兼容**: 现有资源会通过迁移自动设置默认权限
