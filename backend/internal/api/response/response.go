package response

import "github.com/gin-gonic/gin"

type Payload struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func Success(c *gin.Context, data any) {
	c.JSON(200, Payload{
		Code:    200,
		Message: "success",
		Data:    data,
	})
}

func Error(c *gin.Context, httpStatus int, code int, message string) {
	c.JSON(httpStatus, Payload{
		Code:    code,
		Message: message,
		Data:    gin.H{},
	})
}
