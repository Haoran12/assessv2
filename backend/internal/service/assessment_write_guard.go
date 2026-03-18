package service

import "gorm.io/gorm"

func ensureLatestAssessmentConfigWritableTx(tx *gorm.DB) error {
	_ = tx
	return nil
}
