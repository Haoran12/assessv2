package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"assessv2/backend/internal/api/response"
	"assessv2/backend/internal/middleware"
	"assessv2/backend/internal/repository"
	"assessv2/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type SystemHandler struct {
	authService       *service.AuthService
	userService       *service.UserService
	objectLinkService *service.AssessmentObjectUserLinkService
}

type resetPasswordRequest struct {
	NewPassword string `json:"newPassword"`
}

type updateUserStatusRequest struct {
	Status string `json:"status"`
}

type upsertUserGroupRequest struct {
	RoleCode    string `json:"roleCode"`
	RoleName    string `json:"roleName"`
	Description string `json:"description"`
}

type updateUserGroupsRequest struct {
	RoleIDs       []uint `json:"roleIds"`
	PrimaryRoleID uint   `json:"primaryRoleId"`
}

type replaceUserObjectLinksRequest struct {
	Items []replaceUserObjectLinkItem `json:"items"`
}

type replaceUserObjectLinkItem struct {
	AssessmentObjectID uint   `json:"assessmentObjectId"`
	LinkType           string `json:"linkType"`
	AccessLevel        string `json:"accessLevel"`
	IsPrimary          bool   `json:"isPrimary"`
	EffectiveFrom      *int64 `json:"effectiveFrom"`
	EffectiveTo        *int64 `json:"effectiveTo"`
	IsActive           *bool  `json:"isActive"`
}

type upsertUserRequest struct {
	Username           string `json:"username"`
	RealName           string `json:"realName"`
	Password           string `json:"password"`
	Status             string `json:"status"`
	MustChangePassword *bool  `json:"mustChangePassword"`
	RoleIDs            []uint `json:"roleIds"`
	PrimaryRoleID      uint   `json:"primaryRoleId"`
}

func NewSystemHandler(
	authService *service.AuthService,
	userService *service.UserService,
	objectLinkService *service.AssessmentObjectUserLinkService,
) *SystemHandler {
	return &SystemHandler{
		authService:       authService,
		userService:       userService,
		objectLinkService: objectLinkService,
	}
}

func (h *SystemHandler) Profile(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	profile, mustChangePassword, err := h.authService.GetProfile(c.Request.Context(), claims.UserID)
	if err != nil {
		if repository.IsRecordNotFound(err) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "user not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to load profile")
		return
	}
	response.Success(c, gin.H{
		"user":               profile,
		"mustChangePassword": mustChangePassword,
	})
}

func (h *SystemHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	input := service.ListUsersInput{
		Page:     page,
		PageSize: pageSize,
		Keyword:  c.Query("keyword"),
		Status:   c.Query("status"),
	}

	result, err := h.userService.ListUsers(c.Request.Context(), input)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to query users")
		return
	}
	response.Success(c, result)
}

func (h *SystemHandler) CreateUser(c *gin.Context) {
	operatorClaims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	var req upsertUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid user payload")
		return
	}

	result, err := h.userService.CreateUser(
		c.Request.Context(),
		operatorClaims.UserID,
		service.CreateUserInput{
			Username:           strings.TrimSpace(req.Username),
			RealName:           strings.TrimSpace(req.RealName),
			Password:           strings.TrimSpace(req.Password),
			Status:             strings.TrimSpace(req.Status),
			MustChangePassword: req.MustChangePassword,
			RoleIDs:            req.RoleIDs,
			PrimaryRoleID:      req.PrimaryRoleID,
		},
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidUsername),
			errors.Is(err, service.ErrInvalidRealName),
			errors.Is(err, service.ErrInvalidUserStatus),
			errors.Is(err, service.ErrInvalidRoleList):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
		case errors.Is(err, service.ErrUsernameExists):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to create user")
		}
		return
	}
	response.Success(c, result)
}

func (h *SystemHandler) UpdateUser(c *gin.Context) {
	operatorClaims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	userID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid user id")
		return
	}

	var req upsertUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid user payload")
		return
	}

	result, err := h.userService.UpdateUser(
		c.Request.Context(),
		operatorClaims.UserID,
		userID,
		service.UpdateUserInput{
			Username:           strings.TrimSpace(req.Username),
			RealName:           strings.TrimSpace(req.RealName),
			Password:           strings.TrimSpace(req.Password),
			Status:             strings.TrimSpace(req.Status),
			MustChangePassword: req.MustChangePassword,
			RoleIDs:            req.RoleIDs,
			PrimaryRoleID:      req.PrimaryRoleID,
		},
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		switch {
		case repository.IsRecordNotFound(err):
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "user not found")
		case errors.Is(err, service.ErrInvalidUsername),
			errors.Is(err, service.ErrInvalidRealName),
			errors.Is(err, service.ErrInvalidUserStatus),
			errors.Is(err, service.ErrInvalidRoleList),
			errors.Is(err, service.ErrCannotDisableSelf),
			errors.Is(err, service.ErrCannotDemoteRoot),
			errors.Is(err, service.ErrCannotRenameRoot):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
		case errors.Is(err, service.ErrUsernameExists):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to update user")
		}
		return
	}
	response.Success(c, result)
}

func (h *SystemHandler) DeleteUser(c *gin.Context) {
	operatorClaims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	userID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid user id")
		return
	}

	if err := h.userService.DeleteUser(
		c.Request.Context(),
		operatorClaims.UserID,
		userID,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	); err != nil {
		switch {
		case repository.IsRecordNotFound(err):
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "user not found")
		case errors.Is(err, service.ErrCannotDeleteRoot), errors.Is(err, service.ErrCannotDeleteSelf):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to delete user")
		}
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *SystemHandler) ResetPassword(c *gin.Context) {
	operatorClaims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	userID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid user id")
		return
	}

	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid reset password payload")
		return
	}

	if err := h.userService.ResetPassword(
		c.Request.Context(),
		operatorClaims.UserID,
		userID,
		strings.TrimSpace(req.NewPassword),
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	); err != nil {
		if repository.IsRecordNotFound(err) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "user not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to reset password")
		return
	}
	response.Success(c, gin.H{"reset": true})
}

func (h *SystemHandler) UpdateUserStatus(c *gin.Context) {
	operatorClaims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	userID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid user id")
		return
	}

	var req updateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid status payload")
		return
	}

	status := strings.TrimSpace(req.Status)
	if err := h.userService.UpdateStatus(
		c.Request.Context(),
		operatorClaims.UserID,
		userID,
		status,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidUserStatus):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
		case errors.Is(err, service.ErrCannotDisableSelf):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
		case repository.IsRecordNotFound(err):
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "user not found")
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to update user status")
		}
		return
	}

	response.Success(c, gin.H{"updated": true})
}

func (h *SystemHandler) ListUserGroups(c *gin.Context) {
	result, err := h.userService.ListUserGroups(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to query user groups")
		return
	}
	response.Success(c, gin.H{"items": result})
}

func (h *SystemHandler) CreateUserGroup(c *gin.Context) {
	operatorClaims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	var req upsertUserGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid user group payload")
		return
	}

	result, err := h.userService.CreateUserGroup(
		c.Request.Context(),
		operatorClaims.UserID,
		service.CreateUserGroupInput{
			RoleCode:    strings.TrimSpace(req.RoleCode),
			RoleName:    strings.TrimSpace(req.RoleName),
			Description: strings.TrimSpace(req.Description),
		},
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRoleCode), errors.Is(err, service.ErrInvalidRoleName), errors.Is(err, service.ErrRoleCodeExists):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to create user group")
		}
		return
	}
	response.Success(c, result)
}

func (h *SystemHandler) UpdateUserGroup(c *gin.Context) {
	operatorClaims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	roleID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid group id")
		return
	}

	var req upsertUserGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid user group payload")
		return
	}

	result, err := h.userService.UpdateUserGroup(
		c.Request.Context(),
		operatorClaims.UserID,
		roleID,
		service.UpdateUserGroupInput{
			RoleCode:    strings.TrimSpace(req.RoleCode),
			RoleName:    strings.TrimSpace(req.RoleName),
			Description: strings.TrimSpace(req.Description),
		},
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRoleNotFound):
			response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
		case errors.Is(err, service.ErrSystemRoleLocked):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
		case errors.Is(err, service.ErrInvalidRoleCode), errors.Is(err, service.ErrInvalidRoleName), errors.Is(err, service.ErrRoleCodeExists):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to update user group")
		}
		return
	}
	response.Success(c, result)
}

func (h *SystemHandler) DeleteUserGroup(c *gin.Context) {
	operatorClaims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	roleID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid group id")
		return
	}

	if err := h.userService.DeleteUserGroup(
		c.Request.Context(),
		operatorClaims.UserID,
		roleID,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	); err != nil {
		switch {
		case errors.Is(err, service.ErrRoleNotFound):
			response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
		case errors.Is(err, service.ErrSystemRoleLocked), errors.Is(err, service.ErrRoleInUse):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to delete user group")
		}
		return
	}

	response.Success(c, gin.H{"deleted": true})
}

func (h *SystemHandler) UpdateUserGroups(c *gin.Context) {
	operatorClaims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	userID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid user id")
		return
	}

	var req updateUserGroupsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid user groups payload")
		return
	}

	if err := h.userService.UpdateUserGroups(
		c.Request.Context(),
		operatorClaims.UserID,
		userID,
		service.UpdateUserGroupsInput{
			RoleIDs:       req.RoleIDs,
			PrimaryRoleID: req.PrimaryRoleID,
		},
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRoleList), errors.Is(err, service.ErrCannotDemoteRoot):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
		case repository.IsRecordNotFound(err):
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "user not found")
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to update user groups")
		}
		return
	}

	response.Success(c, gin.H{"updated": true})
}

func (h *SystemHandler) ListUserObjectLinks(c *gin.Context) {
	userID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid user id")
		return
	}
	if err := h.userService.EnsureUserExists(c.Request.Context(), userID); err != nil {
		if repository.IsRecordNotFound(err) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "user not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to verify user")
		return
	}

	yearID, err := parseOptionalUintQuery(c.Query("yearId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid yearId")
		return
	}

	items, err := h.objectLinkService.ListUserLinks(c.Request.Context(), userID, yearID)
	if err != nil {
		if errors.Is(err, service.ErrInvalidParam) {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to query user object links")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *SystemHandler) ReplaceUserObjectLinks(c *gin.Context) {
	operatorClaims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	userID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid user id")
		return
	}
	if err := h.userService.EnsureUserExists(c.Request.Context(), userID); err != nil {
		if repository.IsRecordNotFound(err) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "user not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to verify user")
		return
	}

	var req replaceUserObjectLinksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid object links payload")
		return
	}

	inputItems := make([]service.AssessmentObjectUserLinkUpsertItem, 0, len(req.Items))
	for _, item := range req.Items {
		inputItems = append(inputItems, service.AssessmentObjectUserLinkUpsertItem{
			AssessmentObjectID: item.AssessmentObjectID,
			LinkType:           strings.TrimSpace(item.LinkType),
			AccessLevel:        strings.TrimSpace(item.AccessLevel),
			IsPrimary:          item.IsPrimary,
			EffectiveFrom:      item.EffectiveFrom,
			EffectiveTo:        item.EffectiveTo,
			IsActive:           item.IsActive,
		})
	}

	items, err := h.objectLinkService.ReplaceUserLinks(
		c.Request.Context(),
		operatorClaims.UserID,
		userID,
		service.ReplaceAssessmentObjectUserLinksInput{Items: inputItems},
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidParam):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
		case errors.Is(err, service.ErrAssessmentObjectNotFound):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to replace user object links")
		}
		return
	}
	response.Success(c, gin.H{"items": items})
}

func parseUserIDParam(c *gin.Context) (uint, error) {
	value := c.Param("id")
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(parsed), nil
}
