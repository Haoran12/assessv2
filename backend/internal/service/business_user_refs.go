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

func resolveRequiredBusinessUserIDTx(tx *gorm.DB, userID uint) (uint, error) {
	_ = tx
	if userID == 0 {
		return 0, ErrInvalidParam
	}
	return userID, nil
}

func ensureBusinessUsersExistTx(tx *gorm.DB, userIDs []uint) error {
	_ = tx
	_ = userIDs
	return nil
}
