package repository

import (
	"context"
	"fmt"

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

func (r *UserRoleRepository) ExistsByRoleID(ctx context.Context, roleID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.UserRole{}).
		Where("role_id = ?", roleID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to count role usage: %w", err)
	}
	return count > 0, nil
}

func (r *UserRoleRepository) ReplaceForUser(
	ctx context.Context,
	userID uint,
	roleIDs []uint,
	primaryRoleID uint,
	createdBy *uint,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
			return fmt.Errorf("failed to clear user roles: %w", err)
		}
		for _, roleID := range roleIDs {
			item := model.UserRole{
				UserID:    userID,
				RoleID:    roleID,
				IsPrimary: roleID == primaryRoleID,
				CreatedBy: createdBy,
			}
			if err := tx.Create(&item).Error; err != nil {
				return fmt.Errorf("failed to create user role: %w", err)
			}
		}
		return nil
	})
}
