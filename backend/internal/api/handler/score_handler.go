package handler

import (
	"errors"
	"net/http"
	"strings"

	"assessv2/backend/internal/api/response"
	"assessv2/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type ScoreHandler struct {
	scoreService *service.ScoreService
}

type directScoreRequest struct {
	YearID     uint    `json:"yearId"`
	PeriodCode string  `json:"periodCode"`
	ModuleID   uint    `json:"moduleId"`
	ObjectID   uint    `json:"objectId"`
	Score      float64 `json:"score"`
	Remark     string  `json:"remark"`
}

type batchDirectScoreEntryRequest struct {
	ObjectID uint    `json:"objectId"`
	Score    float64 `json:"score"`
	Remark   string  `json:"remark"`
}

type batchDirectScoreRequest struct {
	YearID     uint                           `json:"yearId"`
	PeriodCode string                         `json:"periodCode"`
	ModuleID   uint                           `json:"moduleId"`
	Overwrite  bool                           `json:"overwrite"`
	Entries    []batchDirectScoreEntryRequest `json:"entries"`
}

type updateDirectScoreRequest struct {
	Score  float64 `json:"score"`
	Remark string  `json:"remark"`
}

type extraPointRequest struct {
	YearID     uint    `json:"yearId"`
	PeriodCode string  `json:"periodCode"`
	ObjectID   uint    `json:"objectId"`
	PointType  string  `json:"pointType"`
	Points     float64 `json:"points"`
	Reason     string  `json:"reason"`
	Evidence   string  `json:"evidence"`
	Approve    *bool   `json:"approve"`
}

type updateExtraPointRequest struct {
	PointType string  `json:"pointType"`
	Points    float64 `json:"points"`
	Reason    string  `json:"reason"`
	Evidence  string  `json:"evidence"`
	Approve   *bool   `json:"approve"`
}

func NewScoreHandler(scoreService *service.ScoreService) *ScoreHandler {
	return &ScoreHandler{scoreService: scoreService}
}

func (h *ScoreHandler) ListDirectScores(c *gin.Context) {
	yearID, err := parseOptionalUintQuery(c.Query("yearId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid yearId")
		return
	}
	moduleID, err := parseOptionalUintQuery(c.Query("moduleId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid moduleId")
		return
	}
	objectID, err := parseOptionalUintQuery(c.Query("objectId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid objectId")
		return
	}

	items, err := h.scoreService.ListDirectScores(c.Request.Context(), service.ListDirectScoreFilter{
		YearID:     yearID,
		PeriodCode: strings.TrimSpace(c.Query("periodCode")),
		ModuleID:   moduleID,
		ObjectID:   objectID,
	})
	if err != nil {
		h.handleScoreError(c, err, "failed to query direct scores")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *ScoreHandler) CreateDirectScore(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	var req directScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid direct score payload")
		return
	}
	result, err := h.scoreService.CreateDirectScore(c.Request.Context(), operatorID, service.CreateDirectScoreInput{
		YearID:     req.YearID,
		PeriodCode: strings.TrimSpace(req.PeriodCode),
		ModuleID:   req.ModuleID,
		ObjectID:   req.ObjectID,
		Score:      req.Score,
		Remark:     strings.TrimSpace(req.Remark),
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleScoreError(c, err, "failed to create direct score")
		return
	}
	response.Success(c, result)
}

func (h *ScoreHandler) BatchUpsertDirectScores(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	var req batchDirectScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid batch direct score payload")
		return
	}
	entries := make([]service.BatchDirectScoreEntry, 0, len(req.Entries))
	for _, item := range req.Entries {
		entries = append(entries, service.BatchDirectScoreEntry{
			ObjectID: item.ObjectID,
			Score:    item.Score,
			Remark:   strings.TrimSpace(item.Remark),
		})
	}
	result, err := h.scoreService.BatchUpsertDirectScores(c.Request.Context(), operatorID, service.BatchDirectScoreInput{
		YearID:     req.YearID,
		PeriodCode: strings.TrimSpace(req.PeriodCode),
		ModuleID:   req.ModuleID,
		Overwrite:  req.Overwrite,
		Entries:    entries,
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleScoreError(c, err, "failed to batch upsert direct scores")
		return
	}
	response.Success(c, result)
}

func (h *ScoreHandler) UpdateDirectScore(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	scoreID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid direct score id")
		return
	}
	var req updateDirectScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid direct score payload")
		return
	}
	result, err := h.scoreService.UpdateDirectScore(c.Request.Context(), operatorID, scoreID, service.UpdateDirectScoreInput{
		Score:  req.Score,
		Remark: strings.TrimSpace(req.Remark),
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleScoreError(c, err, "failed to update direct score")
		return
	}
	response.Success(c, result)
}

func (h *ScoreHandler) DeleteDirectScore(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	scoreID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid direct score id")
		return
	}
	if err := h.scoreService.DeleteDirectScore(c.Request.Context(), operatorID, scoreID, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		h.handleScoreError(c, err, "failed to delete direct score")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *ScoreHandler) ListExtraPoints(c *gin.Context) {
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
	items, err := h.scoreService.ListExtraPoints(c.Request.Context(), service.ListExtraPointFilter{
		YearID:     yearID,
		PeriodCode: strings.TrimSpace(c.Query("periodCode")),
		ObjectID:   objectID,
		PointType:  strings.TrimSpace(c.Query("pointType")),
	})
	if err != nil {
		h.handleScoreError(c, err, "failed to query extra points")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *ScoreHandler) CreateExtraPoint(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	var req extraPointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid extra point payload")
		return
	}
	result, err := h.scoreService.CreateExtraPoint(c.Request.Context(), operatorID, service.CreateExtraPointInput{
		YearID:     req.YearID,
		PeriodCode: strings.TrimSpace(req.PeriodCode),
		ObjectID:   req.ObjectID,
		PointType:  strings.TrimSpace(req.PointType),
		Points:     req.Points,
		Reason:     strings.TrimSpace(req.Reason),
		Evidence:   strings.TrimSpace(req.Evidence),
		Approve:    boolOrDefault(req.Approve, false),
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleScoreError(c, err, "failed to create extra point")
		return
	}
	response.Success(c, result)
}

func (h *ScoreHandler) UpdateExtraPoint(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	extraPointID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid extra point id")
		return
	}
	var req updateExtraPointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid extra point payload")
		return
	}
	result, err := h.scoreService.UpdateExtraPoint(c.Request.Context(), operatorID, extraPointID, service.UpdateExtraPointInput{
		PointType: strings.TrimSpace(req.PointType),
		Points:    req.Points,
		Reason:    strings.TrimSpace(req.Reason),
		Evidence:  strings.TrimSpace(req.Evidence),
		Approve:   req.Approve,
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleScoreError(c, err, "failed to update extra point")
		return
	}
	response.Success(c, result)
}

func (h *ScoreHandler) ApproveExtraPoint(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	extraPointID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid extra point id")
		return
	}
	result, err := h.scoreService.ApproveExtraPoint(c.Request.Context(), operatorID, extraPointID, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleScoreError(c, err, "failed to approve extra point")
		return
	}
	response.Success(c, result)
}

func (h *ScoreHandler) DeleteExtraPoint(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	extraPointID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid extra point id")
		return
	}
	if err := h.scoreService.DeleteExtraPoint(c.Request.Context(), operatorID, extraPointID, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		h.handleScoreError(c, err, "failed to delete extra point")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *ScoreHandler) handleScoreError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrInvalidParam),
		errors.Is(err, service.ErrInvalidScoreValue),
		errors.Is(err, service.ErrInvalidScoreModule),
		errors.Is(err, service.ErrInvalidExtraPointType),
		errors.Is(err, service.ErrInvalidExtraPointValue),
		errors.Is(err, service.ErrExtraPointReasonEmpty):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
	case errors.Is(err, service.ErrDirectScoreExists),
		errors.Is(err, service.ErrPeriodLocked):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
	case errors.Is(err, service.ErrDirectScoreNotFound),
		errors.Is(err, service.ErrExtraPointNotFound),
		errors.Is(err, service.ErrAssessmentObjectNotFound),
		errors.Is(err, service.ErrPeriodNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, fallback)
	}
}
