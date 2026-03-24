package middleware

import (
	"fmt"
	"net/http"
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
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
			c.Abort()
			return
		}

		c.Set(orgScopesContextKey, claims.OrgScopes)
		if auth.HasBusinessRole(claims.Roles, auth.RoleRoot) || auth.HasPermission(claims.Permissions, "*") {
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
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid organizationId")
			c.Abort()
			return
		}
		if !containsOrgScope(claims.OrgScopes, orgType, uint(orgID)) {
			message := fmt.Sprintf(
				"organization scope denied (Code 40302). 当前用户没有权限访问该组织范围: organizationType=%s, organizationId=%d。",
				orgType,
				orgID,
			)
			response.Error(c, http.StatusForbidden, response.CodeForbiddenOrgScope, message)
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
