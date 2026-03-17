package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

const assessmentPeriodTemplatesSettingKey = "assessment.period_templates"

var monthDayPattern = regexp.MustCompile(`^(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$`)

type AssessmentPeriodTemplateItem struct {
	PeriodCode string `json:"periodCode"`
	PeriodName string `json:"periodName"`
	StartDay   string `json:"startDay,omitempty"`
	EndDay     string `json:"endDay,omitempty"`
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
	year int,
	operatorID *uint,
	templates []AssessmentPeriodTemplateItem,
) ([]model.AssessmentPeriod, error) {
	items := make([]model.AssessmentPeriod, 0, len(templates))
	for _, template := range templates {
		startDate, err := monthDayToDate(year, template.StartDay)
		if err != nil {
			return nil, err
		}
		endDate, err := monthDayToDate(year, template.EndDay)
		if err != nil {
			return nil, err
		}
		items = append(items, model.AssessmentPeriod{
			YearID:     yearID,
			PeriodCode: template.PeriodCode,
			PeriodName: template.PeriodName,
			Status:     assessmentStatusPreparing,
			StartDate:  startDate,
			EndDate:    endDate,
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
		startDay := strings.TrimSpace(item.StartDay)
		endDay := strings.TrimSpace(item.EndDay)
		sortOrder := item.SortOrder

		if !isValidPeriodCode(code) || name == "" {
			return nil, ErrInvalidPeriodTemplate
		}
		if len(name) > 100 {
			return nil, ErrInvalidPeriodTemplate
		}
		if startDay != "" && !monthDayPattern.MatchString(startDay) {
			return nil, ErrInvalidPeriodTemplate
		}
		if endDay != "" && !monthDayPattern.MatchString(endDay) {
			return nil, ErrInvalidPeriodTemplate
		}
		if (startDay == "") != (endDay == "") {
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
			StartDay:   startDay,
			EndDay:     endDay,
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
		{PeriodCode: "Q1", PeriodName: "\u7b2c\u4e00\u5b63\u5ea6", StartDay: "01-01", EndDay: "03-31", SortOrder: 1},
		{PeriodCode: "Q2", PeriodName: "\u7b2c\u4e8c\u5b63\u5ea6", StartDay: "04-01", EndDay: "06-30", SortOrder: 2},
		{PeriodCode: "Q3", PeriodName: "\u7b2c\u4e09\u5b63\u5ea6", StartDay: "07-01", EndDay: "09-30", SortOrder: 3},
		{PeriodCode: "Q4", PeriodName: "\u7b2c\u56db\u5b63\u5ea6", StartDay: "10-01", EndDay: "12-31", SortOrder: 4},
		{PeriodCode: "YEAR_END", PeriodName: "\u5e74\u7ec8\u8003\u6838", StartDay: "12-01", EndDay: "12-31", SortOrder: 5},
	}
}

func monthDayToDate(year int, monthDay string) (*time.Time, error) {
	text := strings.TrimSpace(monthDay)
	if text == "" {
		return nil, nil
	}
	if !monthDayPattern.MatchString(text) {
		return nil, ErrInvalidPeriodTemplate
	}
	date, err := time.ParseInLocation("2006-01-02", fmt.Sprintf("%04d-%s", year, text), time.Local)
	if err != nil {
		return nil, ErrInvalidPeriodTemplate
	}
	return &date, nil
}
