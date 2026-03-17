package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"assessv2/backend/internal/auth"
	"gorm.io/gorm"
)

type assessmentAccessScope struct {
	unrestricted      bool
	readableObjectIDs map[uint]struct{}
	detailObjectIDs   map[uint]struct{}
}

func buildAssessmentAccessScope(ctx context.Context, db *gorm.DB, claims *auth.Claims) (*assessmentAccessScope, error) {
	if claims == nil {
		return nil, ErrForbidden
	}

	normalizedRoles := auth.NormalizeRoleCodes(claims.Roles)
	for _, roleCode := range normalizedRoles {
		if roleCode == auth.RoleRoot {
			return &assessmentAccessScope{unrestricted: true}, nil
		}
	}

	scope := &assessmentAccessScope{
		readableObjectIDs: map[uint]struct{}{},
		detailObjectIDs:   map[uint]struct{}{},
	}

	bindings := make([]auth.PermissionBinding, 0, len(claims.PermissionBindings))
	bindings = append(bindings, claims.PermissionBindings...)
	if len(bindings) == 0 {
		for _, orgScope := range claims.OrgScopes {
			scopeID := orgScope.OrganizationID
			bindings = append(bindings, auth.PermissionBinding{
				ScopeOrgType: orgScope.OrganizationType,
				ScopeOrgID:   &scopeID,
				IsPrimary:    orgScope.IsPrimary,
			})
		}
	}

	for _, binding := range bindings {
		applyRoles := normalizedRoles
		if roleCode := strings.TrimSpace(binding.RoleCode); roleCode != "" {
			normalizedBindingRole := auth.NormalizeRoleCode(roleCode)
			if !auth.HasBusinessRole(normalizedRoles, normalizedBindingRole) {
				continue
			}
			applyRoles = []string{normalizedBindingRole}
		}

		for _, roleCode := range applyRoles {
			if err := applyBindingToScope(ctx, db, scope, roleCode, binding); err != nil {
				return nil, err
			}
		}
	}

	return scope, nil
}

func applyBindingToScope(
	ctx context.Context,
	db *gorm.DB,
	scope *assessmentAccessScope,
	roleCode string,
	binding auth.PermissionBinding,
) error {
	switch auth.NormalizeRoleCode(roleCode) {
	case auth.RoleStaff:
		addObjectID(scope.readableObjectIDs, binding.PersonObjectID)
		addObjectID(scope.detailObjectIDs, binding.PersonObjectID)
		return nil
	case auth.RoleLeader:
		addObjectID(scope.readableObjectIDs, binding.PersonObjectID)
		addObjectID(scope.detailObjectIDs, binding.PersonObjectID)
		addObjectID(scope.readableObjectIDs, binding.TeamObjectID)
		if binding.ScopeOrgID == nil || strings.TrimSpace(binding.ScopeOrgType) == "" {
			return nil
		}
		objectIDs, err := resolveScopeObjectIDs(ctx, db, binding.ScopeOrgType, *binding.ScopeOrgID, false)
		if err != nil {
			return err
		}
		addObjectIDs(scope.readableObjectIDs, objectIDs)
		return nil
	case auth.RoleAssessmentAdmin:
		addObjectID(scope.readableObjectIDs, binding.PersonObjectID)
		addObjectID(scope.detailObjectIDs, binding.PersonObjectID)
		addObjectID(scope.readableObjectIDs, binding.TeamObjectID)
		addObjectID(scope.detailObjectIDs, binding.TeamObjectID)
		if binding.ScopeOrgID == nil || strings.TrimSpace(binding.ScopeOrgType) == "" {
			return nil
		}
		objectIDs, err := resolveScopeObjectIDs(ctx, db, binding.ScopeOrgType, *binding.ScopeOrgID, true)
		if err != nil {
			return err
		}
		addObjectIDs(scope.readableObjectIDs, objectIDs)
		addObjectIDs(scope.detailObjectIDs, objectIDs)
		return nil
	default:
		return nil
	}
}

func (s *assessmentAccessScope) allowsReadableObject(objectID uint) bool {
	if s == nil || objectID == 0 {
		return false
	}
	if s.unrestricted {
		return true
	}
	_, ok := s.readableObjectIDs[objectID]
	return ok
}

func (s *assessmentAccessScope) allowsDetailObject(objectID uint) bool {
	if s == nil || objectID == 0 {
		return false
	}
	if s.unrestricted {
		return true
	}
	_, ok := s.detailObjectIDs[objectID]
	return ok
}

func (s *assessmentAccessScope) applyReadableObjectFilter(query *gorm.DB, columnName string) *gorm.DB {
	if s == nil {
		return query.Where("1 = 0")
	}
	if s.unrestricted {
		return query
	}
	objectIDs := setToSortedUintSlice(s.readableObjectIDs)
	if len(objectIDs) == 0 {
		return query.Where("1 = 0")
	}
	if strings.TrimSpace(columnName) == "" {
		columnName = "object_id"
	}
	return query.Where(fmt.Sprintf("%s IN ?", columnName), objectIDs)
}

func (s *assessmentAccessScope) applyDetailObjectFilter(query *gorm.DB, columnName string) *gorm.DB {
	if s == nil {
		return query.Where("1 = 0")
	}
	if s.unrestricted {
		return query
	}
	objectIDs := setToSortedUintSlice(s.detailObjectIDs)
	if len(objectIDs) == 0 {
		return query.Where("1 = 0")
	}
	if strings.TrimSpace(columnName) == "" {
		columnName = "object_id"
	}
	return query.Where(fmt.Sprintf("%s IN ?", columnName), objectIDs)
}

func resolveScopeObjectIDs(
	ctx context.Context,
	db *gorm.DB,
	scopeType string,
	scopeID uint,
	includeDescendants bool,
) ([]uint, error) {
	normalizedType := strings.ToLower(strings.TrimSpace(scopeType))
	switch normalizedType {
	case "organization", "org", "group", "company":
		orgIDs, err := resolveOrganizationIDs(ctx, db, scopeID, includeDescendants)
		if err != nil {
			return nil, err
		}
		return loadAssessmentObjectIDsByOrganizationIDs(ctx, db, orgIDs)
	case "department", "dept":
		departmentIDs, err := resolveDepartmentIDs(ctx, db, scopeID, includeDescendants)
		if err != nil {
			return nil, err
		}
		return loadAssessmentObjectIDsByDepartmentIDs(ctx, db, departmentIDs)
	default:
		return []uint{}, nil
	}
}

func resolveOrganizationIDs(ctx context.Context, db *gorm.DB, organizationID uint, includeDescendants bool) ([]uint, error) {
	if organizationID == 0 {
		return []uint{}, nil
	}

	var ids []uint
	if includeDescendants {
		err := db.WithContext(ctx).Raw(`
WITH RECURSIVE org_tree(id) AS (
    SELECT id
    FROM organizations
    WHERE id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT o.id
    FROM organizations o
    JOIN org_tree ot ON o.parent_id = ot.id
    WHERE o.deleted_at IS NULL
)
SELECT id FROM org_tree
`, organizationID).Scan(&ids).Error
		if err != nil {
			return nil, fmt.Errorf("failed to resolve descendant organizations: %w", err)
		}
		return ids, nil
	}

	err := db.WithContext(ctx).Table("organizations").
		Where("deleted_at IS NULL AND (id = ? OR parent_id = ?)", organizationID, organizationID).
		Pluck("id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("failed to resolve direct-scope organizations: %w", err)
	}
	return ids, nil
}

func resolveDepartmentIDs(ctx context.Context, db *gorm.DB, departmentID uint, includeDescendants bool) ([]uint, error) {
	if departmentID == 0 {
		return []uint{}, nil
	}

	var ids []uint
	if includeDescendants {
		err := db.WithContext(ctx).Raw(`
WITH RECURSIVE dept_tree(id) AS (
    SELECT id
    FROM departments
    WHERE id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT d.id
    FROM departments d
    JOIN dept_tree dt ON d.parent_dept_id = dt.id
    WHERE d.deleted_at IS NULL
)
SELECT id FROM dept_tree
`, departmentID).Scan(&ids).Error
		if err != nil {
			return nil, fmt.Errorf("failed to resolve descendant departments: %w", err)
		}
		return ids, nil
	}

	err := db.WithContext(ctx).Table("departments").
		Where("deleted_at IS NULL AND (id = ? OR parent_dept_id = ?)", departmentID, departmentID).
		Pluck("id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("failed to resolve direct-scope departments: %w", err)
	}
	return ids, nil
}

func loadAssessmentObjectIDsByOrganizationIDs(ctx context.Context, db *gorm.DB, organizationIDs []uint) ([]uint, error) {
	if len(organizationIDs) == 0 {
		return []uint{}, nil
	}

	var objectIDs []uint
	err := db.WithContext(ctx).Table("assessment_objects ao").
		Distinct("ao.id").
		Where("ao.is_active = 1").
		Where(
			`(ao.target_type IN ? AND ao.target_id IN ?)
OR (ao.target_type = 'department' AND ao.target_id IN (
    SELECT d.id
    FROM departments d
    WHERE d.deleted_at IS NULL AND d.organization_id IN ?
))
OR (ao.target_type = 'employee' AND ao.target_id IN (
    SELECT e.id
    FROM employees e
    WHERE e.deleted_at IS NULL AND e.organization_id IN ?
))`,
			[]string{"organization", "leadership_team"},
			organizationIDs,
			organizationIDs,
			organizationIDs,
		).
		Pluck("ao.id", &objectIDs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query assessment objects by organization scope: %w", err)
	}
	return objectIDs, nil
}

func loadAssessmentObjectIDsByDepartmentIDs(ctx context.Context, db *gorm.DB, departmentIDs []uint) ([]uint, error) {
	if len(departmentIDs) == 0 {
		return []uint{}, nil
	}

	var objectIDs []uint
	err := db.WithContext(ctx).Table("assessment_objects ao").
		Distinct("ao.id").
		Where("ao.is_active = 1").
		Where(
			`(ao.target_type = 'department' AND ao.target_id IN ?)
OR (ao.target_type = 'employee' AND ao.target_id IN (
    SELECT e.id
    FROM employees e
    WHERE e.deleted_at IS NULL AND e.department_id IN ?
))`,
			departmentIDs,
			departmentIDs,
		).
		Pluck("ao.id", &objectIDs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query assessment objects by department scope: %w", err)
	}
	return objectIDs, nil
}

func addObjectID(set map[uint]struct{}, objectID *uint) {
	if set == nil || objectID == nil || *objectID == 0 {
		return
	}
	set[*objectID] = struct{}{}
}

func addObjectIDs(set map[uint]struct{}, objectIDs []uint) {
	if set == nil {
		return
	}
	for _, objectID := range objectIDs {
		if objectID == 0 {
			continue
		}
		set[objectID] = struct{}{}
	}
}

func setToSortedUintSlice(set map[uint]struct{}) []uint {
	if len(set) == 0 {
		return []uint{}
	}
	result := make([]uint, 0, len(set))
	for key := range set {
		result = append(result, key)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}
