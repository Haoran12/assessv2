package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"assessv2/backend/internal/api/response"
	"assessv2/backend/internal/middleware"
	"assessv2/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type ImportExportHandler struct {
	importExportService *service.ImportExportService
}

type confirmDirectImportRequest struct {
	YearID     uint                                  `json:"yearId"`
	PeriodCode string                                `json:"periodCode"`
	ModuleID   uint                                  `json:"moduleId"`
	Overwrite  bool                                  `json:"overwrite"`
	Rows       []service.DirectScoreImportConfirmRow `json:"rows"`
}

func NewImportExportHandler(importExportService *service.ImportExportService) *ImportExportHandler {
	return &ImportExportHandler{
		importExportService: importExportService,
	}
}

func (h *ImportExportHandler) DownloadTemplate(c *gin.Context) {
	fileName, content, err := h.importExportService.GenerateTemplate(c.Param("type"))
	if err != nil {
		h.handleImportExportError(c, err, "failed to generate template")
		return
	}
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}

func (h *ImportExportHandler) PreviewDirectScoreImport(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	yearID, err := parseUintField(c.PostForm("yearId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid yearId")
		return
	}
	moduleID, err := parseUintField(c.PostForm("moduleId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid moduleId")
		return
	}
	overwrite := false
	if value := strings.TrimSpace(c.PostForm("overwrite")); value != "" {
		parsed, parseErr := strconv.ParseBool(value)
		if parseErr != nil {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid overwrite")
			return
		}
		overwrite = parsed
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "import file is required")
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "failed to read import file")
		return
	}
	defer func() {
		_ = file.Close()
	}()
	fileContent, err := io.ReadAll(file)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "failed to read import file")
		return
	}

	result, err := h.importExportService.PreviewDirectScoreImport(c.Request.Context(), claims, service.DirectScoreImportPreviewInput{
		YearID:      yearID,
		PeriodCode:  strings.TrimSpace(c.PostForm("periodCode")),
		ModuleID:    moduleID,
		Overwrite:   overwrite,
		FileName:    fileHeader.Filename,
		FileContent: fileContent,
	})
	if err != nil {
		h.handleImportExportError(c, err, "failed to preview import file")
		return
	}
	response.Success(c, result)
}

func (h *ImportExportHandler) ConfirmDirectScoreImport(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	operatorID := claims.UserID

	var req confirmDirectImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid import confirm payload")
		return
	}

	result, err := h.importExportService.ConfirmDirectScoreImport(c.Request.Context(), claims, operatorID, service.DirectScoreImportConfirmInput{
		YearID:     req.YearID,
		PeriodCode: strings.TrimSpace(req.PeriodCode),
		ModuleID:   req.ModuleID,
		Overwrite:  req.Overwrite,
		Rows:       req.Rows,
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleImportExportError(c, err, "failed to confirm import")
		return
	}
	response.Success(c, result)
}

func (h *ImportExportHandler) ExportWorkbook(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	operatorID := claims.UserID

	yearID, err := parseUintField(c.Query("yearId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid yearId")
		return
	}
	fileName, content, err := h.importExportService.ExportWorkbook(c.Request.Context(), claims, operatorID, service.ExportWorkbookInput{
		YearID:         yearID,
		PeriodCode:     strings.TrimSpace(c.Query("periodCode")),
		ObjectCategory: strings.TrimSpace(c.Query("objectCategory")),
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleImportExportError(c, err, "failed to export workbook")
		return
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}

func (h *ImportExportHandler) handleImportExportError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrInvalidParam),
		errors.Is(err, service.ErrInvalidImportTemplateType),
		errors.Is(err, service.ErrImportFileRequired),
		errors.Is(err, service.ErrImportFileInvalid),
		errors.Is(err, service.ErrImportNoValidRows),
		errors.Is(err, service.ErrImportRowLimitExceeded),
		errors.Is(err, service.ErrInvalidScoreModule):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
	case errors.Is(err, service.ErrForbidden):
		response.Error(c, http.StatusForbidden, response.CodeForbidden, err.Error())
	case errors.Is(err, service.ErrAssessmentObjectNotFound),
		errors.Is(err, service.ErrPeriodNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, fallback)
	}
}

func parseUintField(raw string) (uint, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, fmt.Errorf("empty")
	}
	parsed, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(parsed), nil
}
