package middleware

import (
	"log"
	"time"

	"assessv2/backend/internal/trace"
	"github.com/gin-gonic/gin"
)

func AccessLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		traceID := trace.FromContext(c)
		userID := uint(0)
		if claims, ok := ClaimsFromContext(c); ok {
			userID = claims.UserID
		}

		if len(c.Errors) > 0 {
			log.Printf(
				"level=error trace_id=%s method=%s path=%s query=%q status=%d latency_ms=%d client_ip=%s user_id=%d error=%q",
				traceID,
				c.Request.Method,
				path,
				query,
				c.Writer.Status(),
				latency.Milliseconds(),
				c.ClientIP(),
				userID,
				c.Errors.String(),
			)
			return
		}

		log.Printf(
			"level=info trace_id=%s method=%s path=%s query=%q status=%d latency_ms=%d client_ip=%s user_id=%d",
			traceID,
			c.Request.Method,
			path,
			query,
			c.Writer.Status(),
			latency.Milliseconds(),
			c.ClientIP(),
			userID,
		)
	}
}
