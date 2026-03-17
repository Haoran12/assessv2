package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

type AuditService struct {
	db        *gorm.DB
	auditRepo *repository.AuditRepository
}

type AuditLogListInput struct {
	Page       int
	PageSize   int
	UserID     *uint
	ActionType string
	TargetType string
	Keyword    string
	StartAt    *int64
	EndAt      *int64
}

type AuditLogListItem struct {
	ID           uint   `json:"id"`
	UserID       *uint  `json:"userId,omitempty"`
	Username     string `json:"username"`
	RealName     string `json:"realName"`
	ActionType   string `json:"actionType"`
	TargetType   string `json:"targetType"`
	TargetID     *uint  `json:"targetId,omitempty"`
	ActionDetail string `json:"actionDetail"`
	IPAddress    string `json:"ipAddress"`
	UserAgent    string `json:"userAgent"`
	CreatedAt    int64  `json:"createdAt"`
}

type AuditLogListResult struct {
	Items    []AuditLogListItem `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
}

type AuditDiffItem struct {
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}

type AuditLogDetail struct {
	AuditLogListItem
	Detail      map[string]any  `json:"detail"`
	Diffs       []AuditDiffItem `json:"diffs"`
	CanRollback bool            `json:"canRollback"`
}

func NewAuditService(db *gorm.DB, auditRepo *repository.AuditRepository) *AuditService {
	return &AuditService{
		db:        db,
		auditRepo: auditRepo,
	}
}

func (s *AuditService) List(ctx context.Context, input AuditLogListInput) (*AuditLogListResult, error) {
	page := input.Page
	if page <= 0 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize <= 0 || pageSize > 500 {
		pageSize = 20
	}

	query := s.db.WithContext(ctx).
		Table("audit_logs AS a").
		Joins("LEFT JOIN users AS u ON u.id = a.user_id")

	if input.UserID != nil {
		query = query.Where("a.user_id = ?", *input.UserID)
	}
	if text := strings.TrimSpace(input.ActionType); text != "" {
		query = query.Where("a.action_type = ?", text)
	}
	if text := strings.TrimSpace(input.TargetType); text != "" {
		query = query.Where("a.target_type = ?", text)
	}
	if input.StartAt != nil {
		query = query.Where("a.created_at >= ?", *input.StartAt)
	}
	if input.EndAt != nil {
		query = query.Where("a.created_at <= ?", *input.EndAt)
	}
	if text := strings.TrimSpace(input.Keyword); text != "" {
		like := "%" + text + "%"
		query = query.Where(
			"(a.action_detail LIKE ? OR COALESCE(u.username, '') LIKE ? OR COALESCE(u.real_name, '') LIKE ?)",
			like,
			like,
			like,
		)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count audit logs: %w", err)
	}

	type row struct {
		ID           uint   `gorm:"column:id"`
		UserID       *uint  `gorm:"column:user_id"`
		Username     string `gorm:"column:username"`
		RealName     string `gorm:"column:real_name"`
		ActionType   string `gorm:"column:action_type"`
		TargetType   string `gorm:"column:target_type"`
		TargetID     *uint  `gorm:"column:target_id"`
		ActionDetail string `gorm:"column:action_detail"`
		IPAddress    string `gorm:"column:ip_address"`
		UserAgent    string `gorm:"column:user_agent"`
		CreatedAt    int64  `gorm:"column:created_at"`
	}
	rows := make([]row, 0, pageSize)
	if err := query.
		Select(
			"a.id, a.user_id, COALESCE(u.username, '') AS username, COALESCE(u.real_name, '') AS real_name, " +
				"a.action_type, COALESCE(a.target_type, '') AS target_type, a.target_id, COALESCE(a.action_detail, '') AS action_detail, " +
				"COALESCE(a.ip_address, '') AS ip_address, COALESCE(a.user_agent, '') AS user_agent, a.created_at",
		).
		Order("a.created_at DESC, a.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}

	items := make([]AuditLogListItem, 0, len(rows))
	for _, item := range rows {
		items = append(items, AuditLogListItem{
			ID:           item.ID,
			UserID:       item.UserID,
			Username:     item.Username,
			RealName:     item.RealName,
			ActionType:   item.ActionType,
			TargetType:   item.TargetType,
			TargetID:     item.TargetID,
			ActionDetail: item.ActionDetail,
			IPAddress:    item.IPAddress,
			UserAgent:    item.UserAgent,
			CreatedAt:    item.CreatedAt,
		})
	}

	return &AuditLogListResult{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *AuditService) Detail(ctx context.Context, auditLogID uint) (*AuditLogDetail, error) {
	type row struct {
		ID           uint   `gorm:"column:id"`
		UserID       *uint  `gorm:"column:user_id"`
		Username     string `gorm:"column:username"`
		RealName     string `gorm:"column:real_name"`
		ActionType   string `gorm:"column:action_type"`
		TargetType   string `gorm:"column:target_type"`
		TargetID     *uint  `gorm:"column:target_id"`
		ActionDetail string `gorm:"column:action_detail"`
		IPAddress    string `gorm:"column:ip_address"`
		UserAgent    string `gorm:"column:user_agent"`
		CreatedAt    int64  `gorm:"column:created_at"`
	}

	var item row
	if err := s.db.WithContext(ctx).
		Table("audit_logs AS a").
		Joins("LEFT JOIN users AS u ON u.id = a.user_id").
		Select(
			"a.id, a.user_id, COALESCE(u.username, '') AS username, COALESCE(u.real_name, '') AS real_name, "+
				"a.action_type, COALESCE(a.target_type, '') AS target_type, a.target_id, COALESCE(a.action_detail, '') AS action_detail, "+
				"COALESCE(a.ip_address, '') AS ip_address, COALESCE(a.user_agent, '') AS user_agent, a.created_at",
		).
		Where("a.id = ?", auditLogID).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAuditLogNotFound
		}
		return nil, fmt.Errorf("failed to query audit log detail: %w", err)
	}

	detail := decodeActionDetail(item.ActionDetail)
	diffs := diffActionDetail(detail)
	canRollback := s.isRollbackSupported(item.TargetType, item.ActionType, detail)

	return &AuditLogDetail{
		AuditLogListItem: AuditLogListItem{
			ID:           item.ID,
			UserID:       item.UserID,
			Username:     item.Username,
			RealName:     item.RealName,
			ActionType:   item.ActionType,
			TargetType:   item.TargetType,
			TargetID:     item.TargetID,
			ActionDetail: item.ActionDetail,
			IPAddress:    item.IPAddress,
			UserAgent:    item.UserAgent,
			CreatedAt:    item.CreatedAt,
		},
		Detail:      detail,
		Diffs:       diffs,
		CanRollback: canRollback,
	}, nil
}

func (s *AuditService) Rollback(
	ctx context.Context,
	operatorID uint,
	auditLogID uint,
	ipAddress string,
	userAgent string,
) error {
	var logRecord model.AuditLog
	if err := s.db.WithContext(ctx).Where("id = ?", auditLogID).First(&logRecord).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAuditLogNotFound
		}
		return fmt.Errorf("failed to query audit log: %w", err)
	}

	detail := decodeActionDetail(logRecord.ActionDetail)
	if !s.isRollbackSupported(logRecord.TargetType, logRecord.ActionType, detail) {
		return ErrAuditRollbackUnsupported
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		switch strings.ToLower(strings.TrimSpace(logRecord.ActionType)) {
		case "update":
			before := pickMap(detail, "before")
			if len(before) == 0 {
				return ErrAuditRollbackUnsupported
			}
			return restoreSystemSetting(tx, before, operatorID)
		case "delete":
			before := pickMap(detail, "before")
			if len(before) == 0 {
				return ErrAuditRollbackUnsupported
			}
			return restoreSystemSetting(tx, before, operatorID)
		case "create":
			after := pickMap(detail, "after")
			key := extractString(after, "setting_key")
			if key == "" {
				key = extractString(after, "settingKey")
			}
			if key == "" {
				return ErrAuditRollbackUnsupported
			}
			return tx.Where("setting_key = ?", key).Delete(&model.SystemSetting{}).Error
		default:
			return ErrAuditRollbackUnsupported
		}
	}); err != nil {
		return err
	}

	sourceID := auditLogID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		&operatorID,
		"update",
		"audit_logs",
		&sourceID,
		map[string]any{
			"event":            "rollback_audit_log",
			"source_log_id":    auditLogID,
			"source_target":    logRecord.TargetType,
			"source_action":    logRecord.ActionType,
			"rollback_applied": true,
		},
		ipAddress,
		userAgent,
	))

	return nil
}

func (s *AuditService) isRollbackSupported(targetType, actionType string, detail map[string]any) bool {
	if strings.TrimSpace(targetType) != "system_settings" {
		return false
	}
	action := strings.ToLower(strings.TrimSpace(actionType))
	switch action {
	case "create":
		after := pickMap(detail, "after")
		return extractString(after, "setting_key") != "" || extractString(after, "settingKey") != ""
	case "update", "delete":
		before := pickMap(detail, "before")
		return extractString(before, "setting_key") != "" || extractString(before, "settingKey") != ""
	default:
		return false
	}
}

func restoreSystemSetting(tx *gorm.DB, payload map[string]any, operatorID uint) error {
	key := extractString(payload, "setting_key")
	if key == "" {
		key = extractString(payload, "settingKey")
	}
	if key == "" {
		return ErrAuditRollbackUnsupported
	}

	value := extractString(payload, "setting_value")
	if value == "" {
		value = extractString(payload, "settingValue")
	}
	settingType := extractString(payload, "setting_type")
	if settingType == "" {
		settingType = extractString(payload, "settingType")
	}
	if settingType == "" {
		settingType = "string"
	}

	description := extractString(payload, "description")
	isSystem := extractBool(payload, "is_system")
	if _, exists := payload["isSystem"]; exists {
		isSystem = extractBool(payload, "isSystem")
	}

	now := time.Now().Unix()
	var existing model.SystemSetting
	err := tx.Where("setting_key = ?", key).First(&existing).Error
	switch {
	case err == nil:
		existing.SettingValue = value
		existing.SettingType = settingType
		existing.Description = description
		existing.IsSystem = isSystem
		existing.UpdatedBy = &operatorID
		existing.UpdatedAt = now
		return tx.Save(&existing).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		record := model.SystemSetting{
			SettingKey:   key,
			SettingValue: value,
			SettingType:  settingType,
			Description:  description,
			IsSystem:     isSystem,
			UpdatedBy:    &operatorID,
			UpdatedAt:    now,
		}
		return tx.Create(&record).Error
	default:
		return err
	}
}

func decodeActionDetail(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}
	}
	result := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return map[string]any{
			"_raw": raw,
		}
	}
	return result
}

func diffActionDetail(detail map[string]any) []AuditDiffItem {
	before := pickMap(detail, "before")
	after := pickMap(detail, "after")
	if len(before) == 0 && len(after) == 0 {
		return []AuditDiffItem{}
	}

	fieldSet := map[string]struct{}{}
	for key := range before {
		fieldSet[key] = struct{}{}
	}
	for key := range after {
		fieldSet[key] = struct{}{}
	}
	fields := make([]string, 0, len(fieldSet))
	for key := range fieldSet {
		fields = append(fields, key)
	}
	sort.Strings(fields)

	items := make([]AuditDiffItem, 0, len(fields))
	for _, key := range fields {
		beforeValue, beforeExists := before[key]
		afterValue, afterExists := after[key]
		if !beforeExists && !afterExists {
			continue
		}
		if fmt.Sprintf("%v", beforeValue) == fmt.Sprintf("%v", afterValue) {
			continue
		}
		items = append(items, AuditDiffItem{
			Field:  key,
			Before: beforeValue,
			After:  afterValue,
		})
	}
	return items
}

func pickMap(data map[string]any, key string) map[string]any {
	value, exists := data[key]
	if !exists {
		return map[string]any{}
	}
	switch object := value.(type) {
	case map[string]any:
		return object
	default:
		buffer, err := json.Marshal(object)
		if err != nil {
			return map[string]any{}
		}
		result := map[string]any{}
		if err := json.Unmarshal(buffer, &result); err != nil {
			return map[string]any{}
		}
		return result
	}
}

func extractString(data map[string]any, key string) string {
	value, exists := data[key]
	if !exists || value == nil {
		return ""
	}
	switch text := value.(type) {
	case string:
		return strings.TrimSpace(text)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", text))
	}
}

func extractBool(data map[string]any, key string) bool {
	value, exists := data[key]
	if !exists || value == nil {
		return false
	}
	switch flag := value.(type) {
	case bool:
		return flag
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(flag))
		if err != nil {
			return false
		}
		return parsed
	default:
		return false
	}
}
