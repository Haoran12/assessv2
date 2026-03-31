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

type AssessmentHandler struct {
	service *service.AssessmentSessionService
}

func NewAssessmentHandler(sessionService *service.AssessmentSessionService) *AssessmentHandler {
	return &AssessmentHandler{service: sessionService}
}

type createAssessmentSessionRequest struct {
	Year              int    `json:"year"`
	OrganizationID    uint   `json:"organizationId"`
	DisplayName       string `json:"displayName"`
	Description       string `json:"description"`
	CopyFromSessionID uint   `json:"copyFromSessionId"`
}

type updateAssessmentSessionRequest struct {
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
}

type updateAssessmentSessionStatusRequest struct {
	Status string `json:"status"`
}

type updatePeriodsRequest struct {
	Items []service.SessionPeriodItem `json:"items"`
}

type updateObjectGroupsRequest struct {
	Items []service.SessionObjectGroupItem `json:"items"`
}

type updateObjectsRequest struct {
	Items []service.SessionObjectUpsertItem `json:"items"`
}

type updateModuleScoresRequest struct {
	Items []service.SessionObjectModuleScoreUpsertItem `json:"items"`
}

func (h *AssessmentHandler) ListSessions(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	items, err := h.service.ListSessions(c.Request.Context(), claims)
	if err != nil {
		h.handleAssessmentError(c, err, "failed to query assessment sessions")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AssessmentHandler) CreateSession(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	var req createAssessmentSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid assessment session payload")
		return
	}
	result, err := h.service.CreateSession(
		c.Request.Context(),
		claims,
		operatorID,
		service.CreateAssessmentSessionInput{
			Year:              req.Year,
			OrganizationID:    req.OrganizationID,
			DisplayName:       strings.TrimSpace(req.DisplayName),
			Description:       strings.TrimSpace(req.Description),
			CopyFromSessionID: req.CopyFromSessionID,
		},
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		h.handleAssessmentError(c, err, "failed to create assessment session")
		return
	}
	response.Success(c, result)
}

func (h *AssessmentHandler) GetSession(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	sessionID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid assessment id")
		return
	}
	result, err := h.service.GetSession(c.Request.Context(), claims, sessionID)
	if err != nil {
		h.handleAssessmentError(c, err, "failed to query assessment session")
		return
	}
	response.Success(c, result)
}

func (h *AssessmentHandler) UpdateSession(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	sessionID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid assessment id")
		return
	}
	var req updateAssessmentSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid assessment payload")
		return
	}
	result, err := h.service.UpdateSession(
		c.Request.Context(),
		claims,
		operatorID,
		sessionID,
		service.UpdateAssessmentSessionInput{
			DisplayName: strings.TrimSpace(req.DisplayName),
			Description: strings.TrimSpace(req.Description),
		},
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		h.handleAssessmentError(c, err, "failed to update assessment session")
		return
	}
	response.Success(c, result)
}

func (h *AssessmentHandler) UpdateSessionStatus(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	sessionID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid assessment id")
		return
	}
	var req updateAssessmentSessionStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid assessment status payload")
		return
	}
	result, err := h.service.UpdateSessionStatus(
		c.Request.Context(),
		claims,
		operatorID,
		sessionID,
		service.UpdateAssessmentSessionStatusInput{
			Status: strings.TrimSpace(req.Status),
		},
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		h.handleAssessmentError(c, err, "failed to update assessment session status")
		return
	}
	response.Success(c, result)
}

func (h *AssessmentHandler) ListObjects(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	sessionID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid assessment id")
		return
	}
	items, err := h.service.ListObjects(c.Request.Context(), claims, sessionID)
	if err != nil {
		h.handleAssessmentError(c, err, "failed to list assessment objects")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AssessmentHandler) ListCalculatedObjects(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	sessionID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid assessment id")
		return
	}
	periodCode := strings.TrimSpace(c.Query("periodCode"))
	objectGroupCode := strings.TrimSpace(c.Query("objectGroupCode"))
	if periodCode == "" || objectGroupCode == "" {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "periodCode and objectGroupCode are required")
		return
	}
	items, err := h.service.ListCalculatedObjects(
		c.Request.Context(),
		claims,
		sessionID,
		periodCode,
		objectGroupCode,
	)
	if err != nil {
		h.handleAssessmentError(c, err, "failed to list calculated assessment objects")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AssessmentHandler) UpsertModuleScores(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	sessionID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid assessment id")
		return
	}
	var req updateModuleScoresRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid module scores payload")
		return
	}
	items, err := h.service.UpsertModuleScores(
		c.Request.Context(),
		claims,
		operatorID,
		sessionID,
		req.Items,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		h.handleAssessmentError(c, err, "failed to upsert module scores")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AssessmentHandler) ListObjectCandidates(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	sessionID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid assessment id")
		return
	}
	items, err := h.service.ListObjectCandidates(c.Request.Context(), claims, sessionID, c.Query("keyword"))
	if err != nil {
		h.handleAssessmentError(c, err, "failed to list assessment object candidates")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AssessmentHandler) ReplaceObjects(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	sessionID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid assessment id")
		return
	}
	var req updateObjectsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid assessment objects payload")
		return
	}
	items, err := h.service.ReplaceObjects(
		c.Request.Context(),
		claims,
		operatorID,
		sessionID,
		req.Items,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		h.handleAssessmentError(c, err, "failed to replace assessment objects")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AssessmentHandler) ReplacePeriods(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	sessionID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid assessment id")
		return
	}
	var req updatePeriodsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid periods payload")
		return
	}
	items, err := h.service.ReplacePeriods(
		c.Request.Context(),
		claims,
		operatorID,
		sessionID,
		req.Items,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		h.handleAssessmentError(c, err, "failed to replace periods")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AssessmentHandler) ReplaceObjectGroups(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	sessionID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid assessment id")
		return
	}
	var req updateObjectGroupsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid object groups payload")
		return
	}
	items, err := h.service.ReplaceObjectGroups(
		c.Request.Context(),
		claims,
		operatorID,
		sessionID,
		req.Items,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		h.handleAssessmentError(c, err, "failed to replace object groups")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AssessmentHandler) ResetObjects(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	sessionID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid assessment id")
		return
	}
	items, err := h.service.ResetObjectsToDefault(
		c.Request.Context(),
		claims,
		operatorID,
		sessionID,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		h.handleAssessmentError(c, err, "failed to reset assessment objects")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AssessmentHandler) handleAssessmentError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrInvalidParam),
		errors.Is(err, service.ErrInvalidScoreModule),
		errors.Is(err, service.ErrInvalidExtraPointValue),
		errors.Is(err, service.ErrInvalidSessionStatus),
		errors.Is(err, service.ErrInvalidPeriodTemplate),
		errors.Is(err, service.ErrInvalidRuleObjectType),
		errors.Is(err, service.ErrInvalidRuleObjectCategory):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
	case errors.Is(err, service.ErrAssessmentReadOnly):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
	case errors.Is(err, service.ErrCalcDependencyCycle),
		errors.Is(err, service.ErrCalcExpressionEval):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
	case errors.Is(err, service.ErrInvalidExpression):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
	case errors.Is(err, service.ErrForbidden):
		response.Error(c, http.StatusForbidden, response.CodeForbidden, err.Error())
	case errors.Is(err, service.ErrYearNotFound),
		errors.Is(err, service.ErrOrganizationNotFound),
		errors.Is(err, service.ErrPeriodNotFound),
		errors.Is(err, service.ErrRuleNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	case errors.Is(err, service.ErrYearAlreadyExists):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, fallback)
	}
}
