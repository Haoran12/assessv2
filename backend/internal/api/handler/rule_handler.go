package handler

import (
	"errors"
	"net/http"
	"strings"

	"assessv2/backend/internal/api/response"
	"assessv2/backend/internal/middleware"
	"assessv2/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type RuleHandler struct {
	service *service.RuleManagementService
}

func NewRuleHandler(ruleService *service.RuleManagementService) *RuleHandler {
	return &RuleHandler{service: ruleService}
}

type ruleFileRequest struct {
	AssessmentID uint   `json:"assessmentId"`
	RuleName     string `json:"ruleName"`
	Description  string `json:"description"`
	ContentJSON  string `json:"contentJson"`
}

type selectBindingRequest struct {
	AssessmentID    uint   `json:"assessmentId"`
	PeriodCode      string `json:"periodCode"`
	ObjectGroupCode string `json:"objectGroupCode"`
	SourceRuleID    uint   `json:"sourceRuleId"`
}

func (h *RuleHandler) ListRuleFiles(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	assessmentID, err := parseOptionalUintQuery(c.Query("assessmentId"))
	if err != nil || assessmentID == nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid assessmentId")
		return
	}
	includeHidden := strings.EqualFold(strings.TrimSpace(c.Query("includeHidden")), "true")
	items, err := h.service.ListRuleFiles(c.Request.Context(), claims, service.RuleFileListFilter{
		AssessmentID:  *assessmentID,
		IncludeHidden: includeHidden,
	})
	if err != nil {
		h.handleRuleError(c, err, "failed to list rule files")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *RuleHandler) CreateRuleFile(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	var req ruleFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid rule payload")
		return
	}
	item, err := h.service.CreateRuleFile(c.Request.Context(), claims, operatorID, service.RuleFileInput{
		AssessmentID: req.AssessmentID,
		RuleName:     strings.TrimSpace(req.RuleName),
		Description:  strings.TrimSpace(req.Description),
		ContentJSON:  strings.TrimSpace(req.ContentJSON),
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleRuleError(c, err, "failed to create rule file")
		return
	}
	response.Success(c, item)
}

func (h *RuleHandler) UpdateRuleFile(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	ruleID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid rule id")
		return
	}
	var req ruleFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid rule payload")
		return
	}
	item, err := h.service.UpdateRuleFile(c.Request.Context(), claims, operatorID, ruleID, service.RuleFileInput{
		AssessmentID: req.AssessmentID,
		RuleName:     strings.TrimSpace(req.RuleName),
		Description:  strings.TrimSpace(req.Description),
		ContentJSON:  strings.TrimSpace(req.ContentJSON),
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleRuleError(c, err, "failed to update rule file")
		return
	}
	response.Success(c, item)
}

func (h *RuleHandler) DeleteRuleFile(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	ruleID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid rule id")
		return
	}
	if err := h.service.DeleteRuleFile(c.Request.Context(), claims, operatorID, ruleID, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		h.handleRuleError(c, err, "failed to delete rule file")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *RuleHandler) HideRuleFile(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	ruleID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid rule id")
		return
	}
	if err := h.service.HideRuleFile(c.Request.Context(), claims, operatorID, ruleID); err != nil {
		h.handleRuleError(c, err, "failed to hide rule file")
		return
	}
	response.Success(c, gin.H{"hidden": true})
}

func (h *RuleHandler) UnhideRuleFile(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	ruleID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid rule id")
		return
	}
	if err := h.service.UnhideRuleFile(c.Request.Context(), claims, ruleID); err != nil {
		h.handleRuleError(c, err, "failed to unhide rule file")
		return
	}
	response.Success(c, gin.H{"hidden": false})
}

func (h *RuleHandler) ListBindings(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	assessmentID, err := parseOptionalUintQuery(c.Query("assessmentId"))
	if err != nil || assessmentID == nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid assessmentId")
		return
	}
	items, err := h.service.ListBindings(c.Request.Context(), claims, service.RuleBindingListFilter{
		AssessmentID: *assessmentID,
		PeriodCode:   strings.TrimSpace(c.Query("periodCode")),
	})
	if err != nil {
		h.handleRuleError(c, err, "failed to list rule bindings")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *RuleHandler) SelectBinding(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	var req selectBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid binding payload")
		return
	}
	item, err := h.service.SelectRuleForBinding(c.Request.Context(), claims, operatorID, service.SelectRuleBindingInput{
		AssessmentID:    req.AssessmentID,
		PeriodCode:      strings.TrimSpace(req.PeriodCode),
		ObjectGroupCode: strings.TrimSpace(req.ObjectGroupCode),
		SourceRuleID:    req.SourceRuleID,
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleRuleError(c, err, "failed to bind rule file")
		return
	}
	response.Success(c, item)
}

func (h *RuleHandler) handleRuleError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrInvalidParam),
		errors.Is(err, service.ErrInvalidRuleName),
		errors.Is(err, service.ErrInvalidRuleObjectCategory):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
	case errors.Is(err, service.ErrForbidden):
		response.Error(c, http.StatusForbidden, response.CodeForbidden, err.Error())
	case errors.Is(err, service.ErrRuleNotFound),
		errors.Is(err, service.ErrPeriodNotFound),
		errors.Is(err, service.ErrYearNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, fallback)
	}
}
