package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"assessv2/backend/internal/trace"
	"github.com/gin-gonic/gin"
)

func RequestContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(trace.RequestIDHeader))
		if requestID == "" {
			requestID = generateTraceID()
		}

		c.Set(trace.ContextKey, requestID)
		c.Writer.Header().Set(trace.TraceHeader, requestID)
		c.Next()
	}
}

func generateTraceID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
