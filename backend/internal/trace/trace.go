package trace

import "github.com/gin-gonic/gin"

const (
	ContextKey      = "traceID"
	TraceHeader     = "X-Trace-ID"
	RequestIDHeader = "X-Request-ID"
)

func FromContext(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return c.GetString(ContextKey)
}
