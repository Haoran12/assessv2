package service

import (
	"context"
	"errors"
	"fmt"
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

func resolveGroupScopedOrganizationIDs(ctx context.Context, db *gorm.DB, claims *auth.Claims) (map[uint]struct{}, error) {
	if isRootClaims(claims) {
		return map[uint]struct{}{}, nil
	}
	if err := requireRootOrAssessmentAdminClaims(claims); err != nil {
		return nil, err
	}

	bindings := collectBindingsForRole(claims, auth.RoleAssessmentAdmin)
	if len(bindings) == 0 {
		return nil, ErrForbidden
	}

	groupScopeIDs := make(map[uint]struct{}, len(bindings))
	for _, binding := range bindings {
		if binding.ScopeOrgID == nil || *binding.ScopeOrgID == 0 {
			continue
		}
		scopeType := strings.ToLower(strings.TrimSpace(binding.ScopeOrgType))
		if scopeType != "organization" && scopeType != "org" && scopeType != "group" && scopeType != "company" {
			continue
		}

		var organization struct {
			ID      uint
			OrgType string
		}
		if err := db.WithContext(ctx).Table("organizations").
			Select("id, org_type").
			Where("id = ? AND deleted_at IS NULL", *binding.ScopeOrgID).
			First(&organization).Error; err != nil {
			if isRecordNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("failed to resolve organization scope for permission binding: %w", err)
		}
		if organization.OrgType == "group" {
			groupScopeIDs[organization.ID] = struct{}{}
		}
	}
	if len(groupScopeIDs) == 0 {
		return nil, ErrForbidden
	}

	allowed := make(map[uint]struct{}, 32)
	for groupID := range groupScopeIDs {
		orgIDs, err := resolveOrganizationIDs(ctx, db, groupID, true)
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

func isRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
