package repository

import (
	"context"
	"fmt"
	"log"

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
	if record.UserID != nil && *record.UserID > 0 {
		resolvedUserID, err := resolveAuditUserID(ctx, r.db, *record.UserID)
		if err != nil {
			log.Printf("audit create resolve user id failed user_id=%d: %v", *record.UserID, err)
			record.UserID = nil
		} else {
			record.UserID = resolvedUserID
		}
	}
	return r.db.WithContext(ctx).Create(&record).Error
}

func resolveAuditUserID(ctx context.Context, db *gorm.DB, userID uint) (*uint, error) {
	if db == nil || userID == 0 {
		return nil, nil
	}
	if !db.Migrator().HasTable("users") {
		return nil, nil
	}
	var count int64
	if err := db.WithContext(ctx).Table("users").Where("id = ?", userID).Count(&count).Error; err != nil {
		return nil, fmt.Errorf("count users by id failed: %w", err)
	}
	if count == 0 {
		return nil, nil
	}
	value := userID
	return &value, nil
}
