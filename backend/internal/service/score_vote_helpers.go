package service

import (
	"regexp"
	"strings"

	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

var (
	periodCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,19}$`)
	voteTaskStatusSet = map[string]struct{}{
		"pending": {}, "completed": {}, "expired": {},
	}
	voteGradeOptionSet = map[string]struct{}{
		"excellent": {}, "good": {}, "average": {}, "poor": {},
	}
)

func normalizePeriodCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func isValidPeriodCode(value string) bool {
	return periodCodePattern.MatchString(strings.TrimSpace(value))
}

func ensurePeriodWritableTx(tx *gorm.DB, yearID uint, periodCode string) error {
	return ensurePeriodDataWritableTx(tx, yearID, periodCode)
}

func ensureAssessmentObjectTx(tx *gorm.DB, objectID, yearID uint) (*model.AssessmentObject, error) {
	var object model.AssessmentObject
	if err := tx.Where("id = ? AND year_id = ? AND is_active = 1", objectID, yearID).First(&object).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrAssessmentObjectNotFound
		}
		return nil, err
	}
	return &object, nil
}

func loadModuleByPeriodTx(tx *gorm.DB, moduleID uint, moduleCode string, yearID uint, periodCode string) (*model.ScoreModule, error) {
	var module model.ScoreModule
	err := tx.Table("score_modules AS sm").
		Select("sm.*").
		Joins("JOIN assessment_rules ar ON ar.id = sm.rule_id").
		Where(
			"sm.id = ? AND sm.module_code = ? AND sm.is_active = 1 AND ar.year_id = ? AND ar.period_code = ? AND ar.is_active = 1",
			moduleID,
			moduleCode,
			yearID,
			periodCode,
		).
		First(&module).Error
	if err != nil {
		return nil, err
	}
	return &module, nil
}
