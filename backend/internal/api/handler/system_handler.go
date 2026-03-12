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
	authService *service.AuthService
	userService *service.UserService
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

func NewSystemHandler(authService *service.AuthService, userService *service.UserService) *SystemHandler {
	return &SystemHandler{
		authService: authService,
		userService: userService,
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

func parseUserIDParam(c *gin.Context) (uint, error) {
	value := c.Param("id")
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(parsed), nil
}
