package response

import (
	"net/http"

	"assessv2/backend/internal/trace"
	"github.com/gin-gonic/gin"
)

type Payload struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	TraceID string `json:"traceId,omitempty"`
}

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Payload{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
		TraceID: trace.FromContext(c),
	})
}

func Error(c *gin.Context, httpStatus int, code int, message string) {
	c.JSON(httpStatus, Payload{
		Code:    code,
		Message: message,
		Data:    gin.H{},
		TraceID: trace.FromContext(c),
	})
}
