package handler

import (
	"net/http"
	"strconv"
	"strings"

	"assessv2/backend/internal/api/response"
	"assessv2/backend/internal/middleware"
	"assessv2/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type BackupHandler struct {
	backupService *service.BackupService
}

type createBackupRequest struct {
	Description string `json:"description"`
}

type restoreBackupRequest struct {
	ConfirmText string `json:"confirmText"`
}

type createOrgPackageRequest struct {
	RootOrganizationID     uint   `json:"rootOrganizationId"`
	Description            string `json:"description"`
	IncludeEmployeeHistory *bool  `json:"includeEmployeeHistory"`
}

type restoreOrgPackageRequest struct {
	ConfirmText              string `json:"confirmText"`
	Mode                     string `json:"mode"`
	TargetRootOrganizationID uint   `json:"targetRootOrganizationId"`
}

func NewBackupHandler(backupService *service.BackupService) *BackupHandler {
	return &BackupHandler{backupService: backupService}
}

func (h *BackupHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	result, err := h.backupService.List(c.Request.Context(), service.BackupListInput{
		Page:     page,
		PageSize: pageSize,
		Type:     strings.TrimSpace(c.Query("type")),
	})
	if err != nil {
		if err == service.ErrInvalidBackupType {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to query backups")
		return
	}
	response.Success(c, result)
}

func (h *BackupHandler) Create(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	var req createBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid backup payload")
		return
	}

	record, err := h.backupService.CreateManual(
		c.Request.Context(),
		claims.UserID,
		strings.TrimSpace(req.Description),
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		if err == service.ErrInvalidBackupType {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to create backup")
		return
	}
	response.Success(c, record)
}

func (h *BackupHandler) Download(c *gin.Context) {
	backupID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid backup id")
		return
	}

	filePath, fileName, err := h.backupService.ResolveDownloadPath(c.Request.Context(), backupID)
	if err != nil {
		if err == service.ErrBackupNotFound {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to resolve backup file")
		return
	}
	c.FileAttachment(filePath, fileName)
}

func (h *BackupHandler) Delete(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	backupID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid backup id")
		return
	}

	if err := h.backupService.Delete(c.Request.Context(), claims.UserID, backupID, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		if err == service.ErrBackupNotFound {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to delete backup")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *BackupHandler) Restore(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	backupID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid backup id")
		return
	}

	var req restoreBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid restore payload")
		return
	}

	err = h.backupService.Restore(
		c.Request.Context(),
		claims.UserID,
		backupID,
		strings.TrimSpace(req.ConfirmText),
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		switch err {
		case service.ErrBackupNotFound:
			response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
		case service.ErrBackupConfirmMismatch:
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to restore backup")
		}
		return
	}
	response.Success(c, gin.H{"restored": true})
}

func (h *BackupHandler) ListOrgPackages(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	var rootOrgID *uint
	if text := strings.TrimSpace(c.Query("rootOrganizationId")); text != "" {
		parsed, err := strconv.ParseUint(text, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid rootOrganizationId")
			return
		}
		value := uint(parsed)
		rootOrgID = &value
	}

	result, err := h.backupService.ListOrgPackages(c.Request.Context(), claims, service.OrgPackageListInput{
		Page:               page,
		PageSize:           pageSize,
		RootOrganizationID: rootOrgID,
	})
	if err != nil {
		if err == service.ErrForbidden {
			response.Error(c, http.StatusForbidden, response.CodeForbidden, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to query org packages")
		return
	}
	response.Success(c, result)
}

func (h *BackupHandler) CreateOrgPackage(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	var req createOrgPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid org package payload")
		return
	}

	includeHistory := true
	if req.IncludeEmployeeHistory != nil {
		includeHistory = *req.IncludeEmployeeHistory
	}

	record, err := h.backupService.CreateOrgPackage(
		c.Request.Context(),
		claims,
		claims.UserID,
		service.OrgPackageCreateInput{
			RootOrganizationID:     req.RootOrganizationID,
			Description:            strings.TrimSpace(req.Description),
			IncludeEmployeeHistory: includeHistory,
		},
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		switch err {
		case service.ErrInvalidParam, service.ErrOrganizationNotFound:
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
		case service.ErrForbidden:
			response.Error(c, http.StatusForbidden, response.CodeForbidden, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to create org package")
		}
		return
	}
	response.Success(c, record)
}

func (h *BackupHandler) DownloadOrgPackage(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	backupID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid backup id")
		return
	}

	filePath, fileName, err := h.backupService.ResolveOrgPackageDownloadPath(c.Request.Context(), claims, backupID)
	if err != nil {
		switch err {
		case service.ErrBackupNotFound:
			response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
		case service.ErrForbidden:
			response.Error(c, http.StatusForbidden, response.CodeForbidden, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to resolve org package file")
		}
		return
	}
	c.FileAttachment(filePath, fileName)
}

func (h *BackupHandler) RestoreOrgPackage(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	backupID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid backup id")
		return
	}

	var req restoreOrgPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid org restore payload")
		return
	}

	err = h.backupService.RestoreOrgPackage(
		c.Request.Context(),
		claims,
		claims.UserID,
		backupID,
		service.OrgPackageRestoreInput{
			ConfirmText:              strings.TrimSpace(req.ConfirmText),
			Mode:                     strings.TrimSpace(req.Mode),
			TargetRootOrganizationID: req.TargetRootOrganizationID,
		},
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		switch err {
		case service.ErrBackupNotFound:
			response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
		case service.ErrForbidden:
			response.Error(c, http.StatusForbidden, response.CodeForbidden, err.Error())
		case service.ErrBackupConfirmMismatch, service.ErrInvalidBackupRestoreMode, service.ErrBackupTargetMismatch, service.ErrBackupPackageBroken, service.ErrInvalidParam:
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to restore org package")
		}
		return
	}
	response.Success(c, gin.H{"restored": true})
}
