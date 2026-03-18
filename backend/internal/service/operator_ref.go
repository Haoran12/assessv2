package service

import "gorm.io/gorm"

func resolveBusinessWriteOperatorRef(db *gorm.DB, operatorID uint) *uint {
	return resolveBusinessWriteOperatorRefTx(db, operatorID)
}

func resolveBusinessWriteOperatorRefTx(tx *gorm.DB, operatorID uint) *uint {
	_ = tx
	if operatorID == 0 {
		return nil
	}
	value := operatorID
	return &value
}
