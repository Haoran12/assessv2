package handler

import (
	"errors"
	"time"

	"assessv2/backend/internal/api/response"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type AuthHandler struct {
	jwtSecret string
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewAuthHandler(jwtSecret string) *AuthHandler {
	return &AuthHandler{
		jwtSecret: jwtSecret,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, 40001, "invalid login payload")
		return
	}

	if err := h.validateLogin(req); err != nil {
		response.Error(c, 401, 40101, err.Error())
		return
	}

	token, err := h.signToken(req.Username)
	if err != nil {
		response.Error(c, 500, 50001, "failed to sign token")
		return
	}

	response.Success(c, gin.H{
		"token":     token,
		"tokenType": "Bearer",
		"expiresIn": 86400,
		"user": gin.H{
			"username": req.Username,
			"role":     "root",
		},
	})
}

func (h *AuthHandler) validateLogin(req loginRequest) error {
	// Bootstrapping stage credential. Replace with DB-based user validation next.
	if req.Username == "root" && req.Password == "#2026@hdwl" {
		return nil
	}
	return errors.New("invalid username or password")
}

func (h *AuthHandler) signToken(username string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  username,
		"role": "root",
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
		"iat":  time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}
