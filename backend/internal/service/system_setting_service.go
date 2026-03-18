package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

var settingKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{2,100}$`)

var voteGradeOptionSet = map[string]struct{}{
	"excellent": {},
	"good":      {},
	"average":   {},
	"poor":      {},
}

const (
	objectLinkTypeMaxLength = 30
	objectLinkTypeMaxCount  = 20

	voteGradeScoreMin = 0
	voteGradeScoreMax = 100
)

type SystemSettingService struct {
	db        *gorm.DB
	auditRepo *repository.AuditRepository
}

type SystemSettingDTO struct {
	ID           uint   `json:"id"`
	SettingKey   string `json:"settingKey"`
	SettingValue string `json:"settingValue"`
	SettingType  string `json:"settingType"`
	Value        any    `json:"value"`
	Description  string `json:"description"`
	IsSystem     bool   `json:"isSystem"`
	UpdatedBy    *uint  `json:"updatedBy,omitempty"`
	UpdatedAt    int64  `json:"updatedAt"`
}

type ListSystemSettingsResult struct {
	Items      []SystemSettingDTO `json:"items"`
	Basic      map[string]any     `json:"basic"`
	Assessment map[string]any     `json:"assessment"`
	Security   map[string]any     `json:"security"`
	Backup     map[string]any     `json:"backup"`
	Other      map[string]any     `json:"other"`
}

type UpdateSystemSettingItem struct {
	SettingKey   string `json:"settingKey"`
	SettingValue any    `json:"settingValue"`
}

func NewSystemSettingService(db *gorm.DB, auditRepo *repository.AuditRepository) *SystemSettingService {
	return &SystemSettingService{
		db:        db,
		auditRepo: auditRepo,
	}
}

func (s *SystemSettingService) List(ctx context.Context) (*ListSystemSettingsResult, error) {
	var settings []model.SystemSetting
	if err := s.db.WithContext(ctx).
		Order("setting_key ASC").
		Find(&settings).Error; err != nil {
		return nil, fmt.Errorf("failed to query system settings: %w", err)
	}

	result := &ListSystemSettingsResult{
		Items:      make([]SystemSettingDTO, 0, len(settings)),
		Basic:      map[string]any{},
		Assessment: map[string]any{},
		Security:   map[string]any{},
		Backup:     map[string]any{},
		Other:      map[string]any{},
	}

	for _, item := range settings {
		decoded := decodeSettingValue(item.SettingType, item.SettingValue)
		dto := SystemSettingDTO{
			ID:           item.ID,
			SettingKey:   item.SettingKey,
			SettingValue: item.SettingValue,
			SettingType:  item.SettingType,
			Value:        decoded,
			Description:  item.Description,
			IsSystem:     item.IsSystem,
			UpdatedBy:    item.UpdatedBy,
			UpdatedAt:    item.UpdatedAt,
		}
		result.Items = append(result.Items, dto)

		group := resolveSettingGroup(item.SettingKey)
		switch group {
		case "basic":
			result.Basic[item.SettingKey] = decoded
		case "assessment":
			result.Assessment[item.SettingKey] = decoded
		case "security":
			result.Security[item.SettingKey] = decoded
		case "backup":
			result.Backup[item.SettingKey] = decoded
		default:
			result.Other[item.SettingKey] = decoded
		}
	}

	return result, nil
}

func (s *SystemSettingService) Update(
	ctx context.Context,
	operatorID uint,
	items []UpdateSystemSettingItem,
	ipAddress string,
	userAgent string,
) (*ListSystemSettingsResult, error) {
	if len(items) == 0 {
		return nil, ErrInvalidSettingKey
	}

	now := time.Now().Unix()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		operatorRef := resolveBusinessWriteOperatorRefTx(tx, operatorID)
		for _, item := range items {
			key := strings.TrimSpace(item.SettingKey)
			if !settingKeyPattern.MatchString(key) {
				return ErrInvalidSettingKey
			}

			var record model.SystemSetting
			findErr := tx.Where("setting_key = ?", key).First(&record).Error
			isCreate := false
			switch {
			case findErr == nil:
				// update below
			case errors.Is(findErr, gorm.ErrRecordNotFound):
				record = model.SystemSetting{
					SettingKey: key,
					IsSystem:   false,
				}
				isCreate = true
			default:
				return fmt.Errorf("failed to query setting %s: %w", key, findErr)
			}

			before := serializeSettingForAudit(&record)
			value, settingType, err := normalizeSettingValue(key, item.SettingValue, record.SettingType)
			if err != nil {
				return err
			}
			record.SettingValue = value
			record.SettingType = settingType
			record.UpdatedBy = operatorRef
			record.UpdatedAt = now

			if isCreate {
				if err := tx.Create(&record).Error; err != nil {
					return fmt.Errorf("failed to create setting %s: %w", key, err)
				}
			} else {
				if err := tx.Save(&record).Error; err != nil {
					return fmt.Errorf("failed to update setting %s: %w", key, err)
				}
			}

			targetID := record.ID
			actionType := "update"
			if isCreate {
				actionType = "create"
			}
			after := serializeSettingForAudit(&record)
			auditRecord := buildAuditRecord(
				operatorRef,
				actionType,
				"system_settings",
				&targetID,
				map[string]any{
					"event":  "update_system_setting",
					"before": before,
					"after":  after,
				},
				ipAddress,
				userAgent,
			)
			if err := tx.Create(&auditRecord).Error; err != nil {
				return fmt.Errorf("failed to create settings audit record: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.List(ctx)
}

func normalizeSettingValue(settingKey string, raw any, existingType string) (string, string, error) {
	settingType := strings.ToLower(strings.TrimSpace(existingType))
	if settingType == "" {
		settingType = inferSettingTypeFromRaw(raw)
	}

	switch settingType {
	case "boolean":
		flag, err := parseBool(raw)
		if err != nil {
			return "", "", ErrInvalidSettingValue
		}
		value := strconv.FormatBool(flag)
		if err := validateSettingValue(settingKey, settingType, value); err != nil {
			return "", "", err
		}
		return value, settingType, nil
	case "number":
		numberText, err := parseNumber(raw)
		if err != nil {
			return "", "", ErrInvalidSettingValue
		}
		if err := validateSettingValue(settingKey, settingType, numberText); err != nil {
			return "", "", err
		}
		return numberText, settingType, nil
	case "json":
		jsonText, err := parseJSON(raw)
		if err != nil {
			return "", "", ErrInvalidSettingValue
		}
		if err := validateSettingValue(settingKey, settingType, jsonText); err != nil {
			return "", "", err
		}
		return jsonText, settingType, nil
	default:
		text := strings.TrimSpace(fmt.Sprintf("%v", raw))
		if err := validateSettingValue(settingKey, "string", text); err != nil {
			return "", "", err
		}
		return text, "string", nil
	}
}

func inferSettingTypeFromRaw(raw any) string {
	switch raw.(type) {
	case bool:
		return "boolean"
	case float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "number"
	case map[string]any, []any:
		return "json"
	default:
		return "string"
	}
}

func validateSettingValue(settingKey, settingType, value string) error {
	switch settingKey {
	case "score.decimal_places":
		number, err := strconv.Atoi(value)
		if err != nil || number < 0 || number > 6 {
			return ErrInvalidSettingValue
		}
	case "backup.retention_days", "backup.max_count", "audit.retention_days", "security.session_timeout_minutes":
		number, err := strconv.Atoi(value)
		if err != nil || number <= 0 {
			return ErrInvalidSettingValue
		}
	case "backup.auto_time", "vote.deadline_time":
		if _, err := time.Parse("15:04", value); err != nil {
			return ErrInvalidSettingValue
		}
	case "backup.auto_enabled":
		if _, err := strconv.ParseBool(value); err != nil {
			return ErrInvalidSettingValue
		}
	case "system.timezone":
		if _, err := time.LoadLocation(value); err != nil {
			return ErrInvalidSettingValue
		}
	case "assessment.object_link_types":
		if err := validateObjectLinkTypeSetting(value); err != nil {
			return ErrInvalidSettingValue
		}
	case "vote.grade_scores":
		if err := validateVoteGradeScoresSetting(value); err != nil {
			return ErrInvalidSettingValue
		}
	}

	switch settingType {
	case "number":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return ErrInvalidSettingValue
		}
	case "boolean":
		if _, err := strconv.ParseBool(value); err != nil {
			return ErrInvalidSettingValue
		}
	case "json":
		if !json.Valid([]byte(value)) {
			return ErrInvalidSettingValue
		}
	}

	return nil
}

func validateObjectLinkTypeSetting(value string) error {
	var raw any
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		return err
	}

	items, ok := raw.([]any)
	if !ok || len(items) == 0 || len(items) > objectLinkTypeMaxCount {
		return ErrInvalidSettingValue
	}

	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return ErrInvalidSettingValue
		}
		normalized := strings.ToLower(strings.TrimSpace(text))
		if normalized == "" || len(normalized) > objectLinkTypeMaxLength {
			return ErrInvalidSettingValue
		}
		if _, exists := seen[normalized]; exists {
			return ErrInvalidSettingValue
		}
		seen[normalized] = struct{}{}
	}
	return nil
}

func validateVoteGradeScoresSetting(value string) error {
	var raw any
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		return err
	}

	object, ok := raw.(map[string]any)
	if !ok {
		return ErrInvalidSettingValue
	}

	normalizedScores := make(map[string]float64, len(object))
	for key, rawScore := range object {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		if _, exists := voteGradeOptionSet[normalizedKey]; !exists {
			return ErrInvalidSettingValue
		}
		if _, duplicated := normalizedScores[normalizedKey]; duplicated {
			return ErrInvalidSettingValue
		}

		score, ok := rawScore.(float64)
		if !ok || math.IsNaN(score) || math.IsInf(score, 0) {
			return ErrInvalidSettingValue
		}
		if score < voteGradeScoreMin || score > voteGradeScoreMax {
			return ErrInvalidSettingValue
		}
		normalizedScores[normalizedKey] = score
	}

	if len(normalizedScores) != len(voteGradeOptionSet) {
		return ErrInvalidSettingValue
	}
	return nil
}

func parseBool(raw any) (bool, error) {
	switch value := raw.(type) {
	case bool:
		return value, nil
	case string:
		return strconv.ParseBool(strings.TrimSpace(value))
	default:
		return false, fmt.Errorf("unsupported bool type")
	}
}

func parseNumber(raw any) (string, error) {
	switch value := raw.(type) {
	case int:
		return strconv.Itoa(value), nil
	case int8:
		return strconv.FormatInt(int64(value), 10), nil
	case int16:
		return strconv.FormatInt(int64(value), 10), nil
	case int32:
		return strconv.FormatInt(int64(value), 10), nil
	case int64:
		return strconv.FormatInt(value, 10), nil
	case uint:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint64:
		return strconv.FormatUint(value, 10), nil
	case float32:
		return strconv.FormatFloat(float64(value), 'f', -1, 64), nil
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64), nil
	case string:
		text := strings.TrimSpace(value)
		if text == "" {
			return "", fmt.Errorf("empty number")
		}
		if _, err := strconv.ParseFloat(text, 64); err != nil {
			return "", err
		}
		return text, nil
	default:
		return "", fmt.Errorf("unsupported number type")
	}
}

func parseJSON(raw any) (string, error) {
	switch value := raw.(type) {
	case string:
		text := strings.TrimSpace(value)
		if !json.Valid([]byte(text)) {
			return "", fmt.Errorf("invalid json")
		}
		return text, nil
	default:
		buffer, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return string(buffer), nil
	}
}

func decodeSettingValue(settingType, settingValue string) any {
	switch strings.ToLower(strings.TrimSpace(settingType)) {
	case "boolean":
		flag, err := strconv.ParseBool(settingValue)
		if err != nil {
			return false
		}
		return flag
	case "number":
		number, err := strconv.ParseFloat(settingValue, 64)
		if err != nil {
			return 0
		}
		return number
	case "json":
		var object any
		if err := json.Unmarshal([]byte(settingValue), &object); err != nil {
			return map[string]any{}
		}
		return object
	default:
		return settingValue
	}
}

func resolveSettingGroup(settingKey string) string {
	switch {
	case strings.HasPrefix(settingKey, "system."), strings.HasPrefix(settingKey, "score."):
		return "basic"
	case strings.HasPrefix(settingKey, "assessment."), strings.HasPrefix(settingKey, "vote."):
		return "assessment"
	case strings.HasPrefix(settingKey, "security."), strings.HasPrefix(settingKey, "audit."):
		return "security"
	case strings.HasPrefix(settingKey, "backup."):
		return "backup"
	default:
		return "other"
	}
}

func serializeSettingForAudit(setting *model.SystemSetting) map[string]any {
	if setting == nil || setting.SettingKey == "" {
		return map[string]any{}
	}
	return map[string]any{
		"id":            setting.ID,
		"setting_key":   setting.SettingKey,
		"setting_type":  setting.SettingType,
		"setting_value": setting.SettingValue,
		"description":   setting.Description,
		"is_system":     setting.IsSystem,
		"updated_by":    setting.UpdatedBy,
		"updated_at":    setting.UpdatedAt,
	}
}
