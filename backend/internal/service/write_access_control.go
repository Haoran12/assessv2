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
	if len(result) > 0 {
		return result
	}

	legacy := make([]auth.PermissionBinding, 0, len(claims.OrgScopes))
	for _, orgScope := range claims.OrgScopes {
		scopeID := orgScope.OrganizationID
		legacy = append(legacy, auth.PermissionBinding{
			RoleCode:     roleCode,
			ScopeOrgType: orgScope.OrganizationType,
			ScopeOrgID:   &scopeID,
			IsPrimary:    orgScope.IsPrimary,
		})
	}
	return legacy
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

func requireAssessmentYearWriteScope(ctx context.Context, db *gorm.DB, claims *auth.Claims, yearID uint) error {
	if yearID == 0 {
		return ErrInvalidParam
	}
	if err := requireRootOrAssessmentAdminClaims(claims); err != nil {
		return err
	}
	if isRootClaims(claims) {
		return nil
	}

	scope, err := buildAssessmentAccessScope(ctx, db, claims)
	if err != nil {
		return err
	}

	var objectIDs []uint
	if err := db.WithContext(ctx).Table("assessment_objects").
		Where("year_id = ? AND is_active = 1", yearID).
		Pluck("id", &objectIDs).Error; err != nil {
		return fmt.Errorf("failed to load assessment objects for year scope check: %w", err)
	}
	if len(objectIDs) == 0 {
		return ErrForbidden
	}
	for _, objectID := range objectIDs {
		if scope.allowsDetailObject(objectID) {
			continue
		}
		return ErrForbidden
	}
	return nil
}

func requireCreateAssessmentYearScope(ctx context.Context, db *gorm.DB, claims *auth.Claims) error {
	if isRootClaims(claims) {
		return nil
	}
	_, err := resolveGroupScopedOrganizationIDs(ctx, db, claims)
	return err
}

func requireRuleDimensionWriteScope(
	ctx context.Context,
	db *gorm.DB,
	claims *auth.Claims,
	yearID uint,
	objectType string,
	objectCategory string,
) error {
	if err := requireRootOrAssessmentAdminClaims(claims); err != nil {
		return err
	}
	if isRootClaims(claims) {
		return nil
	}

	scope, err := buildAssessmentAccessScope(ctx, db, claims)
	if err != nil {
		return err
	}

	query := db.WithContext(ctx).Table("assessment_objects").
		Select("id").
		Where("year_id = ? AND is_active = 1 AND object_type = ? AND object_category = ?", yearID, objectType, objectCategory)

	var objectIDs []uint
	if err := query.Pluck("id", &objectIDs).Error; err != nil {
		return fmt.Errorf("failed to load assessment objects for rule scope check: %w", err)
	}
	if len(objectIDs) == 0 {
		return ErrForbidden
	}
	for _, objectID := range objectIDs {
		if scope.allowsDetailObject(objectID) {
			continue
		}
		return ErrForbidden
	}
	return nil
}

func isRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
