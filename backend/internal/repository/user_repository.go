package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"assessv2/backend/internal/model"
	"gorm.io/gorm"
)

type UserListFilter struct {
	Keyword string
	Status  string
	Offset  int
	Limit   int
}

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).
		Where("username = ? AND deleted_at IS NULL", username).
		Preload("UserRoles.Role").
		Preload("UserOrganizations").
		Preload("UserPermissionBindings").
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, userID uint) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", userID).
		Preload("UserRoles.Role").
		Preload("UserOrganizations").
		Preload("UserPermissionBindings").
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) List(ctx context.Context, filter UserListFilter) ([]model.User, int64, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	base := r.db.WithContext(ctx).Model(&model.User{}).Where("deleted_at IS NULL")
	if filter.Status != "" {
		base = base.Where("status = ?", filter.Status)
	}
	if filter.Keyword != "" {
		kw := strings.TrimSpace(filter.Keyword)
		base = base.Where("(username LIKE ? OR real_name LIKE ?)", "%"+kw+"%", "%"+kw+"%")
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	var users []model.User
	if err := base.
		Preload("UserRoles.Role").
		Preload("UserOrganizations").
		Preload("UserPermissionBindings").
		Order("id ASC").
		Offset(filter.Offset).
		Limit(filter.Limit).
		Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to query users: %w", err)
	}

	return users, total, nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID uint, passwordHash string, mustChange bool) error {
	updates := map[string]any{
		"password_hash":        passwordHash,
		"must_change_password": mustChange,
		"updated_at":           time.Now().Unix(),
	}
	result := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ? AND deleted_at IS NULL", userID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update password: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *UserRepository) UpdateStatus(ctx context.Context, userID uint, status string) error {
	result := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ? AND deleted_at IS NULL", userID).
		Updates(map[string]any{
			"status":     status,
			"updated_at": time.Now().Unix(),
		})
	if result.Error != nil {
		return fmt.Errorf("failed to update status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, userID uint, ipAddress string) error {
	now := time.Now().Unix()
	result := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ? AND deleted_at IS NULL", userID).
		Updates(map[string]any{
			"last_login_at": now,
			"last_login_ip": ipAddress,
			"updated_at":    now,
		})
	if result.Error != nil {
		return fmt.Errorf("failed to update last login: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *UserRepository) EnsureExists(ctx context.Context, userID uint) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ? AND deleted_at IS NULL", userID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func IsRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
