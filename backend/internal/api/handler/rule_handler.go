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

func (h *RuleHandler) GetExpressionContext(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	assessmentID, err := parseOptionalUintQuery(c.Query("assessmentId"))
	if err != nil || assessmentID == nil || *assessmentID == 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid assessmentId")
		return
	}
	result, err := h.service.GetRuleExpressionContext(
		c.Request.Context(),
		claims,
		*assessmentID,
		c.Query("periodCode"),
		c.Query("objectGroupCode"),
	)
	if err != nil {
		h.handleRuleError(c, err, "failed to query expression context")
		return
	}
	response.Success(c, result)
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

func (h *RuleHandler) CheckRuleDependencies(c *gin.Context) {
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
	result, err := h.service.CheckRuleDependencies(c.Request.Context(), claims, ruleID)
	if err != nil {
		h.handleRuleError(c, err, "failed to check rule dependencies")
		return
	}
	response.Success(c, result)
}

func (h *RuleHandler) handleRuleError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrInvalidParam),
		errors.Is(err, service.ErrInvalidRuleName),
		errors.Is(err, service.ErrInvalidRuleObjectCategory):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
	case errors.Is(err, service.ErrInvalidExpression),
		errors.Is(err, service.ErrCalcExpressionEval):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
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
