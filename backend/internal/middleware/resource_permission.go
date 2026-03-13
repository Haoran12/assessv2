package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"assessv2/backend/internal/api/response"
	"assessv2/backend/internal/auth"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ResourceInfo contains information needed for resource permission check
type ResourceInfo struct {
	OwnerID         uint
	PermissionMode  uint16
	OrgType         string
	OrgID           uint
}

// ResourceLoader is a function that loads resource information from database
type ResourceLoader func(ctx context.Context, db *gorm.DB, resourceID uint) (*ResourceInfo, error)

// RequireResourcePermission creates a middleware that checks resource-level permissions
// root role always bypasses resource permission checks
func RequireResourcePermission(
	db *gorm.DB,
	action string,
	idExtractor func(*gin.Context) (uint, error),
	resourceLoader ResourceLoader,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := ClaimsFromContext(c)
		if !ok {
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
			c.Abort()
			return
		}

		// root role bypasses all resource-level checks
		if auth.HasRole(claims.Roles, "root") {
			c.Next()
			return
		}

		resourceID, err := idExtractor(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid resource id")
			c.Abort()
			return
		}

		// Load resource information
		resourceInfo, err := resourceLoader(c.Request.Context(), db, resourceID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				response.Error(c, http.StatusNotFound, response.CodeNotFound, "resource not found")
			} else {
				response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to load resource")
			}
			c.Abort()
			return
		}

		// Check resource permission
		allowed := auth.CheckResourcePermission(
			claims.UserID,
			claims.Roles,
			resourceInfo.OwnerID,
			resourceInfo.PermissionMode,
			claims.OrgScopes,
			resourceInfo.OrgType,
			resourceInfo.OrgID,
			action,
		)

		if !allowed {
			response.Error(c, http.StatusForbidden, response.CodeForbidden, "resource permission denied")
			c.Abort()
			return
		}

		c.Next()
	}
}

// Common ID extractors

// ExtractIDFromPath extracts resource ID from path parameter
func ExtractIDFromPath(paramName string) func(*gin.Context) (uint, error) {
	return func(c *gin.Context) (uint, error) {
		idStr := c.Param(paramName)
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid %s parameter", paramName)
		}
		return uint(id), nil
	}
}

// ExtractIDFromQuery extracts resource ID from query parameter
func ExtractIDFromQuery(paramName string) func(*gin.Context) (uint, error) {
	return func(c *gin.Context) (uint, error) {
		idStr := c.Query(paramName)
		if idStr == "" {
			return 0, fmt.Errorf("missing %s parameter", paramName)
		}
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid %s parameter", paramName)
		}
		return uint(id), nil
	}
}
