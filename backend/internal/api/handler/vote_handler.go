package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"assessv2/backend/internal/api/response"
	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/middleware"
	"assessv2/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type VoteHandler struct {
	voteService *service.VoteService
}

type generateVoteTasksRequest struct {
	YearID     uint   `json:"yearId"`
	PeriodCode string `json:"periodCode"`
	ModuleID   uint   `json:"moduleId"`
	ObjectIDs  []uint `json:"objectIds"`
}

type voteRecordRequest struct {
	GradeOption string `json:"gradeOption"`
	Remark      string `json:"remark"`
}

func NewVoteHandler(voteService *service.VoteService) *VoteHandler {
	return &VoteHandler{voteService: voteService}
}

func (h *VoteHandler) GenerateVoteTasks(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	var req generateVoteTasksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid vote task generate payload")
		return
	}
	result, err := h.voteService.GenerateVoteTasks(c.Request.Context(), operatorID, service.GenerateVoteTasksInput{
		YearID:     req.YearID,
		PeriodCode: strings.TrimSpace(req.PeriodCode),
		ModuleID:   req.ModuleID,
		ObjectIDs:  req.ObjectIDs,
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleVoteError(c, err, "failed to generate vote tasks")
		return
	}
	response.Success(c, result)
}

func (h *VoteHandler) ListVoteTasks(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	isRoot := auth.HasRole(claims.Roles, "root")

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
	voterID, err := parseOptionalUintQuery(c.Query("voterId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid voterId")
		return
	}

	if mine, parseErr := strconv.ParseBool(strings.TrimSpace(defaultQueryValue(c.Query("mine"), "false"))); parseErr == nil && mine {
		value := claims.UserID
		voterID = &value
	}
	if !isRoot && voterID != nil && *voterID != claims.UserID {
		response.Error(c, http.StatusForbidden, response.CodeForbidden, service.ErrVoteTaskForbidden.Error())
		return
	}

	items, err := h.voteService.ListVoteTasks(c.Request.Context(), service.ListVoteTaskFilter{
		YearID:     yearID,
		PeriodCode: strings.TrimSpace(c.Query("periodCode")),
		ModuleID:   moduleID,
		ObjectID:   objectID,
		VoterID:    voterID,
		Status:     strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		h.handleVoteError(c, err, "failed to query vote tasks")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *VoteHandler) SaveVoteDraft(c *gin.Context) {
	operatorID, isRoot, taskID, req, ok := h.parseVoteActionContext(c)
	if !ok {
		return
	}
	result, err := h.voteService.SaveVoteDraft(c.Request.Context(), operatorID, isRoot, taskID, service.VoteRecordInput{
		GradeOption: strings.TrimSpace(req.GradeOption),
		Remark:      strings.TrimSpace(req.Remark),
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleVoteError(c, err, "failed to save vote draft")
		return
	}
	response.Success(c, result)
}

func (h *VoteHandler) SubmitVote(c *gin.Context) {
	operatorID, isRoot, taskID, req, ok := h.parseVoteActionContext(c)
	if !ok {
		return
	}
	result, err := h.voteService.SubmitVote(c.Request.Context(), operatorID, isRoot, taskID, service.VoteRecordInput{
		GradeOption: strings.TrimSpace(req.GradeOption),
		Remark:      strings.TrimSpace(req.Remark),
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleVoteError(c, err, "failed to submit vote")
		return
	}
	response.Success(c, result)
}

func (h *VoteHandler) ResetVoteTask(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	taskID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid vote task id")
		return
	}
	result, err := h.voteService.ResetVoteTask(c.Request.Context(), operatorID, taskID, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleVoteError(c, err, "failed to reset vote task")
		return
	}
	response.Success(c, result)
}

func (h *VoteHandler) VoteStatistics(c *gin.Context) {
	yearID, err := parseOptionalUintQuery(c.Query("yearId"))
	if err != nil || yearID == nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid yearId")
		return
	}
	moduleID, err := parseOptionalUintQuery(c.Query("moduleId"))
	if err != nil || moduleID == nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid moduleId")
		return
	}
	objectID, err := parseOptionalUintQuery(c.Query("objectId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid objectId")
		return
	}
	result, err := h.voteService.ListVoteStatistics(c.Request.Context(), service.VoteStatisticsFilter{
		YearID:     *yearID,
		PeriodCode: strings.TrimSpace(c.Query("periodCode")),
		ModuleID:   *moduleID,
		ObjectID:   objectID,
	})
	if err != nil {
		h.handleVoteError(c, err, "failed to query vote statistics")
		return
	}
	response.Success(c, result)
}

func (h *VoteHandler) parseVoteActionContext(c *gin.Context) (uint, bool, uint, voteRecordRequest, bool) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return 0, false, 0, voteRecordRequest{}, false
	}
	taskID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid vote task id")
		return 0, false, 0, voteRecordRequest{}, false
	}
	var req voteRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid vote payload")
		return 0, false, 0, voteRecordRequest{}, false
	}
	isRoot := auth.HasRole(claims.Roles, "root")
	return claims.UserID, isRoot, taskID, req, true
}

func (h *VoteHandler) handleVoteError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrInvalidParam),
		errors.Is(err, service.ErrInvalidVoteModule),
		errors.Is(err, service.ErrInvalidVoteTaskStatus),
		errors.Is(err, service.ErrInvalidVoteGradeOption):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
	case errors.Is(err, service.ErrVoteTaskLocked),
		errors.Is(err, service.ErrVoteTaskNotResettable),
		errors.Is(err, service.ErrPeriodLocked):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
	case errors.Is(err, service.ErrVoteTaskForbidden):
		response.Error(c, http.StatusForbidden, response.CodeForbidden, err.Error())
	case errors.Is(err, service.ErrVoteTaskNotFound),
		errors.Is(err, service.ErrAssessmentObjectNotFound),
		errors.Is(err, service.ErrPeriodNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, fallback)
	}
}

func defaultQueryValue(value string, fallback string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return fallback
	}
	return text
}
