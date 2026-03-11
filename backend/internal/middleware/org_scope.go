package middleware

import (
	"strconv"

	"assessv2/backend/internal/api/response"
	"assessv2/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

const orgScopesContextKey = "orgScopes"

func RequireOrgScope() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := ClaimsFromContext(c)
		if !ok {
			response.Error(c, 401, 40100, "missing auth context")
			c.Abort()
			return
		}

		c.Set(orgScopesContextKey, claims.OrgScopes)
		if auth.HasPermission(claims.Permissions, "*") {
			c.Next()
			return
		}

		orgType := c.Query("organizationType")
		orgIDRaw := c.Query("organizationId")
		if orgType == "" || orgIDRaw == "" {
			c.Next()
			return
		}

		orgID, err := strconv.ParseUint(orgIDRaw, 10, 64)
		if err != nil {
			response.Error(c, 400, 40002, "invalid organizationId")
			c.Abort()
			return
		}
		if !containsOrgScope(claims.OrgScopes, orgType, uint(orgID)) {
			response.Error(c, 403, 40302, "organization scope denied")
			c.Abort()
			return
		}
		c.Next()
	}
}

func containsOrgScope(scopes []auth.OrganizationScope, organizationType string, organizationID uint) bool {
	for _, scope := range scopes {
		if scope.OrganizationType == organizationType && scope.OrganizationID == organizationID {
			return true
		}
	}
	return false
}
