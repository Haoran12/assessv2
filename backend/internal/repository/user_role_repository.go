package repository

import (
	"context"

	"assessv2/backend/internal/model"
	"gorm.io/gorm"
)

type UserRoleRepository struct {
	db *gorm.DB
}

func NewUserRoleRepository(db *gorm.DB) *UserRoleRepository {
	return &UserRoleRepository{db: db}
}

func (r *UserRoleRepository) ListByUserID(ctx context.Context, userID uint) ([]model.UserRole, error) {
	var items []model.UserRole
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Preload("Role").
		Order("is_primary DESC, id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}
