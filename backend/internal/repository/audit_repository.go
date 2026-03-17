package repository

import (
	"context"

	"assessv2/backend/internal/model"
	"gorm.io/gorm"
)

type AuditRepository struct {
	db *gorm.DB
}

func NewAuditRepository(db *gorm.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Create(ctx context.Context, record model.AuditLog) error {
	return r.db.WithContext(ctx).Create(&record).Error
}
