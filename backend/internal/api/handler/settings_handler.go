package handler

import (
	"net/http"
	"strings"

	"assessv2/backend/internal/api/response"
	"assessv2/backend/internal/middleware"
	"assessv2/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type SettingsHandler struct {
	settingsService *service.SystemSettingService
}

type updateSettingsRequest struct {
	Items []struct {
		SettingKey   string `json:"settingKey"`
		SettingValue any    `json:"settingValue"`
	} `json:"items"`
}

func NewSettingsHandler(settingsService *service.SystemSettingService) *SettingsHandler {
	return &SettingsHandler{
		settingsService: settingsService,
	}
}

func (h *SettingsHandler) List(c *gin.Context) {
	result, err := h.settingsService.List(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to query settings")
		return
	}
	response.Success(c, result)
}

func (h *SettingsHandler) Update(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid settings payload")
		return
	}

	items := make([]service.UpdateSystemSettingItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, service.UpdateSystemSettingItem{
			SettingKey:   strings.TrimSpace(item.SettingKey),
			SettingValue: item.SettingValue,
		})
	}

	result, err := h.settingsService.Update(
		c.Request.Context(),
		claims.UserID,
		items,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		switch err {
		case service.ErrInvalidSettingKey, service.ErrInvalidSettingValue:
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to update settings")
		}
		return
	}
	response.Success(c, result)
}
