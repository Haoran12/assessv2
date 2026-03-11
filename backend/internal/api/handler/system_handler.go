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

func NewSystemHandler(authService *service.AuthService, userService *service.UserService) *SystemHandler {
	return &SystemHandler{
		authService: authService,
		userService: userService,
	}
}

func (h *SystemHandler) Profile(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, 40100, "missing auth context")
		return
	}

	profile, mustChangePassword, err := h.authService.GetProfile(c.Request.Context(), claims.UserID)
	if err != nil {
		if repository.IsRecordNotFound(err) {
			response.Error(c, http.StatusNotFound, 40401, "user not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, 50001, "failed to load profile")
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
		response.Error(c, http.StatusInternalServerError, 50001, "failed to query users")
		return
	}
	response.Success(c, result)
}

func (h *SystemHandler) ResetPassword(c *gin.Context) {
	operatorClaims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, 40100, "missing auth context")
		return
	}

	userID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40002, "invalid user id")
		return
	}

	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		response.Error(c, http.StatusBadRequest, 40001, "invalid reset password payload")
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
			response.Error(c, http.StatusNotFound, 40401, "user not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, 50001, "failed to reset password")
		return
	}
	response.Success(c, gin.H{"reset": true})
}

func (h *SystemHandler) UpdateUserStatus(c *gin.Context) {
	operatorClaims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, 40100, "missing auth context")
		return
	}

	userID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40002, "invalid user id")
		return
	}

	var req updateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "invalid status payload")
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
			response.Error(c, http.StatusBadRequest, 40002, err.Error())
		case errors.Is(err, service.ErrCannotDisableSelf):
			response.Error(c, http.StatusBadRequest, 40003, err.Error())
		case repository.IsRecordNotFound(err):
			response.Error(c, http.StatusNotFound, 40401, "user not found")
		default:
			response.Error(c, http.StatusInternalServerError, 50001, "failed to update user status")
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
