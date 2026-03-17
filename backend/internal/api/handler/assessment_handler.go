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
	assessmentService *service.AssessmentService
}

func NewAssessmentHandler(assessmentService *service.AssessmentService) *AssessmentHandler {
	return &AssessmentHandler{assessmentService: assessmentService}
}

type createAssessmentYearRequest struct {
	Year           int    `json:"year"`
	YearName       string `json:"yearName"`
	Description    string `json:"description"`
	StartDate      string `json:"startDate"`
	EndDate        string `json:"endDate"`
	CopyFromYearID *uint  `json:"copyFromYearId"`
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

type assessmentPeriodTemplateItemRequest struct {
	PeriodCode string `json:"periodCode"`
	PeriodName string `json:"periodName"`
	StartDay   string `json:"startDay"`
	EndDay     string `json:"endDay"`
	SortOrder  int    `json:"sortOrder"`
}

type updatePeriodTemplatesRequest struct {
	Items []assessmentPeriodTemplateItemRequest `json:"items"`
}

func (h *AssessmentHandler) ListYears(c *gin.Context) {
	result, err := h.assessmentService.ListYears(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to query assessment years")
		return
	}
	response.Success(c, gin.H{"items": result})
}

func (h *AssessmentHandler) CreateYear(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	var req createAssessmentYearRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid assessment year payload")
		return
	}
	startDate, err := parseDateOrNil(req.StartDate)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid startDate")
		return
	}
	endDate, err := parseDateOrNil(req.EndDate)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid endDate")
		return
	}
	result, err := h.assessmentService.CreateYear(c.Request.Context(), claims, operatorID, service.CreateAssessmentYearInput{Year: req.Year, YearName: strings.TrimSpace(req.YearName), Description: strings.TrimSpace(req.Description), StartDate: startDate, EndDate: endDate, CopyFromYearID: req.CopyFromYearID}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleAssessmentError(c, err, "failed to create assessment year")
		return
	}
	response.Success(c, result)
}

func (h *AssessmentHandler) UpdateYearStatus(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	yearID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid year id")
		return
	}
	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid year status payload")
		return
	}
	result, err := h.assessmentService.UpdateYearStatus(c.Request.Context(), claims, operatorID, yearID, strings.TrimSpace(req.Status), c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleAssessmentError(c, err, "failed to update year status")
		return
	}
	response.Success(c, result)
}

func (h *AssessmentHandler) ListPeriods(c *gin.Context) {
	yearID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid year id")
		return
	}
	result, err := h.assessmentService.ListPeriods(c.Request.Context(), yearID)
	if err != nil {
		h.handleAssessmentError(c, err, "failed to query periods")
		return
	}
	response.Success(c, gin.H{"items": result})
}

func (h *AssessmentHandler) ListPeriodTemplates(c *gin.Context) {
	result, err := h.assessmentService.ListPeriodTemplates(c.Request.Context())
	if err != nil {
		h.handleAssessmentError(c, err, "failed to query period templates")
		return
	}
	response.Success(c, gin.H{"items": result})
}

func (h *AssessmentHandler) UpdatePeriodTemplates(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	var req updatePeriodTemplatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid period templates payload")
		return
	}

	items := make([]service.AssessmentPeriodTemplateItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, service.AssessmentPeriodTemplateItem{
			PeriodCode: strings.TrimSpace(item.PeriodCode),
			PeriodName: strings.TrimSpace(item.PeriodName),
			StartDay:   strings.TrimSpace(item.StartDay),
			EndDay:     strings.TrimSpace(item.EndDay),
			SortOrder:  item.SortOrder,
		})
	}

	result, err := h.assessmentService.UpdatePeriodTemplates(
		c.Request.Context(),
		claims,
		operatorID,
		items,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		h.handleAssessmentError(c, err, "failed to update period templates")
		return
	}
	response.Success(c, gin.H{"items": result})
}

func (h *AssessmentHandler) UpdatePeriodStatus(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	periodID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid period id")
		return
	}
	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid period status payload")
		return
	}
	result, err := h.assessmentService.UpdatePeriodStatus(c.Request.Context(), claims, operatorID, periodID, strings.TrimSpace(req.Status), c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleAssessmentError(c, err, "failed to update period status")
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
	yearID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid year id")
		return
	}
	result, err := h.assessmentService.ListObjects(c.Request.Context(), claims, yearID)
	if err != nil {
		h.handleAssessmentError(c, err, "failed to query assessment objects")
		return
	}
	response.Success(c, gin.H{"items": result})
}

func (h *AssessmentHandler) handleAssessmentError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrInvalidParam),
		errors.Is(err, service.ErrInvalidYearStatus),
		errors.Is(err, service.ErrInvalidYearTransition),
		errors.Is(err, service.ErrInvalidPeriodStatus),
		errors.Is(err, service.ErrInvalidPeriodTransition),
		errors.Is(err, service.ErrInvalidPeriodTemplate):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
	case errors.Is(err, service.ErrYearAlreadyExists),
		errors.Is(err, service.ErrAssessmentReadOnly),
		errors.Is(err, service.ErrAssessmentNotActive),
		errors.Is(err, service.ErrPeriodNotActive):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
	case errors.Is(err, service.ErrForbidden):
		response.Error(c, http.StatusForbidden, response.CodeForbidden, err.Error())
	case errors.Is(err, service.ErrYearNotFound),
		errors.Is(err, service.ErrPeriodNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, fallback)
	}
}
