package auth

import (
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type OrganizationScope struct {
	OrganizationType string `json:"organizationType"`
	OrganizationID   uint   `json:"organizationId"`
	RoleInOrg        string `json:"roleInOrg,omitempty"`
	IsPrimary        bool   `json:"isPrimary"`
}

type PermissionBinding struct {
	RoleCode       string `json:"roleCode"`
	ScopeOrgType   string `json:"scopeOrgType,omitempty"`
	ScopeOrgID     *uint  `json:"scopeOrgId,omitempty"`
	PersonObjectID *uint  `json:"personObjectId,omitempty"`
	TeamObjectID   *uint  `json:"teamObjectId,omitempty"`
	IsPrimary      bool   `json:"isPrimary"`
}

type Claims struct {
	UserID             uint                `json:"uid"`
	Username           string              `json:"username"`
	Roles              []string            `json:"roles"`
	Permissions        []string            `json:"permissions"`
	OrgScopes          []OrganizationScope `json:"orgScopes"`
	PermissionBindings []PermissionBinding `json:"permissionBindings"`
	jwt.RegisteredClaims
}

func SignToken(secret string, claims Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseToken(secret, tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("invalid signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func HasPermission(granted []string, required string) bool {
	if required == "" {
		return true
	}
	for _, permission := range granted {
		if permission == "*" || permission == required {
			return true
		}
		if strings.HasSuffix(permission, "*") {
			prefix := strings.TrimSuffix(permission, "*")
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}
	}
	return false
}

func HasRole(granted []string, required string) bool {
	if required == "" {
		return true
	}
	for _, role := range granted {
		if role == required {
			return true
		}
	}
	return false
}
