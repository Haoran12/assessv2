package auth

import (
	"sort"
	"strings"
)

const (
	RoleRoot            = "root"
	RoleAssessmentAdmin = "assessment_admin"
	RoleLeader          = "leader"
	RoleStaff           = "staff"

	// Legacy role codes kept for compatibility with historical data.
	RoleViewerLegacy       = "viewer"
	RoleGroupAdminLegacy   = "group_admin"
	RoleCompanyAdminLegacy = "company_admin"
	RoleDeptAdminLegacy    = "dept_admin"
)

var rolePermissions = map[string][]string{
	RoleAssessmentAdmin: {
		"assessment:view",
		"assessment:update",
		"rule:view",
		"rule:update",
		"score:view",
		"score:update",
		"org:view",
		"org:update",
		"vote:view",
		"vote:submit",
		"vote:manage",
		"vote:detail:view",
		"report:view",
		"backup:view",
		"backup:update",
		"audit:view",
		"audit:rollback",
		"setting:view",
		"setting:update",
	},
	RoleLeader: {
		"assessment:view",
		"rule:view",
		"score:view",
		"vote:view",
		"vote:submit",
		"report:view",
	},
	RoleStaff: {
		"assessment:view",
		"rule:view",
		"score:view",
		"vote:view",
		"vote:submit",
		"report:view",
	},
}

func NormalizeRoleCode(code string) string {
	roleCode := strings.ToLower(strings.TrimSpace(code))
	switch roleCode {
	case RoleRoot:
		return RoleRoot
	case RoleAssessmentAdmin:
		return RoleAssessmentAdmin
	case RoleLeader:
		return RoleLeader
	case RoleStaff:
		return RoleStaff
	case RoleViewerLegacy:
		return RoleStaff
	case RoleGroupAdminLegacy, RoleCompanyAdminLegacy, RoleDeptAdminLegacy:
		return RoleAssessmentAdmin
	default:
		return roleCode
	}
}

func NormalizeRoleCodes(codes []string) []string {
	result := make([]string, 0, len(codes))
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		normalized := NormalizeRoleCode(code)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func HasBusinessRole(granted []string, required string) bool {
	requiredRole := NormalizeRoleCode(required)
	if requiredRole == "" {
		return true
	}
	for _, item := range NormalizeRoleCodes(granted) {
		if item == requiredRole {
			return true
		}
	}
	return false
}

func RoleAllowsPermission(roleCodes []string, requiredPermission string) bool {
	required := strings.TrimSpace(requiredPermission)
	if required == "" {
		return true
	}

	normalizedRoles := NormalizeRoleCodes(roleCodes)
	for _, roleCode := range normalizedRoles {
		if roleCode == RoleRoot {
			return true
		}
		for _, granted := range rolePermissions[roleCode] {
			if permissionMatches(granted, required) {
				return true
			}
		}
	}
	return false
}

func PermissionsForRoles(roleCodes []string) []string {
	normalizedRoles := NormalizeRoleCodes(roleCodes)
	for _, roleCode := range normalizedRoles {
		if roleCode == RoleRoot {
			return []string{"*"}
		}
	}

	seen := map[string]struct{}{}
	result := make([]string, 0, 16)
	for _, roleCode := range normalizedRoles {
		for _, permission := range rolePermissions[roleCode] {
			if _, exists := seen[permission]; exists {
				continue
			}
			seen[permission] = struct{}{}
			result = append(result, permission)
		}
	}
	sort.Strings(result)
	return result
}

func permissionMatches(grantedPermission, requiredPermission string) bool {
	if grantedPermission == "*" || grantedPermission == requiredPermission {
		return true
	}
	if strings.HasSuffix(grantedPermission, "*") {
		prefix := strings.TrimSuffix(grantedPermission, "*")
		return strings.HasPrefix(requiredPermission, prefix)
	}
	return false
}
