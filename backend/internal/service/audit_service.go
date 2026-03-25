package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type AuditService struct {
	db     *gorm.DB
	userDB *gorm.DB
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
	EventCode    string `json:"eventCode"`
	Summary      string `json:"summary"`
	ChangeCount  int    `json:"changeCount"`
	HasDiff      bool   `json:"hasDiff"`
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
	Field      string `json:"field"`
	Label      string `json:"label,omitempty"`
	Before     any    `json:"before"`
	After      any    `json:"after"`
	ChangeType string `json:"changeType,omitempty"`
}

type AuditLogDetail struct {
	AuditLogListItem
	Detail map[string]any  `json:"detail"`
	Diffs  []AuditDiffItem `json:"diffs"`
}

func NewAuditService(db *gorm.DB, userDB *gorm.DB) *AuditService {
	if userDB == nil {
		userDB = db
	}
	return &AuditService{
		db:     db,
		userDB: userDB,
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
		Table("audit_logs AS a")

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
		userIDs, err := s.lookupUserIDsByKeyword(ctx, text)
		if err == nil && len(userIDs) > 0 {
			query = query.Where("(a.action_detail LIKE ? OR a.summary LIKE ? OR a.event_code LIKE ? OR a.user_id IN ?)", like, like, like, userIDs)
		} else {
			query = query.Where("(a.action_detail LIKE ? OR a.summary LIKE ? OR a.event_code LIKE ?)", like, like, like)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count audit logs: %w", err)
	}

	type row struct {
		ID           uint   `gorm:"column:id"`
		UserID       *uint  `gorm:"column:user_id"`
		ActionType   string `gorm:"column:action_type"`
		TargetType   string `gorm:"column:target_type"`
		TargetID     *uint  `gorm:"column:target_id"`
		EventCode    string `gorm:"column:event_code"`
		Summary      string `gorm:"column:summary"`
		ChangeCount  int    `gorm:"column:change_count"`
		HasDiff      bool   `gorm:"column:has_diff"`
		ActionDetail string `gorm:"column:action_detail"`
		IPAddress    string `gorm:"column:ip_address"`
		UserAgent    string `gorm:"column:user_agent"`
		CreatedAt    int64  `gorm:"column:created_at"`
	}
	rows := make([]row, 0, pageSize)
	if err := query.
		Select(
			"a.id, a.user_id, a.action_type, COALESCE(a.target_type, '') AS target_type, a.target_id, " +
				"COALESCE(a.event_code, '') AS event_code, COALESCE(a.summary, '') AS summary, COALESCE(a.change_count, 0) AS change_count, " +
				"COALESCE(a.has_diff, 0) AS has_diff, COALESCE(a.action_detail, '') AS action_detail, " +
				"COALESCE(a.ip_address, '') AS ip_address, COALESCE(a.user_agent, '') AS user_agent, a.created_at",
		).
		Order("a.created_at DESC, a.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}

	userIDSet := map[uint]struct{}{}
	for _, row := range rows {
		if row.UserID != nil {
			userIDSet[*row.UserID] = struct{}{}
		}
	}
	userIDs := make([]uint, 0, len(userIDSet))
	for userID := range userIDSet {
		userIDs = append(userIDs, userID)
	}

	profiles, err := s.loadUserProfilesByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	items := make([]AuditLogListItem, 0, len(rows))
	for _, item := range rows {
		username := ""
		realName := ""
		if item.UserID != nil {
			if profile, ok := profiles[*item.UserID]; ok {
				username = profile.Username
				realName = profile.Username
			}
		}
		detail := decodeActionDetail(item.ActionDetail, item.ActionType, item.TargetType, item.TargetID)
		diffs := diffActionDetail(detail)
		eventCode := strings.TrimSpace(item.EventCode)
		if eventCode == "" {
			eventCode = extractString(detail, "eventCode")
		}
		summary := strings.TrimSpace(item.Summary)
		if summary == "" {
			summary = extractString(detail, "summary")
		}
		changeCount := item.ChangeCount
		if changeCount <= 0 && len(diffs) > 0 {
			changeCount = len(diffs)
		}
		hasDiff := item.HasDiff || changeCount > 0

		items = append(items, AuditLogListItem{
			ID:           item.ID,
			UserID:       item.UserID,
			Username:     username,
			RealName:     realName,
			ActionType:   item.ActionType,
			TargetType:   item.TargetType,
			TargetID:     item.TargetID,
			EventCode:    eventCode,
			Summary:      summary,
			ChangeCount:  changeCount,
			HasDiff:      hasDiff,
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
		ActionType   string `gorm:"column:action_type"`
		TargetType   string `gorm:"column:target_type"`
		TargetID     *uint  `gorm:"column:target_id"`
		EventCode    string `gorm:"column:event_code"`
		Summary      string `gorm:"column:summary"`
		ChangeCount  int    `gorm:"column:change_count"`
		HasDiff      bool   `gorm:"column:has_diff"`
		ActionDetail string `gorm:"column:action_detail"`
		IPAddress    string `gorm:"column:ip_address"`
		UserAgent    string `gorm:"column:user_agent"`
		CreatedAt    int64  `gorm:"column:created_at"`
	}

	var item row
	if err := s.db.WithContext(ctx).
		Table("audit_logs AS a").
		Select(
			"a.id, a.user_id, a.action_type, COALESCE(a.target_type, '') AS target_type, a.target_id, "+
				"COALESCE(a.event_code, '') AS event_code, COALESCE(a.summary, '') AS summary, COALESCE(a.change_count, 0) AS change_count, "+
				"COALESCE(a.has_diff, 0) AS has_diff, COALESCE(a.action_detail, '') AS action_detail, "+
				"COALESCE(a.ip_address, '') AS ip_address, COALESCE(a.user_agent, '') AS user_agent, a.created_at",
		).
		Where("a.id = ?", auditLogID).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAuditLogNotFound
		}
		return nil, fmt.Errorf("failed to query audit log detail: %w", err)
	}

	username := ""
	realName := ""
	if item.UserID != nil {
		profile, err := s.loadUserProfileByID(ctx, *item.UserID)
		if err != nil {
			return nil, err
		}
		if profile != nil {
			username = profile.Username
			realName = profile.Username
		}
	}

	detail := decodeActionDetail(item.ActionDetail, item.ActionType, item.TargetType, item.TargetID)
	diffs := diffActionDetail(detail)
	eventCode := strings.TrimSpace(item.EventCode)
	if eventCode == "" {
		eventCode = extractString(detail, "eventCode")
	}
	summary := strings.TrimSpace(item.Summary)
	if summary == "" {
		summary = extractString(detail, "summary")
	}
	changeCount := item.ChangeCount
	if changeCount <= 0 && len(diffs) > 0 {
		changeCount = len(diffs)
	}
	hasDiff := item.HasDiff || changeCount > 0

	return &AuditLogDetail{
		AuditLogListItem: AuditLogListItem{
			ID:           item.ID,
			UserID:       item.UserID,
			Username:     username,
			RealName:     realName,
			ActionType:   item.ActionType,
			TargetType:   item.TargetType,
			TargetID:     item.TargetID,
			EventCode:    eventCode,
			Summary:      summary,
			ChangeCount:  changeCount,
			HasDiff:      hasDiff,
			ActionDetail: item.ActionDetail,
			IPAddress:    item.IPAddress,
			UserAgent:    item.UserAgent,
			CreatedAt:    item.CreatedAt,
		},
		Detail: detail,
		Diffs:  diffs,
	}, nil
}

type auditUserProfile struct {
	ID       uint   `gorm:"column:id"`
	Username string `gorm:"column:username"`
}

func (s *AuditService) lookupUserIDsByKeyword(ctx context.Context, keyword string) ([]uint, error) {
	like := "%" + strings.TrimSpace(keyword) + "%"
	ids := make([]uint, 0)
	if err := s.userDB.WithContext(ctx).
		Table("users").
		Select("id").
		Where("deleted_at IS NULL").
		Where("username LIKE ?", like).
		Scan(&ids).Error; err != nil {
		// User lookup is best-effort to avoid breaking audit logs when accounts schema is unavailable.
		return nil, nil
	}
	return ids, nil
}

func (s *AuditService) loadUserProfilesByIDs(ctx context.Context, userIDs []uint) (map[uint]auditUserProfile, error) {
	if len(userIDs) == 0 {
		return map[uint]auditUserProfile{}, nil
	}

	profiles := make([]auditUserProfile, 0, len(userIDs))
	if err := s.userDB.WithContext(ctx).
		Table("users").
		Select("id, username").
		Where("deleted_at IS NULL").
		Where("id IN ?", userIDs).
		Scan(&profiles).Error; err != nil {
		// User profile lookup is best-effort to avoid breaking audit logs when accounts schema is unavailable.
		return map[uint]auditUserProfile{}, nil
	}

	index := make(map[uint]auditUserProfile, len(profiles))
	for _, item := range profiles {
		index[item.ID] = item
	}
	return index, nil
}

func (s *AuditService) loadUserProfileByID(ctx context.Context, userID uint) (*auditUserProfile, error) {
	var profile auditUserProfile
	if err := s.userDB.WithContext(ctx).
		Table("users").
		Select("id, username").
		Where("deleted_at IS NULL").
		Where("id = ?", userID).
		First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		// User profile lookup is best-effort to avoid breaking audit logs when accounts schema is unavailable.
		return nil, nil
	}
	return &profile, nil
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
