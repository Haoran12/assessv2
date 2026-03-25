package middleware

import (
	"fmt"
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
			message := fmt.Sprintf(
				"Request Failed 40301: 当前账号缺少权限「%s」，请联系管理员授权后重试。",
				permission,
			)
			response.Error(c, http.StatusForbidden, response.CodeForbidden, message)
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
			response.Error(c, http.StatusForbidden, response.CodeForbidden, "Request Failed 40301: 当前操作仅允许 root 角色执行，请联系管理员处理。")
			c.Abort()
			return
		}
		c.Next()
	}
}
