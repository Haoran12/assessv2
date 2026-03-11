package repository

import (
	"context"

	"assessv2/backend/internal/model"
	"gorm.io/gorm"
)

type RoleRepository struct {
	db *gorm.DB
}

func NewRoleRepository(db *gorm.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

func (r *RoleRepository) ListByUserID(ctx context.Context, userID uint) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.WithContext(ctx).
		Model(&model.Role{}).
		Joins("JOIN user_roles ur ON ur.role_id = roles.id").
		Where("ur.user_id = ?", userID).
		Order("ur.is_primary DESC, roles.id ASC").
		Find(&roles).Error
	if err != nil {
		return nil, err
	}
	return roles, nil
}
