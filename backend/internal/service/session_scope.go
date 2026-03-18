package service

import (
	"strings"

	"assessv2/backend/internal/auth"
)

func resolveAdminOrganizationID(claims *auth.Claims) (uint, error) {
	if claims == nil {
		return 0, ErrForbidden
	}
	if isRootClaims(claims) {
		return 0, nil
	}
	if !auth.HasBusinessRole(claims.Roles, auth.RoleAssessmentAdmin) {
		return 0, ErrForbidden
	}

	var fallback uint
	for _, scope := range claims.OrgScopes {
		scopeType := strings.ToLower(strings.TrimSpace(scope.OrganizationType))
		if scope.OrganizationID == 0 {
			continue
		}
		if scopeType != "organization" && scopeType != "org" && scopeType != "company" && scopeType != "group" {
			continue
		}
		if scope.IsPrimary {
			return scope.OrganizationID, nil
		}
		if fallback == 0 {
			fallback = scope.OrganizationID
		}
	}
	if fallback == 0 {
		return 0, ErrForbidden
	}
	return fallback, nil
}

func ensureAssessmentOrganizationScope(claims *auth.Claims, organizationID uint) error {
	if claims == nil {
		return ErrForbidden
	}
	if organizationID == 0 {
		return ErrInvalidParam
	}
	if isRootClaims(claims) {
		return nil
	}
	adminOrgID, err := resolveAdminOrganizationID(claims)
	if err != nil {
		return err
	}
	if adminOrgID != organizationID {
		return ErrForbidden
	}
	return nil
}
