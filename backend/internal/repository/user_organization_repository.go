package repository

import (
	"context"
	"fmt"

	"assessv2/backend/internal/model"
	"gorm.io/gorm"
)

type UserOrganizationRepository struct {
	db *gorm.DB
}

func NewUserOrganizationRepository(db *gorm.DB) *UserOrganizationRepository {
	return &UserOrganizationRepository{db: db}
}

type UserOrganizationAssignment struct {
	OrganizationType string
	OrganizationID   uint
	RoleInOrg        string
	IsPrimary        bool
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

func (r *UserOrganizationRepository) ReplaceForUser(
	ctx context.Context,
	userID uint,
	assignments []UserOrganizationAssignment,
	createdBy *uint,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return r.ReplaceForUserWithTx(tx, userID, assignments, createdBy)
	})
}

func (r *UserOrganizationRepository) ReplaceForUserWithTx(
	tx *gorm.DB,
	userID uint,
	assignments []UserOrganizationAssignment,
	createdBy *uint,
) error {
	if err := tx.Where("user_id = ?", userID).Delete(&model.UserOrganization{}).Error; err != nil {
		return fmt.Errorf("failed to clear user organizations: %w", err)
	}
	for _, assignment := range assignments {
		record := model.UserOrganization{
			UserID:           userID,
			OrganizationType: assignment.OrganizationType,
			OrganizationID:   assignment.OrganizationID,
			RoleInOrg:        assignment.RoleInOrg,
			IsPrimary:        assignment.IsPrimary,
			CreatedBy:        createdBy,
		}
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("failed to create user organization: %w", err)
		}
	}
	return nil
}

func (r *UserOrganizationRepository) DeleteByUserIDWithTx(tx *gorm.DB, userID uint) error {
	if err := tx.Where("user_id = ?", userID).Delete(&model.UserOrganization{}).Error; err != nil {
		return fmt.Errorf("failed to delete user organizations: %w", err)
	}
	return nil
}
