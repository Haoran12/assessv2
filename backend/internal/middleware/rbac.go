package middleware

import (
	"net/http"

	"assessv2/backend/internal/api/response"
	"assessv2/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := ClaimsFromContext(c)
		if !ok {
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
			c.Abort()
			return
		}
		if !auth.RoleAllowsPermission(claims.Roles, permission) && !auth.HasPermission(claims.Permissions, permission) {
			response.Error(c, http.StatusForbidden, response.CodeForbidden, "Request failed with Code 403: Permission Denied")
			c.Abort()
			return
		}
		c.Next()
	}
}

func RequireRoot() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := ClaimsFromContext(c)
		if !ok {
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
			c.Abort()
			return
		}
		if !auth.HasRole(claims.Roles, "root") {
			response.Error(c, http.StatusForbidden, response.CodeForbidden, "Request failed with Code 403: Permission Denied")
			c.Abort()
			return
		}
		c.Next()
	}
}
