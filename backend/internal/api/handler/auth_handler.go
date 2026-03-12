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

type AuthHandler struct {
	authService *service.AuthService
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid login payload")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	if req.Username == "" || req.Password == "" {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "username and password are required")
		return
	}

	result, err := h.authService.Login(c.Request.Context(), req.Username, req.Password, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorizedInvalidCredential, err.Error())
		case errors.Is(err, service.ErrAccountInactive), errors.Is(err, service.ErrAccountLocked):
			response.Error(c, http.StatusForbidden, response.CodeForbidden, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to login")
		}
		return
	}
	response.Success(c, result)
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid password payload")
		return
	}
	req.OldPassword = strings.TrimSpace(req.OldPassword)
	req.NewPassword = strings.TrimSpace(req.NewPassword)
	if req.OldPassword == "" || req.NewPassword == "" {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "oldPassword and newPassword are required")
		return
	}
	if len(req.NewPassword) < 8 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "newPassword must be at least 8 characters")
		return
	}
	if req.NewPassword == req.OldPassword {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "newPassword must be different from oldPassword")
		return
	}

	if err := h.authService.ChangePassword(
		c.Request.Context(),
		claims.UserID,
		req.OldPassword,
		req.NewPassword,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidPassword):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, "oldPassword is incorrect")
		case errors.Is(err, service.ErrForbidden):
			response.Error(c, http.StatusForbidden, response.CodeForbidden, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to change password")
		}
		return
	}

	response.Success(c, gin.H{"changed": true})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}

	if err := h.authService.Logout(c.Request.Context(), claims.UserID, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to logout")
		return
	}
	response.Success(c, gin.H{"loggedOut": true})
}
