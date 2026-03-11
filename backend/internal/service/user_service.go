package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	userRepo        *repository.UserRepository
	auditRepo       *repository.AuditRepository
	defaultPassword string
}

type ListUsersInput struct {
	Page     int
	PageSize int
	Keyword  string
	Status   string
}

type UserListItem struct {
	ID                 uint                     `json:"id"`
	Username           string                   `json:"username"`
	RealName           string                   `json:"realName"`
	Status             string                   `json:"status"`
	MustChangePassword bool                     `json:"mustChangePassword"`
	LastLoginAt        *int64                   `json:"lastLoginAt,omitempty"`
	LastLoginIP        *string                  `json:"lastLoginIp,omitempty"`
	Roles              []string                 `json:"roles"`
	PrimaryRole        string                   `json:"primaryRole"`
	Organizations      []auth.OrganizationScope `json:"organizations"`
	CreatedAt          int64                    `json:"createdAt"`
	UpdatedAt          int64                    `json:"updatedAt"`
}

type ListUsersOutput struct {
	Items    []UserListItem `json:"items"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
}

func NewUserService(
	userRepo *repository.UserRepository,
	auditRepo *repository.AuditRepository,
	defaultPassword string,
) *UserService {
	return &UserService{
		userRepo:        userRepo,
		auditRepo:       auditRepo,
		defaultPassword: defaultPassword,
	}
}

func (s *UserService) ListUsers(ctx context.Context, input ListUsersInput) (*ListUsersOutput, error) {
	page := input.Page
	if page <= 0 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	users, total, err := s.userRepo.List(ctx, repository.UserListFilter{
		Keyword: strings.TrimSpace(input.Keyword),
		Status:  strings.TrimSpace(input.Status),
		Offset:  (page - 1) * pageSize,
		Limit:   pageSize,
	})
	if err != nil {
		return nil, err
	}

	items := make([]UserListItem, 0, len(users))
	for _, user := range users {
		primaryRole, roleCodes, _, orgScopes, identityErr := extractIdentity(&user)
		if identityErr != nil {
			return nil, identityErr
		}
		items = append(items, UserListItem{
			ID:                 user.ID,
			Username:           user.Username,
			RealName:           user.RealName,
			Status:             user.Status,
			MustChangePassword: user.MustChangePassword,
			LastLoginAt:        user.LastLoginAt,
			LastLoginIP:        user.LastLoginIP,
			Roles:              roleCodes,
			PrimaryRole:        primaryRole,
			Organizations:      orgScopes,
			CreatedAt:          user.CreatedAt,
			UpdatedAt:          user.UpdatedAt,
		})
	}

	return &ListUsersOutput{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *UserService) ResetPassword(ctx context.Context, operatorID, targetUserID uint, newPassword, ipAddress, userAgent string) error {
	if strings.TrimSpace(newPassword) == "" {
		newPassword = s.defaultPassword
	}
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	if err := s.userRepo.UpdatePassword(ctx, targetUserID, string(hashBytes), true); err != nil {
		return err
	}

	targetID := targetUserID
	operator := operatorID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "users", &targetID, map[string]any{
		"event":  "reset_password",
		"target": strconv.FormatUint(uint64(targetUserID), 10),
	}, ipAddress, userAgent))
	return nil
}

func (s *UserService) UpdateStatus(ctx context.Context, operatorID, targetUserID uint, status string, ipAddress, userAgent string) error {
	switch status {
	case "active", "inactive", "locked":
	default:
		return ErrInvalidUserStatus
	}
	if operatorID == targetUserID && status != "active" {
		return ErrCannotDisableSelf
	}

	if err := s.userRepo.UpdateStatus(ctx, targetUserID, status); err != nil {
		return err
	}

	targetID := targetUserID
	operator := operatorID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "users", &targetID, map[string]any{
		"event":  "update_status",
		"status": status,
	}, ipAddress, userAgent))
	return nil
}
