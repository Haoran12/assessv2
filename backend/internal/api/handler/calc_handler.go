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

type CalcHandler struct {
	calcService *service.CalculationService
}

type recalculateRequest struct {
	YearID         uint   `json:"yearId"`
	PeriodCode     string `json:"periodCode"`
	ObjectIDs      []uint `json:"objectIds"`
	ObjectType     string `json:"objectType"`
	ObjectCategory string `json:"objectCategory"`
	TargetType     string `json:"targetType"`
	TargetID       *uint  `json:"targetId"`
}

func NewCalcHandler(calcService *service.CalculationService) *CalcHandler {
	return &CalcHandler{calcService: calcService}
}

func (h *CalcHandler) Recalculate(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	var req recalculateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid recalculate payload")
		return
	}
	trigger := "manual"
	result, err := h.calcService.Recalculate(c.Request.Context(), &operatorID, service.RecalculateInput{
		YearID:         req.YearID,
		PeriodCode:     strings.TrimSpace(req.PeriodCode),
		ObjectIDs:      req.ObjectIDs,
		ObjectType:     strings.TrimSpace(req.ObjectType),
		ObjectCategory: strings.TrimSpace(req.ObjectCategory),
		TargetType:     strings.TrimSpace(req.TargetType),
		TargetID:       req.TargetID,
		TriggerMode:    trigger,
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleCalcError(c, err, "failed to recalculate scores")
		return
	}
	response.Success(c, result)
}

func (h *CalcHandler) ListCalculatedScores(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	yearID, err := parseOptionalUintQuery(c.Query("yearId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid yearId")
		return
	}
	objectID, err := parseOptionalUintQuery(c.Query("objectId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid objectId")
		return
	}

	items, err := h.calcService.ListCalculatedScores(c.Request.Context(), claims, service.ListCalculatedScoreFilter{
		YearID:         yearID,
		PeriodCode:     strings.TrimSpace(c.Query("periodCode")),
		ObjectID:       objectID,
		ObjectType:     strings.TrimSpace(c.Query("objectType")),
		ObjectCategory: strings.TrimSpace(c.Query("objectCategory")),
	})
	if err != nil {
		h.handleCalcError(c, err, "failed to list calculated scores")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *CalcHandler) ListCalculatedModuleScores(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	calculatedScoreID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid calculated score id")
		return
	}
	items, err := h.calcService.ListCalculatedModuleScores(c.Request.Context(), claims, calculatedScoreID)
	if err != nil {
		h.handleCalcError(c, err, "failed to list calculated module scores")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *CalcHandler) ListRankings(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	yearID, err := parseOptionalUintQuery(c.Query("yearId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid yearId")
		return
	}

	items, err := h.calcService.ListRankings(c.Request.Context(), claims, service.ListRankingFilter{
		YearID:         yearID,
		PeriodCode:     strings.TrimSpace(c.Query("periodCode")),
		RankingScope:   strings.TrimSpace(c.Query("scope")),
		ScopeKey:       strings.TrimSpace(c.Query("scopeKey")),
		ObjectType:     strings.TrimSpace(c.Query("objectType")),
		ObjectCategory: strings.TrimSpace(c.Query("objectCategory")),
	})
	if err != nil {
		h.handleCalcError(c, err, "failed to list rankings")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *CalcHandler) handleCalcError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrInvalidParam),
		errors.Is(err, service.ErrCalcExpressionEval):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
	case errors.Is(err, service.ErrCalcDependencyCycle),
		errors.Is(err, service.ErrAssessmentReadOnly),
		errors.Is(err, service.ErrAssessmentNotActive),
		errors.Is(err, service.ErrPeriodNotActive):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
	case errors.Is(err, service.ErrForbidden):
		response.Error(c, http.StatusForbidden, response.CodeForbidden, err.Error())
	case errors.Is(err, service.ErrAssessmentObjectNotFound),
		errors.Is(err, service.ErrPeriodNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, fallback)
	}
}
