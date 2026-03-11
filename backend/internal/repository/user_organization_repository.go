package repository

import (
	"context"

	"assessv2/backend/internal/model"
	"gorm.io/gorm"
)

type UserOrganizationRepository struct {
	db *gorm.DB
}

func NewUserOrganizationRepository(db *gorm.DB) *UserOrganizationRepository {
	return &UserOrganizationRepository{db: db}
}

func (r *UserOrganizationRepository) ListByUserID(ctx context.Context, userID uint) ([]model.UserOrganization, error) {
	var items []model.UserOrganization
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("is_primary DESC, id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}
