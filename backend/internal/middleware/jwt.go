package middleware

import (
	"strings"

	"assessv2/backend/internal/api/response"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

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

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			response.Error(c, 401, 40100, "invalid token")
			c.Abort()
			return
		}

		c.Set("jwtClaims", token.Claims)
		c.Next()
	}
}
