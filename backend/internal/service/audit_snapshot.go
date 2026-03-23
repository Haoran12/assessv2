package service

import (
	"sort"
	"strings"

	"assessv2/backend/internal/model"
)

func buildAuditDetail(eventCode string, before map[string]any, after map[string]any, extras map[string]any) map[string]any {
	detail := map[string]any{
		"eventCode": strings.TrimSpace(eventCode),
	}
	if before == nil {
		before = map[string]any{}
	}
	if after == nil {
		after = map[string]any{}
	}
	detail["before"] = before
	detail["after"] = after
	for key, value := range extras {
		detail[key] = value
	}
	return detail
}

func serializeUserForAudit(user *model.User) map[string]any {
	if user == nil {
		return map[string]any{}
	}
	roleIDs := make([]uint, 0, len(user.UserRoles))
	roleCodes := make([]string, 0, len(user.UserRoles))
	primaryRoleID := uint(0)
	for _, item := range user.UserRoles {
		roleIDs = append(roleIDs, item.RoleID)
		code := strings.TrimSpace(item.Role.RoleCode)
		if code != "" {
			roleCodes = append(roleCodes, code)
		}
		if item.IsPrimary {
			primaryRoleID = item.RoleID
		}
	}
	sort.Slice(roleIDs, func(i, j int) bool { return roleIDs[i] < roleIDs[j] })
	sort.Strings(roleCodes)

	return map[string]any{
		"id":                   user.ID,
		"username":             user.Username,
		"status":               user.Status,
		"must_change_password": user.MustChangePassword,
		"role_ids":             roleIDs,
		"role_codes":           roleCodes,
		"primary_role_id":      primaryRoleID,
		"last_login_at":        user.LastLoginAt,
		"last_login_ip":        user.LastLoginIP,
		"updated_at":           user.UpdatedAt,
		"deleted_at":           user.DeletedAt,
	}
}

func serializeRoleForAudit(role *model.Role) map[string]any {
	if role == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id":          role.ID,
		"role_code":   role.RoleCode,
		"role_name":   role.RoleName,
		"description": role.Description,
		"is_system":   role.IsSystem,
		"updated_at":  role.UpdatedAt,
	}
}

func serializeOrganizationForAudit(item *model.Organization) map[string]any {
	if item == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id":         item.ID,
		"org_name":   item.OrgName,
		"org_type":   item.OrgType,
		"parent_id":  item.ParentID,
		"leader_id":  item.LeaderID,
		"sort_order": item.SortOrder,
		"status":     item.Status,
		"updated_at": item.UpdatedAt,
		"deleted_at": item.DeletedAt,
	}
}

func serializeDepartmentForAudit(item *model.Department) map[string]any {
	if item == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id":              item.ID,
		"dept_name":       item.DeptName,
		"organization_id": item.OrganizationID,
		"parent_dept_id":  item.ParentDeptID,
		"leader_id":       item.LeaderID,
		"sort_order":      item.SortOrder,
		"status":          item.Status,
		"updated_at":      item.UpdatedAt,
		"deleted_at":      item.DeletedAt,
	}
}

func serializePositionLevelForAudit(item *model.PositionLevel) map[string]any {
	if item == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id":                item.ID,
		"level_code":        item.LevelCode,
		"level_name":        item.LevelName,
		"description":       item.Description,
		"is_system":         item.IsSystem,
		"is_for_assessment": item.IsForAssessment,
		"sort_order":        item.SortOrder,
		"status":            item.Status,
		"updated_at":        item.UpdatedAt,
	}
}

func serializeEmployeeForAudit(item *model.Employee) map[string]any {
	if item == nil {
		return map[string]any{}
	}
	hireDate := ""
	if item.HireDate != nil {
		hireDate = item.HireDate.Format("2006-01-02")
	}
	return map[string]any{
		"id":                item.ID,
		"emp_name":          item.EmpName,
		"organization_id":   item.OrganizationID,
		"department_id":     item.DepartmentID,
		"position_level_id": item.PositionLevelID,
		"position_title":    item.PositionTitle,
		"hire_date":         hireDate,
		"status":            item.Status,
		"updated_at":        item.UpdatedAt,
		"deleted_at":        item.DeletedAt,
	}
}
