package middleware

import (
	"assessv2/backend/internal/api/response"
	"assessv2/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := ClaimsFromContext(c)
		if !ok {
			response.Error(c, 401, 40100, "missing auth context")
			c.Abort()
			return
		}
		if !auth.HasPermission(claims.Permissions, permission) {
			response.Error(c, 403, 40301, "permission denied")
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
			response.Error(c, 401, 40100, "missing auth context")
			c.Abort()
			return
		}
		if !auth.HasRole(claims.Roles, "root") {
			response.Error(c, 403, 40301, "root role required")
			c.Abort()
			return
		}
		c.Next()
	}
}
