package handler

import (
	"net/http"
	"strconv"
	"strings"

	"assessv2/backend/internal/api/response"
	"assessv2/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type AuditHandler struct {
	auditService *service.AuditService
}

func NewAuditHandler(auditService *service.AuditService) *AuditHandler {
	return &AuditHandler{
		auditService: auditService,
	}
}

func (h *AuditHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	userID, err := parseOptionalUintQuery(c.Query("userId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid userId")
		return
	}
	startAt, err := parseOptionalInt64Query(c.Query("startAt"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid startAt")
		return
	}
	endAt, err := parseOptionalInt64Query(c.Query("endAt"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid endAt")
		return
	}

	result, err := h.auditService.List(c.Request.Context(), service.AuditLogListInput{
		Page:       page,
		PageSize:   pageSize,
		UserID:     userID,
		ActionType: strings.TrimSpace(c.Query("actionType")),
		TargetType: strings.TrimSpace(c.Query("targetType")),
		Keyword:    strings.TrimSpace(c.Query("keyword")),
		StartAt:    startAt,
		EndAt:      endAt,
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to query audit logs")
		return
	}
	response.Success(c, result)
}

func (h *AuditHandler) Detail(c *gin.Context) {
	auditID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid audit id")
		return
	}

	result, err := h.auditService.Detail(c.Request.Context(), auditID)
	if err != nil {
		if err == service.ErrAuditLogNotFound {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to query audit detail")
		return
	}
	response.Success(c, result)
}

func parseOptionalInt64Query(raw string) (*int64, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return nil, err
	}
	return &value, nil
}
