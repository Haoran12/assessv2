package service

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

const businessShadowUserNamePrefix = "shadow_user_"

func resolveBusinessWriteOperatorRef(db *gorm.DB, operatorID uint) *uint {
	return resolveBusinessWriteOperatorRefTx(db, operatorID)
}

func resolveBusinessWriteOperatorRefTx(tx *gorm.DB, operatorID uint) *uint {
	if operatorID == 0 || tx == nil {
		return nil
	}
	if !tx.Migrator().HasTable("users") {
		return nil
	}
	var count int64
	if err := tx.Table("users").Where("id = ?", operatorID).Count(&count).Error; err != nil {
		return nil
	}
	if count == 0 {
		return nil
	}
	value := operatorID
	return &value
}

func resolveRequiredBusinessUserIDTx(tx *gorm.DB, userID uint) (uint, error) {
	if userID == 0 {
		return 0, ErrInvalidParam
	}
	if tx == nil || !tx.Migrator().HasTable("users") {
		return userID, nil
	}

	if err := ensureBusinessUserRowTx(tx, userID); err != nil {
		return 0, err
	}
	return userID, nil
}

func ensureBusinessUsersExistTx(tx *gorm.DB, userIDs []uint) error {
	if tx == nil || len(userIDs) == 0 || !tx.Migrator().HasTable("users") {
		return nil
	}
	seen := make(map[uint]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID == 0 {
			continue
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		if err := ensureBusinessUserRowTx(tx, userID); err != nil {
			return err
		}
	}
	return nil
}

func ensureBusinessUserRowTx(tx *gorm.DB, userID uint) error {
	if userID == 0 || tx == nil || !tx.Migrator().HasTable("users") {
		return nil
	}

	var count int64
	if err := tx.Table("users").Where("id = ?", userID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to verify business user row id=%d: %w", userID, err)
	}
	if count > 0 {
		return nil
	}

	now := time.Now().Unix()
	payload := map[string]any{
		"id":       userID,
		"username": fmt.Sprintf("%s%d", businessShadowUserNamePrefix, userID),
	}
	if tx.Migrator().HasColumn("users", "password_hash") {
		payload["password_hash"] = fmt.Sprintf("shadow:%d", userID)
	}
	if tx.Migrator().HasColumn("users", "real_name") {
		payload["real_name"] = fmt.Sprintf("Shadow User %d", userID)
	}
	if tx.Migrator().HasColumn("users", "status") {
		payload["status"] = "active"
	}
	if tx.Migrator().HasColumn("users", "must_change_password") {
		payload["must_change_password"] = true
	}
	if tx.Migrator().HasColumn("users", "created_at") {
		payload["created_at"] = now
	}
	if tx.Migrator().HasColumn("users", "updated_at") {
		payload["updated_at"] = now
	}

	if err := tx.Table("users").Create(payload).Error; err != nil {
		if isUniqueConstraintError(err) {
			var retryCount int64
			if retryErr := tx.Table("users").Where("id = ?", userID).Count(&retryCount).Error; retryErr == nil && retryCount > 0 {
				return nil
			}
		}
		return fmt.Errorf("failed to ensure business user row id=%d: %w", userID, err)
	}
	return nil
}
