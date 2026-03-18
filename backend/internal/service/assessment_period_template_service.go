package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

const assessmentPeriodTemplatesSettingKey = "assessment.period_templates"

type AssessmentPeriodTemplateItem struct {
	PeriodCode string `json:"periodCode"`
	PeriodName string `json:"periodName"`
	SortOrder  int    `json:"sortOrder"`
}

func (s *AssessmentService) ListPeriodTemplates(ctx context.Context) ([]AssessmentPeriodTemplateItem, error) {
	return s.loadPeriodTemplatesTx(s.db.WithContext(ctx))
}

func (s *AssessmentService) UpdatePeriodTemplates(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	input []AssessmentPeriodTemplateItem,
	ipAddress string,
	userAgent string,
) ([]AssessmentPeriodTemplateItem, error) {
	if err := requireRootOrAssessmentAdminClaims(claims); err != nil {
		return nil, err
	}
	items, err := normalizePeriodTemplateItems(input)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("failed to encode period templates: %w", err)
	}

	operator := operatorID
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operator)
	now := time.Now().Unix()
	var targetID uint
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		operatorRef = resolveBusinessWriteOperatorRefTx(tx, operator)
		var setting model.SystemSetting
		findErr := tx.Where("setting_key = ?", assessmentPeriodTemplatesSettingKey).First(&setting).Error
		switch {
		case findErr == nil:
			setting.SettingValue = string(raw)
			setting.SettingType = "json"
			setting.Description = "Assessment period templates for creating new years"
			setting.IsSystem = true
			setting.UpdatedBy = operatorRef
			setting.UpdatedAt = now
			if err := tx.Save(&setting).Error; err != nil {
				return fmt.Errorf("failed to update period template setting: %w", err)
			}
			targetID = setting.ID
		case repository.IsRecordNotFound(findErr):
			setting = model.SystemSetting{
				SettingKey:   assessmentPeriodTemplatesSettingKey,
				SettingValue: string(raw),
				SettingType:  "json",
				Description:  "Assessment period templates for creating new years",
				IsSystem:     true,
				UpdatedBy:    operatorRef,
				UpdatedAt:    now,
			}
			if err := tx.Create(&setting).Error; err != nil {
				return fmt.Errorf("failed to create period template setting: %w", err)
			}
			targetID = setting.ID
		default:
			return fmt.Errorf("failed to query period template setting: %w", findErr)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	_ = s.auditRepo.Create(ctx, buildAuditRecord(operatorRef, "update", "system_settings", &targetID, map[string]any{
		"event":      "update_assessment_period_templates",
		"settingKey": assessmentPeriodTemplatesSettingKey,
		"count":      len(items),
	}, ipAddress, userAgent))

	return items, nil
}

func (s *AssessmentService) loadPeriodTemplatesTx(tx *gorm.DB) ([]AssessmentPeriodTemplateItem, error) {
	var setting model.SystemSetting
	err := tx.Where("setting_key = ?", assessmentPeriodTemplatesSettingKey).First(&setting).Error
	switch {
	case err == nil:
		items, decodeErr := decodePeriodTemplateSetting(setting.SettingValue)
		if decodeErr != nil {
			return defaultPeriodTemplates(), nil
		}
		return items, nil
	case repository.IsRecordNotFound(err):
		return defaultPeriodTemplates(), nil
	default:
		return nil, fmt.Errorf("failed to query period template setting: %w", err)
	}
}

func buildPeriodsFromTemplates(
	yearID uint,
	operatorID *uint,
	templates []AssessmentPeriodTemplateItem,
) ([]model.AssessmentPeriod, error) {
	items := make([]model.AssessmentPeriod, 0, len(templates))
	for _, template := range templates {
		items = append(items, model.AssessmentPeriod{
			YearID:     yearID,
			PeriodCode: template.PeriodCode,
			PeriodName: template.PeriodName,
			Status:     assessmentStatusPreparing,
			CreatedBy:  operatorID,
			UpdatedBy:  operatorID,
		})
	}
	return items, nil
}

func decodePeriodTemplateSetting(raw string) ([]AssessmentPeriodTemplateItem, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return defaultPeriodTemplates(), nil
	}

	var direct []AssessmentPeriodTemplateItem
	if err := json.Unmarshal([]byte(text), &direct); err == nil {
		return normalizePeriodTemplateItems(direct)
	}

	var envelope struct {
		Items []AssessmentPeriodTemplateItem `json:"items"`
	}
	if err := json.Unmarshal([]byte(text), &envelope); err == nil {
		return normalizePeriodTemplateItems(envelope.Items)
	}

	return nil, ErrInvalidPeriodTemplate
}

func normalizePeriodTemplateItems(input []AssessmentPeriodTemplateItem) ([]AssessmentPeriodTemplateItem, error) {
	if len(input) == 0 || len(input) > 24 {
		return nil, ErrInvalidPeriodTemplate
	}

	items := make([]AssessmentPeriodTemplateItem, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for index, item := range input {
		code := normalizePeriodCode(item.PeriodCode)
		name := strings.TrimSpace(item.PeriodName)
		sortOrder := item.SortOrder

		if !isValidPeriodCode(code) || name == "" {
			return nil, ErrInvalidPeriodTemplate
		}
		if len(name) > 100 {
			return nil, ErrInvalidPeriodTemplate
		}
		if _, exists := seen[code]; exists {
			return nil, ErrInvalidPeriodTemplate
		}
		seen[code] = struct{}{}

		if sortOrder <= 0 {
			sortOrder = index + 1
		}
		items = append(items, AssessmentPeriodTemplateItem{
			PeriodCode: code,
			PeriodName: name,
			SortOrder:  sortOrder,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SortOrder != items[j].SortOrder {
			return items[i].SortOrder < items[j].SortOrder
		}
		return items[i].PeriodCode < items[j].PeriodCode
	})
	for index := range items {
		items[index].SortOrder = index + 1
	}
	return items, nil
}

func defaultPeriodTemplates() []AssessmentPeriodTemplateItem {
	return []AssessmentPeriodTemplateItem{
		{PeriodCode: "Q1", PeriodName: "\u7b2c\u4e00\u5b63\u5ea6", SortOrder: 1},
		{PeriodCode: "Q2", PeriodName: "\u7b2c\u4e8c\u5b63\u5ea6", SortOrder: 2},
		{PeriodCode: "Q3", PeriodName: "\u7b2c\u4e09\u5b63\u5ea6", SortOrder: 3},
		{PeriodCode: "Q4", PeriodName: "\u7b2c\u56db\u5b63\u5ea6", SortOrder: 4},
		{PeriodCode: "YEAR_END", PeriodName: "\u5e74\u7ec8\u8003\u6838", SortOrder: 5},
	}
}
