package service

import (
	"fmt"
	"strings"

	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

const (
	assessmentStatusPreparing = "preparing"
	assessmentStatusActive    = "active"
	assessmentStatusCompleted = "completed"
)

func normalizeYearStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ended":
		return assessmentStatusCompleted
	case assessmentStatusPreparing, assessmentStatusActive, assessmentStatusCompleted:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizePeriodStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "not_started":
		return assessmentStatusPreparing
	case "ended", "locked":
		return assessmentStatusCompleted
	case assessmentStatusPreparing, assessmentStatusActive, assessmentStatusCompleted:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func isEditableAssessmentStatus(status string) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	return normalized == assessmentStatusPreparing || normalized == assessmentStatusActive
}

func ensureAssessmentYearConfigWritableTx(tx *gorm.DB, yearID uint) error {
	year, err := loadAssessmentYearTx(tx, yearID)
	if err != nil {
		return err
	}
	if year.Status == assessmentStatusCompleted {
		return ErrAssessmentReadOnly
	}
	if !isEditableAssessmentStatus(year.Status) {
		return ErrInvalidYearStatus
	}
	return nil
}

func ensureAssessmentYearActiveTx(tx *gorm.DB, yearID uint) error {
	year, err := loadAssessmentYearTx(tx, yearID)
	if err != nil {
		return err
	}
	if year.Status == assessmentStatusCompleted {
		return ErrAssessmentReadOnly
	}
	if year.Status != assessmentStatusActive {
		return ErrAssessmentNotActive
	}
	return nil
}

func ensurePeriodConfigWritableTx(tx *gorm.DB, yearID uint, periodCode string) error {
	if err := ensureAssessmentYearConfigWritableTx(tx, yearID); err != nil {
		return err
	}
	period, err := loadAssessmentPeriodTx(tx, yearID, periodCode)
	if err != nil {
		return err
	}
	if period.Status == assessmentStatusCompleted {
		return ErrAssessmentReadOnly
	}
	if !isEditableAssessmentStatus(period.Status) {
		return ErrInvalidPeriodStatus
	}
	return nil
}

func ensurePeriodDataWritableTx(tx *gorm.DB, yearID uint, periodCode string) error {
	if err := ensureAssessmentYearActiveTx(tx, yearID); err != nil {
		return err
	}
	period, err := loadAssessmentPeriodTx(tx, yearID, periodCode)
	if err != nil {
		return err
	}
	if period.Status == assessmentStatusCompleted {
		return ErrAssessmentReadOnly
	}
	if period.Status != assessmentStatusActive {
		return ErrPeriodNotActive
	}
	return nil
}

func ensureLatestAssessmentConfigWritableTx(tx *gorm.DB) error {
	var year model.AssessmentYear
	if err := tx.Order("year DESC, id DESC").First(&year).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to query latest assessment year: %w", err)
	}
	year.Status = normalizeYearStatus(year.Status)
	if year.Status == assessmentStatusCompleted {
		return ErrAssessmentReadOnly
	}
	if !isEditableAssessmentStatus(year.Status) {
		return ErrInvalidYearStatus
	}
	return nil
}

func loadAssessmentYearTx(tx *gorm.DB, yearID uint) (*model.AssessmentYear, error) {
	var year model.AssessmentYear
	if err := tx.Where("id = ?", yearID).First(&year).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrYearNotFound
		}
		return nil, fmt.Errorf("failed to query assessment year: %w", err)
	}
	year.Status = normalizeYearStatus(year.Status)
	return &year, nil
}

func loadAssessmentPeriodTx(tx *gorm.DB, yearID uint, periodCode string) (*model.AssessmentPeriod, error) {
	var period model.AssessmentPeriod
	if err := tx.Where("year_id = ? AND period_code = ?", yearID, periodCode).First(&period).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrPeriodNotFound
		}
		return nil, fmt.Errorf("failed to query assessment period: %w", err)
	}
	period.Status = normalizePeriodStatus(period.Status)
	return &period, nil
}
