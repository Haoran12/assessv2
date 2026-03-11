package handler

import (
	"time"

	"assessv2/backend/internal/api/response"
	"github.com/gin-gonic/gin"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) Health(c *gin.Context) {
	response.Success(c, gin.H{
		"service":   "assessv2-backend",
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
