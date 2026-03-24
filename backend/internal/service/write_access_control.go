package service

import (
	"context"
	"strings"

	"assessv2/backend/internal/auth"
	"gorm.io/gorm"
)

type orgWriteScope struct {
	unrestricted bool
	allowedOrgID map[uint]struct{}
}

func (s *orgWriteScope) allowsOrganization(organizationID uint) bool {
	if organizationID == 0 {
		return false
	}
	if s == nil {
		return false
	}
	if s.unrestricted {
		return true
	}
	_, ok := s.allowedOrgID[organizationID]
	return ok
}

func isRootClaims(claims *auth.Claims) bool {
	if claims == nil {
		return false
	}
	return auth.HasBusinessRole(claims.Roles, auth.RoleRoot)
}

func requireRootOrAssessmentAdminClaims(claims *auth.Claims) error {
	if claims == nil {
		return ErrForbidden
	}
	if isRootClaims(claims) {
		return nil
	}
	if !auth.HasBusinessRole(claims.Roles, auth.RoleAssessmentAdmin) {
		return ErrForbidden
	}
	return nil
}

func collectBindingsForRole(claims *auth.Claims, roleCode string) []auth.PermissionBinding {
	if claims == nil {
		return []auth.PermissionBinding{}
	}
	result := make([]auth.PermissionBinding, 0, len(claims.PermissionBindings))
	normalizedTargetRole := auth.NormalizeRoleCode(roleCode)
	for _, item := range claims.PermissionBindings {
		if strings.TrimSpace(item.RoleCode) == "" {
			result = append(result, item)
			continue
		}
		if auth.NormalizeRoleCode(item.RoleCode) == normalizedTargetRole {
			result = append(result, item)
		}
	}
	return result
}

func isSupportedOrgScopeType(scopeType string) bool {
	switch strings.ToLower(strings.TrimSpace(scopeType)) {
	case "organization", "org", "group", "company":
		return true
	default:
		return false
	}
}

func collectScopedOrganizationRoots(claims *auth.Claims) []uint {
	if claims == nil {
		return []uint{}
	}

	roots := make([]uint, 0, len(claims.PermissionBindings)+len(claims.OrgScopes))
	seen := make(map[uint]struct{}, len(claims.PermissionBindings)+len(claims.OrgScopes))
	appendScope := func(id uint) {
		if id == 0 {
			return
		}
		if _, exists := seen[id]; exists {
			return
		}
		seen[id] = struct{}{}
		roots = append(roots, id)
	}

	for _, binding := range collectBindingsForRole(claims, auth.RoleAssessmentAdmin) {
		if binding.ScopeOrgID == nil || *binding.ScopeOrgID == 0 {
			continue
		}
		if !isSupportedOrgScopeType(binding.ScopeOrgType) {
			continue
		}
		appendScope(*binding.ScopeOrgID)
	}
	for _, scope := range claims.OrgScopes {
		if scope.OrganizationID == 0 {
			continue
		}
		if !isSupportedOrgScopeType(scope.OrganizationType) {
			continue
		}
		appendScope(scope.OrganizationID)
	}
	return roots
}

func resolveGroupScopedOrganizationIDs(ctx context.Context, db *gorm.DB, claims *auth.Claims) (map[uint]struct{}, error) {
	if isRootClaims(claims) {
		return map[uint]struct{}{}, nil
	}
	if err := requireRootOrAssessmentAdminClaims(claims); err != nil {
		return nil, err
	}

	scopeRoots := collectScopedOrganizationRoots(claims)
	if len(scopeRoots) == 0 {
		return nil, ErrForbidden
	}

	allowed := make(map[uint]struct{}, 32)
	for _, scopeRootID := range scopeRoots {
		orgIDs, err := resolveOrganizationIDs(ctx, db, scopeRootID, true)
		if err != nil {
			return nil, err
		}
		for _, orgID := range orgIDs {
			if orgID == 0 {
				continue
			}
			allowed[orgID] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return nil, ErrForbidden
	}
	return allowed, nil
}

func requireOrgWriteScope(ctx context.Context, db *gorm.DB, claims *auth.Claims) (*orgWriteScope, error) {
	if isRootClaims(claims) {
		return &orgWriteScope{unrestricted: true, allowedOrgID: map[uint]struct{}{}}, nil
	}

	allowedOrgID, err := resolveGroupScopedOrganizationIDs(ctx, db, claims)
	if err != nil {
		return nil, err
	}
	return &orgWriteScope{
		unrestricted: false,
		allowedOrgID: allowedOrgID,
	}, nil
}
