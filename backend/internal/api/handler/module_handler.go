package handler

import (
	"assessv2/backend/internal/api/response"
	"github.com/gin-gonic/gin"
)

type ModuleHandler struct{}

func NewModuleHandler() *ModuleHandler {
	return &ModuleHandler{}
}

func (h *ModuleHandler) ModulePing(moduleName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		response.Success(c, gin.H{
			"module": moduleName,
			"status": "initialized",
		})
	}
}
