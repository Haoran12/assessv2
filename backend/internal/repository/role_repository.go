package repository

import (
	"context"
	"fmt"

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

func (r *RoleRepository) List(ctx context.Context) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.WithContext(ctx).
		Order("is_system DESC, id ASC").
		Find(&roles).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}
	return roles, nil
}

func (r *RoleRepository) ListByIDs(ctx context.Context, ids []uint) ([]model.Role, error) {
	if len(ids) == 0 {
		return []model.Role{}, nil
	}
	var roles []model.Role
	err := r.db.WithContext(ctx).
		Where("id IN ?", ids).
		Find(&roles).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list roles by ids: %w", err)
	}
	return roles, nil
}

func (r *RoleRepository) GetByID(ctx context.Context, roleID uint) (*model.Role, error) {
	var role model.Role
	err := r.db.WithContext(ctx).Where("id = ?", roleID).First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepository) GetByCode(ctx context.Context, code string) (*model.Role, error) {
	var role model.Role
	err := r.db.WithContext(ctx).Where("role_code = ?", code).First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepository) Create(ctx context.Context, role *model.Role) error {
	if err := r.db.WithContext(ctx).Create(role).Error; err != nil {
		return fmt.Errorf("failed to create role: %w", err)
	}
	return nil
}

func (r *RoleRepository) Save(ctx context.Context, role *model.Role) error {
	if err := r.db.WithContext(ctx).Save(role).Error; err != nil {
		return fmt.Errorf("failed to save role: %w", err)
	}
	return nil
}

func (r *RoleRepository) DeleteByID(ctx context.Context, roleID uint) error {
	result := r.db.WithContext(ctx).Delete(&model.Role{}, roleID)
	if result.Error != nil {
		return fmt.Errorf("failed to delete role: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
