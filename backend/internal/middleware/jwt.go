package middleware

import (
	"strings"

	"assessv2/backend/internal/api/response"
	"assessv2/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

const claimsContextKey = "authClaims"

func RequireJWT(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, 401, 40100, "missing authorization header")
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			response.Error(c, 401, 40100, "invalid authorization header format")
			c.Abort()
			return
		}

		claims, err := auth.ParseToken(secret, tokenString)
		if err != nil {
			response.Error(c, 401, 40100, "invalid token")
			c.Abort()
			return
		}

		c.Set(claimsContextKey, claims)
		c.Next()
	}
}

func ClaimsFromContext(c *gin.Context) (*auth.Claims, bool) {
	value, ok := c.Get(claimsContextKey)
	if !ok {
		return nil, false
	}
	claims, ok := value.(*auth.Claims)
	return claims, ok
}
