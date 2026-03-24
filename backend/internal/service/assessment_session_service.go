package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

type AssessmentSessionService struct {
	db        *gorm.DB
	auditRepo *repository.AuditRepository
}

type AssessmentSessionSummary struct {
	model.AssessmentSession
	OrganizationName string `json:"organizationName"`
}

type SessionPeriodItem struct {
	PeriodCode     string `json:"periodCode"`
	PeriodName     string `json:"periodName"`
	RuleBindingKey string `json:"ruleBindingKey"`
	SortOrder      int    `json:"sortOrder"`
}

type SessionObjectGroupItem struct {
	ObjectType string `json:"objectType"`
	GroupCode  string `json:"groupCode"`
	GroupName  string `json:"groupName"`
	SortOrder  int    `json:"sortOrder"`
}

type SessionObjectUpsertItem struct {
	ObjectType       string `json:"objectType"`
	GroupCode        string `json:"groupCode"`
	TargetType       string `json:"targetType"`
	TargetID         uint   `json:"targetId"`
	ParentTargetType string `json:"parentTargetType"`
	ParentTargetID   uint   `json:"parentTargetId"`
	SortOrder        int    `json:"sortOrder"`
	IsActive         bool   `json:"isActive"`
}

type SessionObjectCandidateItem struct {
	TargetType            string `json:"targetType"`
	TargetID              uint   `json:"targetId"`
	ObjectName            string `json:"objectName"`
	OrganizationID        uint   `json:"organizationId"`
	OrganizationName      string `json:"organizationName"`
	DepartmentID          *uint  `json:"departmentId,omitempty"`
	DepartmentName        string `json:"departmentName,omitempty"`
	RecommendedObjectType string `json:"recommendedObjectType"`
	RecommendedGroupCode  string `json:"recommendedGroupCode"`
}

type AssessmentSessionDetail struct {
	Session      AssessmentSessionSummary        `json:"session"`
	Periods      []model.AssessmentSessionPeriod `json:"periods"`
	ObjectGroups []model.AssessmentObjectGroup   `json:"objectGroups"`
	ObjectCount  int                             `json:"objectCount"`
}

type CreateAssessmentSessionInput struct {
	Year           int
	OrganizationID uint
	DisplayName    string
	Description    string
}

type UpdateAssessmentSessionInput struct {
	DisplayName string
	Description string
}

func NewAssessmentSessionService(db *gorm.DB, auditRepo *repository.AuditRepository) *AssessmentSessionService {
	return &AssessmentSessionService{db: db, auditRepo: auditRepo}
}

func (s *AssessmentSessionService) ListSessions(ctx context.Context, claims *auth.Claims) ([]AssessmentSessionSummary, error) {
	query := s.db.WithContext(ctx).
		Table("assessment_sessions AS a").
		Select("a.*, o.org_name AS organization_name").
		Joins("JOIN organizations o ON o.id = a.organization_id")

	if !isRootClaims(claims) {
		adminOrgID, err := resolveAdminOrganizationID(claims)
		if err != nil {
			return nil, err
		}
		query = query.Where("a.organization_id = ?", adminOrgID)
	}

	items := make([]AssessmentSessionSummary, 0, 16)
	if err := query.Order("a.created_at DESC, a.id DESC").Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list assessment sessions: %w", err)
	}
	return items, nil
}

func (s *AssessmentSessionService) GetSession(ctx context.Context, claims *auth.Claims, sessionID uint) (*AssessmentSessionDetail, error) {
	if sessionID == 0 {
		return nil, ErrInvalidParam
	}

	summary, err := s.loadSessionSummary(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := ensureAssessmentOrganizationScope(claims, summary.OrganizationID); err != nil {
		return nil, err
	}

	periods, err := s.listPeriods(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	groups, err := s.listObjectGroups(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	objects, err := s.ListObjects(ctx, claims, sessionID)
	if err != nil {
		return nil, err
	}

	return &AssessmentSessionDetail{
		Session:      *summary,
		Periods:      periods,
		ObjectGroups: groups,
		ObjectCount:  len(objects),
	}, nil
}

func (s *AssessmentSessionService) CreateSession(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	input CreateAssessmentSessionInput,
	ipAddress string,
	userAgent string,
) (*AssessmentSessionDetail, error) {
	if input.Year < 2000 || input.Year > 9999 || input.OrganizationID == 0 {
		return nil, ErrInvalidParam
	}
	if err := ensureAssessmentOrganizationScope(claims, input.OrganizationID); err != nil {
		return nil, err
	}

	orgName, err := s.ensureActiveOrganization(ctx, input.OrganizationID)
	if err != nil {
		return nil, err
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = fmt.Sprintf("%d年%s考核", input.Year, orgName)
	}
	assessmentName := buildAssessmentName(displayName)
	if assessmentName == "" {
		assessmentName = fmt.Sprintf("%d-%d-%d", input.Year, input.OrganizationID, time.Now().Unix())
	}

	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID)
	createdSession := model.AssessmentSession{}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		operatorRef = resolveBusinessWriteOperatorRefTx(tx, operatorID)
		dataDir, err := ensureAssessmentDataDir(assessmentName)
		if err != nil {
			return err
		}

		session := model.AssessmentSession{
			AssessmentName: assessmentName,
			DisplayName:    displayName,
			Year:           input.Year,
			OrganizationID: input.OrganizationID,
			Description:    strings.TrimSpace(input.Description),
			DataDir:        dataDir,
			CreatedBy:      operatorRef,
			UpdatedBy:      operatorRef,
		}
		if err := tx.Create(&session).Error; err != nil {
			if isUniqueConstraintError(err) {
				return ErrYearAlreadyExists
			}
			return fmt.Errorf("failed to create assessment session: %w", err)
		}

		createdSession = session
		return nil
	}); err != nil {
		return nil, err
	}

	targetID := createdSession.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(operatorRef, "create", "assessment_sessions", &targetID, map[string]any{
		"event":          "create_assessment_session",
		"assessmentName": createdSession.AssessmentName,
		"organizationId": createdSession.OrganizationID,
		"year":           createdSession.Year,
	}, ipAddress, userAgent))

	createdSummary, err := s.loadSessionSummary(ctx, createdSession.ID)
	if err != nil {
		return nil, err
	}
	if err := withSessionBusinessDB(ctx, createdSummary, func(sessionDB *gorm.DB) error {
		return sessionDB.Transaction(func(tx *gorm.DB) error {
			periods := defaultSessionPeriods(createdSession.ID, operatorRef)
			if err := tx.Create(&periods).Error; err != nil {
				return fmt.Errorf("failed to create default periods: %w", err)
			}

			groups := defaultObjectGroups(createdSession.ID, operatorRef)
			if err := tx.Create(&groups).Error; err != nil {
				return fmt.Errorf("failed to create default object groups: %w", err)
			}

			if _, err := s.generateDefaultObjectsTx(s.db.WithContext(ctx), tx, createdSession.ID, createdSession.OrganizationID, operatorRef); err != nil {
				return err
			}
			return nil
		})
	}); err != nil {
		return nil, err
	}
	if err := persistSessionDefaultObjectSnapshot(ctx, createdSummary); err != nil {
		return nil, err
	}
	if err := syncSessionBusinessDataFile(ctx, createdSummary); err != nil {
		return nil, err
	}

	return s.GetSession(ctx, claims, createdSession.ID)
}

func (s *AssessmentSessionService) UpdateSession(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	sessionID uint,
	input UpdateAssessmentSessionInput,
	ipAddress string,
	userAgent string,
) (*AssessmentSessionDetail, error) {
	if sessionID == 0 {
		return nil, ErrInvalidParam
	}
	summary, err := s.loadSessionSummary(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := ensureAssessmentOrganizationScope(claims, summary.OrganizationID); err != nil {
		return nil, err
	}

	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = summary.DisplayName
	}
	updates := map[string]any{
		"display_name": displayName,
		"description":  strings.TrimSpace(input.Description),
		"updated_at":   time.Now().Unix(),
		"updated_by":   resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID),
	}
	if err := s.db.WithContext(ctx).Model(&model.AssessmentSession{}).Where("id = ?", sessionID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update assessment session: %w", err)
	}

	targetID := sessionID
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID)
	_ = s.auditRepo.Create(ctx, buildAuditRecord(operatorRef, "update", "assessment_sessions", &targetID, map[string]any{
		"event": "update_assessment_session",
	}, ipAddress, userAgent))
	summary, err = s.loadSessionSummary(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := syncSessionBusinessDataFile(ctx, summary); err != nil {
		return nil, err
	}
	return s.GetSession(ctx, claims, sessionID)
}

func (s *AssessmentSessionService) ReplacePeriods(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	sessionID uint,
	items []SessionPeriodItem,
	ipAddress string,
	userAgent string,
) ([]model.AssessmentSessionPeriod, error) {
	if sessionID == 0 || len(items) == 0 {
		return nil, ErrInvalidParam
	}

	summary, err := s.loadSessionSummary(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := ensureAssessmentOrganizationScope(claims, summary.OrganizationID); err != nil {
		return nil, err
	}

	normalized := make([]model.AssessmentSessionPeriod, 0, len(items))
	seen := map[string]struct{}{}
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID)
	bindingMap := map[string]string{}
	for idx, item := range items {
		code := strings.ToUpper(strings.TrimSpace(item.PeriodCode))
		name := strings.TrimSpace(item.PeriodName)
		if code == "" || name == "" {
			return nil, ErrInvalidPeriodTemplate
		}
		if _, exists := seen[code]; exists {
			return nil, ErrInvalidPeriodTemplate
		}
		seen[code] = struct{}{}
		ruleBindingKey := strings.ToUpper(strings.TrimSpace(item.RuleBindingKey))
		if ruleBindingKey == "" {
			ruleBindingKey = code
		}
		bindingMap[code] = ruleBindingKey
		normalized = append(normalized, model.AssessmentSessionPeriod{
			AssessmentID:   sessionID,
			PeriodCode:     code,
			PeriodName:     name,
			RuleBindingKey: ruleBindingKey,
			SortOrder:      idx + 1,
			CreatedBy:      operatorRef,
			UpdatedBy:      operatorRef,
		})
	}
	for _, bindingKey := range bindingMap {
		if _, exists := seen[bindingKey]; !exists {
			return nil, ErrInvalidPeriodTemplate
		}
	}

	if err := withSessionBusinessDB(ctx, summary, func(sessionDB *gorm.DB) error {
		return sessionDB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("assessment_id = ?", sessionID).Delete(&model.AssessmentSessionPeriod{}).Error; err != nil {
				return fmt.Errorf("failed to delete old periods: %w", err)
			}
			if err := tx.Create(&normalized).Error; err != nil {
				return fmt.Errorf("failed to save periods: %w", err)
			}
			return nil
		})
	}); err != nil {
		return nil, err
	}

	targetID := sessionID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(operatorRef, "update", "assessment_sessions", &targetID, map[string]any{
		"event": "replace_assessment_periods",
		"count": len(normalized),
	}, ipAddress, userAgent))
	if err := syncSessionBusinessDataFile(ctx, summary); err != nil {
		return nil, err
	}
	return s.listPeriods(ctx, sessionID)
}

func (s *AssessmentSessionService) ReplaceObjectGroups(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	sessionID uint,
	items []SessionObjectGroupItem,
	ipAddress string,
	userAgent string,
) ([]model.AssessmentObjectGroup, error) {
	if sessionID == 0 || len(items) == 0 {
		return nil, ErrInvalidParam
	}
	summary, err := s.loadSessionSummary(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := ensureAssessmentOrganizationScope(claims, summary.OrganizationID); err != nil {
		return nil, err
	}

	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID)
	normalized := make([]model.AssessmentObjectGroup, 0, len(items))
	seen := map[string]struct{}{}
	for idx, item := range items {
		objectType, ok := normalizeObjectType(item.ObjectType)
		if !ok {
			return nil, ErrInvalidRuleObjectType
		}
		groupCode := strings.TrimSpace(item.GroupCode)
		groupName := strings.TrimSpace(item.GroupName)
		if groupCode == "" || groupName == "" {
			return nil, ErrInvalidParam
		}
		key := objectType + ":" + groupCode
		if _, exists := seen[key]; exists {
			return nil, ErrInvalidParam
		}
		seen[key] = struct{}{}
		normalized = append(normalized, model.AssessmentObjectGroup{
			AssessmentID: sessionID,
			ObjectType:   objectType,
			GroupCode:    groupCode,
			GroupName:    groupName,
			SortOrder:    idx + 1,
			IsSystem:     false,
			CreatedBy:    operatorRef,
			UpdatedBy:    operatorRef,
		})
	}

	if err := withSessionBusinessDB(ctx, summary, func(sessionDB *gorm.DB) error {
		return sessionDB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("assessment_id = ?", sessionID).Delete(&model.AssessmentObjectGroup{}).Error; err != nil {
				return fmt.Errorf("failed to delete old object groups: %w", err)
			}
			if err := tx.Create(&normalized).Error; err != nil {
				return fmt.Errorf("failed to save object groups: %w", err)
			}
			return nil
		})
	}); err != nil {
		return nil, err
	}

	targetID := sessionID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(operatorRef, "update", "assessment_sessions", &targetID, map[string]any{
		"event": "replace_assessment_object_groups",
		"count": len(normalized),
	}, ipAddress, userAgent))
	if err := syncSessionBusinessDataFile(ctx, summary); err != nil {
		return nil, err
	}
	return s.listObjectGroups(ctx, sessionID)
}

func (s *AssessmentSessionService) ListObjects(ctx context.Context, claims *auth.Claims, sessionID uint) ([]model.AssessmentSessionObject, error) {
	if sessionID == 0 {
		return nil, ErrInvalidParam
	}
	summary, err := s.loadSessionSummary(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := ensureAssessmentOrganizationScope(claims, summary.OrganizationID); err != nil {
		return nil, err
	}
	items := make([]model.AssessmentSessionObject, 0, 64)
	if err := withSessionBusinessDB(ctx, summary, func(sessionDB *gorm.DB) error {
		if err := sessionDB.
			Where("assessment_id = ?", sessionID).
			Order("sort_order ASC, id ASC").
			Find(&items).Error; err != nil {
			return fmt.Errorf("failed to list assessment objects: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *AssessmentSessionService) ListObjectCandidates(
	ctx context.Context,
	claims *auth.Claims,
	sessionID uint,
	keyword string,
) ([]SessionObjectCandidateItem, error) {
	if sessionID == 0 {
		return nil, ErrInvalidParam
	}
	summary, err := s.loadSessionSummary(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := ensureAssessmentOrganizationScope(claims, summary.OrganizationID); err != nil {
		return nil, err
	}

	items, err := s.listObjectCandidatesForSession(ctx, summary.OrganizationID)
	if err != nil {
		return nil, err
	}

	text := strings.ToLower(strings.TrimSpace(keyword))
	if text == "" {
		return items, nil
	}
	filtered := make([]SessionObjectCandidateItem, 0, len(items))
	for _, item := range items {
		hit := strings.Contains(strings.ToLower(item.ObjectName), text) ||
			strings.Contains(strings.ToLower(item.OrganizationName), text) ||
			strings.Contains(strings.ToLower(item.DepartmentName), text)
		if hit {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (s *AssessmentSessionService) ReplaceObjects(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	sessionID uint,
	items []SessionObjectUpsertItem,
	ipAddress string,
	userAgent string,
) ([]model.AssessmentSessionObject, error) {
	if sessionID == 0 {
		return nil, ErrInvalidParam
	}
	summary, err := s.loadSessionSummary(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := ensureAssessmentOrganizationScope(claims, summary.OrganizationID); err != nil {
		return nil, err
	}

	groupTypeMap := map[string]string{}
	groups, err := s.listObjectGroups(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		groupTypeMap[group.GroupCode] = group.ObjectType
	}

	candidates, err := s.listObjectCandidatesForSession(ctx, summary.OrganizationID)
	if err != nil {
		return nil, err
	}
	candidateMap := map[string]SessionObjectCandidateItem{}
	for _, item := range candidates {
		candidateMap[buildTargetKey(item.TargetType, item.TargetID)] = item
	}

	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID)
	normalized := make([]model.AssessmentSessionObject, 0, len(items))
	targetKeyByIndex := make([]string, 0, len(items))
	parentKeyByIndex := make([]string, 0, len(items))
	seen := map[string]struct{}{}

	for idx, item := range items {
		objectType, ok := normalizeObjectType(item.ObjectType)
		if !ok {
			return nil, ErrInvalidRuleObjectType
		}
		groupCode := strings.TrimSpace(item.GroupCode)
		groupType, exists := groupTypeMap[groupCode]
		if !exists || groupType != objectType {
			return nil, ErrInvalidRuleObjectCategory
		}
		targetType := normalizeTargetType(item.TargetType)
		if targetType == "" || item.TargetID == 0 {
			return nil, ErrInvalidParam
		}
		targetKey := buildTargetKey(targetType, item.TargetID)
		if _, dup := seen[targetKey]; dup {
			return nil, ErrInvalidParam
		}
		seen[targetKey] = struct{}{}

		candidate, ok := candidateMap[targetKey]
		if !ok {
			return nil, ErrInvalidParam
		}

		sortOrder := idx + 1
		if item.SortOrder > 0 {
			sortOrder = item.SortOrder
		}

		parentKey := ""
		parentTargetType := normalizeTargetType(item.ParentTargetType)
		if parentTargetType != "" && item.ParentTargetID > 0 {
			parentKey = buildTargetKey(parentTargetType, item.ParentTargetID)
		} else if targetType == "employee" {
			if candidate.DepartmentID != nil && candidate.OrganizationID == summary.OrganizationID {
				parentKey = buildTargetKey("department", *candidate.DepartmentID)
			} else if candidate.OrganizationID != 0 && candidate.OrganizationID != summary.OrganizationID {
				parentKey = buildTargetKey("organization", candidate.OrganizationID)
			}
		}

		normalized = append(normalized, model.AssessmentSessionObject{
			AssessmentID: sessionID,
			ObjectType:   objectType,
			GroupCode:    groupCode,
			TargetID:     item.TargetID,
			TargetType:   targetType,
			ObjectName:   candidate.ObjectName,
			SortOrder:    sortOrder,
			IsActive:     item.IsActive,
			CreatedBy:    operatorRef,
			UpdatedBy:    operatorRef,
		})
		targetKeyByIndex = append(targetKeyByIndex, targetKey)
		parentKeyByIndex = append(parentKeyByIndex, parentKey)
	}

	if err := withSessionBusinessDB(ctx, summary, func(sessionDB *gorm.DB) error {
		return sessionDB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("assessment_id = ?", sessionID).Delete(&model.AssessmentSessionObject{}).Error; err != nil {
				return fmt.Errorf("failed to clear assessment objects: %w", err)
			}
			if len(normalized) == 0 {
				return nil
			}

			idByTargetKey := map[string]uint{}
			rowIDs := make([]uint, len(normalized))
			for idx := range normalized {
				normalized[idx].ParentObjectID = nil
				if err := tx.Create(&normalized[idx]).Error; err != nil {
					return fmt.Errorf("failed to create assessment object: %w", err)
				}
				rowIDs[idx] = normalized[idx].ID
				idByTargetKey[targetKeyByIndex[idx]] = normalized[idx].ID
			}
			for idx, parentKey := range parentKeyByIndex {
				if parentKey == "" {
					continue
				}
				parentID, ok := idByTargetKey[parentKey]
				if !ok || parentID == rowIDs[idx] {
					continue
				}
				if err := tx.Model(&model.AssessmentSessionObject{}).
					Where("id = ?", rowIDs[idx]).
					Update("parent_object_id", parentID).Error; err != nil {
					return fmt.Errorf("failed to set parent object: %w", err)
				}
			}
			return nil
		})
	}); err != nil {
		return nil, err
	}

	targetID := sessionID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(operatorRef, "update", "assessment_sessions", &targetID, map[string]any{
		"event": "replace_assessment_objects",
		"count": len(normalized),
	}, ipAddress, userAgent))
	if err := syncSessionBusinessDataFile(ctx, summary); err != nil {
		return nil, err
	}
	return s.ListObjects(ctx, claims, sessionID)
}

func (s *AssessmentSessionService) ResetObjectsToDefault(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	sessionID uint,
	ipAddress string,
	userAgent string,
) ([]model.AssessmentSessionObject, error) {
	summary, err := s.loadSessionSummary(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := ensureAssessmentOrganizationScope(claims, summary.OrganizationID); err != nil {
		return nil, err
	}

	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID)
	snapshotItems, hasSnapshot, err := loadSessionDefaultObjectSnapshot(summary)
	if err != nil {
		return nil, fmt.Errorf("failed to load default object snapshot: %w", err)
	}
	if err := withSessionBusinessDB(ctx, summary, func(sessionDB *gorm.DB) error {
		return sessionDB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("assessment_id = ?", sessionID).Delete(&model.AssessmentSessionObject{}).Error; err != nil {
				return fmt.Errorf("failed to clear session objects: %w", err)
			}
			if hasSnapshot {
				return restoreSessionObjectsFromSnapshotTx(tx, sessionID, snapshotItems, operatorRef)
			}
			_, restoreErr := s.generateDefaultObjectsTx(s.db.WithContext(ctx), tx, sessionID, summary.OrganizationID, operatorRef)
			return restoreErr
		})
	}); err != nil {
		return nil, err
	}

	targetID := sessionID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(operatorRef, "update", "assessment_sessions", &targetID, map[string]any{
		"event": "reset_assessment_objects_default",
	}, ipAddress, userAgent))
	if !hasSnapshot {
		if persistErr := persistSessionDefaultObjectSnapshot(ctx, summary); persistErr != nil {
			return nil, persistErr
		}
	}
	if err := syncSessionBusinessDataFile(ctx, summary); err != nil {
		return nil, err
	}
	return s.ListObjects(ctx, claims, sessionID)
}

func (s *AssessmentSessionService) loadSessionSummary(ctx context.Context, sessionID uint) (*AssessmentSessionSummary, error) {
	item := &AssessmentSessionSummary{}
	if err := s.db.WithContext(ctx).
		Table("assessment_sessions AS a").
		Select("a.*, o.org_name AS organization_name").
		Joins("JOIN organizations o ON o.id = a.organization_id").
		Where("a.id = ?", sessionID).
		First(item).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrYearNotFound
		}
		return nil, fmt.Errorf("failed to query assessment session: %w", err)
	}
	return item, nil
}

func (s *AssessmentSessionService) listPeriods(ctx context.Context, sessionID uint) ([]model.AssessmentSessionPeriod, error) {
	summary, err := s.loadSessionSummary(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	items := make([]model.AssessmentSessionPeriod, 0, 8)
	if err := withSessionBusinessDB(ctx, summary, func(sessionDB *gorm.DB) error {
		if err := sessionDB.
			Where("assessment_id = ?", sessionID).
			Order("sort_order ASC, id ASC").
			Find(&items).Error; err != nil {
			return fmt.Errorf("failed to list periods: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *AssessmentSessionService) listObjectGroups(ctx context.Context, sessionID uint) ([]model.AssessmentObjectGroup, error) {
	summary, err := s.loadSessionSummary(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	items := make([]model.AssessmentObjectGroup, 0, 16)
	if err := withSessionBusinessDB(ctx, summary, func(sessionDB *gorm.DB) error {
		if err := sessionDB.
			Where("assessment_id = ?", sessionID).
			Order("sort_order ASC, id ASC").
			Find(&items).Error; err != nil {
			return fmt.Errorf("failed to list object groups: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *AssessmentSessionService) ensureActiveOrganization(ctx context.Context, organizationID uint) (string, error) {
	var row struct {
		OrgName string
	}
	if err := s.db.WithContext(ctx).
		Table("organizations").
		Select("org_name").
		Where("id = ? AND deleted_at IS NULL AND status = 'active'", organizationID).
		First(&row).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return "", ErrOrganizationNotFound
		}
		return "", fmt.Errorf("failed to verify organization: %w", err)
	}
	return row.OrgName, nil
}

func defaultSessionPeriods(sessionID uint, operatorID *uint) []model.AssessmentSessionPeriod {
	base := []SessionPeriodItem{
		{PeriodCode: "Q1", PeriodName: "第一季度", SortOrder: 1},
		{PeriodCode: "Q2", PeriodName: "第二季度", SortOrder: 2},
		{PeriodCode: "Q3", PeriodName: "第三季度", SortOrder: 3},
		{PeriodCode: "Q4", PeriodName: "第四季度", SortOrder: 4},
		{PeriodCode: "YEAR_END", PeriodName: "年终", SortOrder: 5},
	}
	result := make([]model.AssessmentSessionPeriod, 0, len(base))
	for _, item := range base {
		ruleBindingKey := item.PeriodCode
		switch item.PeriodCode {
		case "Q2", "Q3", "Q4":
			ruleBindingKey = "Q1"
		}
		result = append(result, model.AssessmentSessionPeriod{
			AssessmentID:   sessionID,
			PeriodCode:     item.PeriodCode,
			PeriodName:     item.PeriodName,
			RuleBindingKey: ruleBindingKey,
			SortOrder:      item.SortOrder,
			CreatedBy:      operatorID,
			UpdatedBy:      operatorID,
		})
	}
	return result
}

func defaultObjectGroups(sessionID uint, operatorID *uint) []model.AssessmentObjectGroup {
	base := []SessionObjectGroupItem{
		{ObjectType: ObjectTypeTeam, GroupCode: "dept", GroupName: "部门", SortOrder: 1},
		{ObjectType: ObjectTypeTeam, GroupCode: "child_org", GroupName: "次级企业", SortOrder: 2},
		{ObjectType: ObjectTypeIndividual, GroupCode: "dept_main", GroupName: "部门正职", SortOrder: 101},
		{ObjectType: ObjectTypeIndividual, GroupCode: "dept_deputy", GroupName: "部门副职", SortOrder: 102},
		{ObjectType: ObjectTypeIndividual, GroupCode: "general_staff", GroupName: "一般人员", SortOrder: 103},
		{ObjectType: ObjectTypeIndividual, GroupCode: "child_leadership_main", GroupName: "次级领导班子正职", SortOrder: 104},
		{ObjectType: ObjectTypeIndividual, GroupCode: "child_leadership_deputy", GroupName: "次级领导班子副职", SortOrder: 105},
	}
	result := make([]model.AssessmentObjectGroup, 0, len(base))
	for _, item := range base {
		result = append(result, model.AssessmentObjectGroup{
			AssessmentID: sessionID,
			ObjectType:   item.ObjectType,
			GroupCode:    item.GroupCode,
			GroupName:    item.GroupName,
			SortOrder:    item.SortOrder,
			IsSystem:     true,
			CreatedBy:    operatorID,
			UpdatedBy:    operatorID,
		})
	}
	return result
}

func (s *AssessmentSessionService) generateDefaultObjectsTx(
	sourceDB *gorm.DB,
	targetTx *gorm.DB,
	sessionID uint,
	organizationID uint,
	operatorID *uint,
) (int, error) {
	count := 0
	deptObjectIDByDept := map[uint]uint{}
	childOrgObjectID := map[uint]uint{}

	var departments []struct {
		ID       uint
		DeptName string
	}
	if err := sourceDB.WithContext(context.Background()).
		Table("departments").
		Select("id, dept_name").
		Where("organization_id = ? AND deleted_at IS NULL AND status = 'active'", organizationID).
		Order("sort_order ASC, id ASC").
		Scan(&departments).Error; err != nil {
		return 0, fmt.Errorf("failed to list departments for default objects: %w", err)
	}
	for idx, dept := range departments {
		row := model.AssessmentSessionObject{
			AssessmentID: sessionID,
			ObjectType:   ObjectTypeTeam,
			GroupCode:    "dept",
			TargetID:     dept.ID,
			TargetType:   "department",
			ObjectName:   dept.DeptName,
			SortOrder:    idx + 1,
			IsActive:     true,
			CreatedBy:    operatorID,
			UpdatedBy:    operatorID,
		}
		if err := targetTx.Create(&row).Error; err != nil {
			return 0, fmt.Errorf("failed to create department object: %w", err)
		}
		deptObjectIDByDept[dept.ID] = row.ID
		count++
	}

	var employees []struct {
		ID           uint
		EmpName      string
		DepartmentID *uint
		LevelCode    string
	}
	if err := sourceDB.WithContext(context.Background()).
		Table("employees e").
		Select("e.id, e.emp_name, e.department_id, p.level_code").
		Joins("JOIN position_levels p ON p.id = e.position_level_id").
		Where("e.organization_id = ? AND e.deleted_at IS NULL AND e.status = 'active'", organizationID).
		Order("e.id ASC").
		Scan(&employees).Error; err != nil {
		return 0, fmt.Errorf("failed to list employees for default objects: %w", err)
	}
	for idx, employee := range employees {
		groupCode := "general_staff"
		levelCode := strings.ToLower(strings.TrimSpace(employee.LevelCode))
		switch levelCode {
		case "department_main":
			groupCode = "dept_main"
		case "department_deputy":
			groupCode = "dept_deputy"
		}

		var parentID *uint
		if employee.DepartmentID != nil {
			if objectID, ok := deptObjectIDByDept[*employee.DepartmentID]; ok {
				parentID = uintPtr(objectID)
			}
		}
		row := model.AssessmentSessionObject{
			AssessmentID:   sessionID,
			ObjectType:     ObjectTypeIndividual,
			GroupCode:      groupCode,
			TargetID:       employee.ID,
			TargetType:     "employee",
			ObjectName:     employee.EmpName,
			ParentObjectID: parentID,
			SortOrder:      idx + 1,
			IsActive:       true,
			CreatedBy:      operatorID,
			UpdatedBy:      operatorID,
		}
		if err := targetTx.Create(&row).Error; err != nil {
			return 0, fmt.Errorf("failed to create employee object: %w", err)
		}
		count++
	}

	var childOrgs []struct {
		ID      uint
		OrgName string
	}
	if err := sourceDB.WithContext(context.Background()).
		Table("organizations").
		Select("id, org_name").
		Where("parent_id = ? AND deleted_at IS NULL AND status = 'active'", organizationID).
		Order("sort_order ASC, id ASC").
		Scan(&childOrgs).Error; err != nil {
		return 0, fmt.Errorf("failed to list child organizations: %w", err)
	}
	for idx, org := range childOrgs {
		row := model.AssessmentSessionObject{
			AssessmentID: sessionID,
			ObjectType:   ObjectTypeTeam,
			GroupCode:    "child_org",
			TargetID:     org.ID,
			TargetType:   "organization",
			ObjectName:   org.OrgName,
			SortOrder:    idx + 1,
			IsActive:     true,
			CreatedBy:    operatorID,
			UpdatedBy:    operatorID,
		}
		if err := targetTx.Create(&row).Error; err != nil {
			return 0, fmt.Errorf("failed to create child organization object: %w", err)
		}
		childOrgObjectID[org.ID] = row.ID
		count++
	}

	var childLeaders []struct {
		ID             uint
		EmpName        string
		OrganizationID uint
		LevelCode      string
	}
	if len(childOrgs) > 0 {
		childOrgIDs := make([]uint, 0, len(childOrgs))
		for _, org := range childOrgs {
			childOrgIDs = append(childOrgIDs, org.ID)
		}
		if err := sourceDB.WithContext(context.Background()).
			Table("employees e").
			Select("e.id, e.emp_name, e.organization_id, p.level_code").
			Joins("JOIN position_levels p ON p.id = e.position_level_id").
			Where("e.organization_id IN ? AND e.deleted_at IS NULL AND e.status = 'active'", childOrgIDs).
			Where("LOWER(p.level_code) IN ?", []string{"leadership_main", "leadership_deputy"}).
			Order("e.id ASC").
			Scan(&childLeaders).Error; err != nil {
			return 0, fmt.Errorf("failed to list child leadership members: %w", err)
		}
	}

	for idx, member := range childLeaders {
		groupCode := "child_leadership_deputy"
		if strings.EqualFold(member.LevelCode, "leadership_main") {
			groupCode = "child_leadership_main"
		}
		var parentID *uint
		if objectID, ok := childOrgObjectID[member.OrganizationID]; ok {
			parentID = uintPtr(objectID)
		}
		row := model.AssessmentSessionObject{
			AssessmentID:   sessionID,
			ObjectType:     ObjectTypeIndividual,
			GroupCode:      groupCode,
			TargetID:       member.ID,
			TargetType:     "employee",
			ObjectName:     member.EmpName,
			ParentObjectID: parentID,
			SortOrder:      idx + 1,
			IsActive:       true,
			CreatedBy:      operatorID,
			UpdatedBy:      operatorID,
		}
		if err := targetTx.Create(&row).Error; err != nil {
			return 0, fmt.Errorf("failed to create child leadership member object: %w", err)
		}
		count++
	}

	return count, nil
}

func restoreSessionObjectsFromSnapshotTx(
	tx *gorm.DB,
	sessionID uint,
	items []sessionDefaultObjectSnapshotItem,
	operatorID *uint,
) error {
	if len(items) == 0 {
		return nil
	}

	normalized := make([]model.AssessmentSessionObject, 0, len(items))
	targetKeys := make([]string, 0, len(items))
	parentKeys := make([]string, 0, len(items))
	seenTargets := map[string]struct{}{}

	for _, item := range items {
		objectType, ok := normalizeObjectType(item.ObjectType)
		if !ok {
			return ErrInvalidRuleObjectType
		}
		groupCode := strings.TrimSpace(item.GroupCode)
		targetType := normalizeTargetType(item.TargetType)
		targetID := item.TargetID
		if groupCode == "" || targetType == "" || targetID == 0 {
			return ErrInvalidParam
		}
		targetKey := buildTargetKey(targetType, targetID)
		if _, exists := seenTargets[targetKey]; exists {
			return ErrInvalidParam
		}
		seenTargets[targetKey] = struct{}{}

		objectName := strings.TrimSpace(item.ObjectName)
		if objectName == "" {
			objectName = targetType + "-" + strconv.FormatUint(uint64(targetID), 10)
		}
		parentKey := ""
		parentType := normalizeTargetType(item.ParentTargetType)
		if parentType != "" && item.ParentTargetID > 0 {
			parentKey = buildTargetKey(parentType, item.ParentTargetID)
		}
		sortOrder := item.SortOrder
		if sortOrder <= 0 {
			sortOrder = len(normalized) + 1
		}
		normalized = append(normalized, model.AssessmentSessionObject{
			AssessmentID: sessionID,
			ObjectType:   objectType,
			GroupCode:    groupCode,
			TargetID:     targetID,
			TargetType:   targetType,
			ObjectName:   objectName,
			SortOrder:    sortOrder,
			IsActive:     item.IsActive,
			CreatedBy:    operatorID,
			UpdatedBy:    operatorID,
		})
		targetKeys = append(targetKeys, targetKey)
		parentKeys = append(parentKeys, parentKey)
	}

	idByTarget := make(map[string]uint, len(normalized))
	rowIDs := make([]uint, len(normalized))
	for idx := range normalized {
		normalized[idx].ParentObjectID = nil
		if err := tx.Create(&normalized[idx]).Error; err != nil {
			return fmt.Errorf("failed to restore assessment object row: %w", err)
		}
		rowIDs[idx] = normalized[idx].ID
		idByTarget[targetKeys[idx]] = normalized[idx].ID
	}

	for idx, parentKey := range parentKeys {
		if parentKey == "" {
			continue
		}
		parentID, ok := idByTarget[parentKey]
		if !ok || parentID == rowIDs[idx] {
			continue
		}
		if err := tx.Model(&model.AssessmentSessionObject{}).
			Where("id = ?", rowIDs[idx]).
			Update("parent_object_id", parentID).Error; err != nil {
			return fmt.Errorf("failed to restore assessment object parent: %w", err)
		}
	}
	return nil
}

func (s *AssessmentSessionService) listObjectCandidatesForSession(
	ctx context.Context,
	organizationID uint,
) ([]SessionObjectCandidateItem, error) {
	currentOrgName, err := s.ensureActiveOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}

	var departments []struct {
		ID       uint
		DeptName string
	}
	if err := s.db.WithContext(ctx).
		Table("departments").
		Select("id, dept_name").
		Where("organization_id = ? AND deleted_at IS NULL AND status = 'active'", organizationID).
		Order("sort_order ASC, id ASC").
		Scan(&departments).Error; err != nil {
		return nil, fmt.Errorf("failed to list candidate departments: %w", err)
	}

	var childOrgs []struct {
		ID      uint
		OrgName string
	}
	if err := s.db.WithContext(ctx).
		Table("organizations").
		Select("id, org_name").
		Where("parent_id = ? AND deleted_at IS NULL AND status = 'active'", organizationID).
		Order("sort_order ASC, id ASC").
		Scan(&childOrgs).Error; err != nil {
		return nil, fmt.Errorf("failed to list candidate child organizations: %w", err)
	}

	childOrgNameByID := make(map[uint]string, len(childOrgs))
	childOrgIDs := make([]uint, 0, len(childOrgs))
	for _, item := range childOrgs {
		childOrgNameByID[item.ID] = item.OrgName
		childOrgIDs = append(childOrgIDs, item.ID)
	}

	type employeeRow struct {
		ID             uint
		EmpName        string
		OrganizationID uint
		DepartmentID   *uint
		DeptName       string
		LevelCode      string
	}
	var ownEmployees []employeeRow
	if err := s.db.WithContext(ctx).
		Table("employees e").
		Select("e.id, e.emp_name, e.organization_id, e.department_id, COALESCE(d.dept_name, '') AS dept_name, p.level_code").
		Joins("JOIN position_levels p ON p.id = e.position_level_id").
		Joins("LEFT JOIN departments d ON d.id = e.department_id").
		Where("e.organization_id = ? AND e.deleted_at IS NULL AND e.status = 'active'", organizationID).
		Order("e.id ASC").
		Scan(&ownEmployees).Error; err != nil {
		return nil, fmt.Errorf("failed to list candidate employees: %w", err)
	}

	childLeaders := make([]employeeRow, 0)
	if len(childOrgIDs) > 0 {
		if err := s.db.WithContext(ctx).
			Table("employees e").
			Select("e.id, e.emp_name, e.organization_id, e.department_id, COALESCE(d.dept_name, '') AS dept_name, p.level_code").
			Joins("JOIN position_levels p ON p.id = e.position_level_id").
			Joins("LEFT JOIN departments d ON d.id = e.department_id").
			Where("e.organization_id IN ? AND e.deleted_at IS NULL AND e.status = 'active'", childOrgIDs).
			Where("LOWER(p.level_code) IN ?", []string{"leadership_main", "leadership_deputy"}).
			Order("e.id ASC").
			Scan(&childLeaders).Error; err != nil {
			return nil, fmt.Errorf("failed to list candidate child leadership members: %w", err)
		}
	}

	result := make([]SessionObjectCandidateItem, 0, len(departments)+len(childOrgs)+len(ownEmployees)+len(childLeaders))
	for _, dept := range departments {
		result = append(result, SessionObjectCandidateItem{
			TargetType:            "department",
			TargetID:              dept.ID,
			ObjectName:            dept.DeptName,
			OrganizationID:        organizationID,
			OrganizationName:      currentOrgName,
			RecommendedObjectType: ObjectTypeTeam,
			RecommendedGroupCode:  "dept",
		})
	}
	for _, org := range childOrgs {
		result = append(result, SessionObjectCandidateItem{
			TargetType:            "organization",
			TargetID:              org.ID,
			ObjectName:            org.OrgName,
			OrganizationID:        org.ID,
			OrganizationName:      org.OrgName,
			RecommendedObjectType: ObjectTypeTeam,
			RecommendedGroupCode:  "child_org",
		})
	}
	for _, employee := range ownEmployees {
		recommendGroup := "general_staff"
		switch strings.ToLower(strings.TrimSpace(employee.LevelCode)) {
		case "department_main":
			recommendGroup = "dept_main"
		case "department_deputy":
			recommendGroup = "dept_deputy"
		}
		result = append(result, SessionObjectCandidateItem{
			TargetType:            "employee",
			TargetID:              employee.ID,
			ObjectName:            employee.EmpName,
			OrganizationID:        organizationID,
			OrganizationName:      currentOrgName,
			DepartmentID:          employee.DepartmentID,
			DepartmentName:        employee.DeptName,
			RecommendedObjectType: ObjectTypeIndividual,
			RecommendedGroupCode:  recommendGroup,
		})
	}
	for _, employee := range childLeaders {
		recommendGroup := "child_leadership_deputy"
		if strings.EqualFold(employee.LevelCode, "leadership_main") {
			recommendGroup = "child_leadership_main"
		}
		result = append(result, SessionObjectCandidateItem{
			TargetType:            "employee",
			TargetID:              employee.ID,
			ObjectName:            employee.EmpName,
			OrganizationID:        employee.OrganizationID,
			OrganizationName:      childOrgNameByID[employee.OrganizationID],
			DepartmentID:          employee.DepartmentID,
			DepartmentName:        employee.DeptName,
			RecommendedObjectType: ObjectTypeIndividual,
			RecommendedGroupCode:  recommendGroup,
		})
	}

	return result, nil
}

func normalizeTargetType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "organization", "department", "employee":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func buildTargetKey(targetType string, targetID uint) string {
	return targetType + ":" + strconv.FormatUint(uint64(targetID), 10)
}

func buildAssessmentName(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	normalized := strings.ReplaceAll(trimmed, "/", "_")
	normalized = strings.ReplaceAll(normalized, "\\", "_")
	normalized = strings.ReplaceAll(normalized, ":", "_")
	normalized = strings.ReplaceAll(normalized, "*", "_")
	normalized = strings.ReplaceAll(normalized, "?", "_")
	normalized = strings.ReplaceAll(normalized, "\"", "_")
	normalized = strings.ReplaceAll(normalized, "<", "_")
	normalized = strings.ReplaceAll(normalized, ">", "_")
	normalized = strings.ReplaceAll(normalized, "|", "_")
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return ""
	}
	return normalized
}

func ensureAssessmentDataDir(assessmentName string) (string, error) {
	root := strings.TrimSpace(os.Getenv("ASSESS_DATA_ROOT"))
	if root == "" {
		root = "data"
	}
	assessmentDir := filepath.Join(root, assessmentName)
	if err := os.MkdirAll(assessmentDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create assessment data directory: %w", err)
	}
	return assessmentDir, nil
}

func buildDefaultRuleTemplateJSON() string {
	payload := map[string]any{
		"version": 3,
		"scopedRules": []map[string]any{
			{
				"id":                     "default_rule",
				"applicablePeriods":      []string{},
				"applicableObjectGroups": []string{},
				"scoreModules": []map[string]any{
					{
						"id":                "base_performance",
						"moduleKey":         "base_performance",
						"moduleName":        "Base Performance",
						"weight":            100,
						"calculationMethod": "direct_input",
						"customScript":      "",
					},
				},
				"grades": []map[string]any{
					{
						"id":    "grade_a",
						"title": "A",
						"scoreNode": map[string]any{
							"hasUpperLimit": true,
							"upperScore":    100,
							"upperOperator": "<=",
							"hasLowerLimit": true,
							"lowerScore":    90,
							"lowerOperator": ">=",
						},
						"extraConditionScript": "",
						"conditionLogic":       "and",
						"maxRatioPercent":      nil,
					},
					{
						"id":    "grade_b",
						"title": "B",
						"scoreNode": map[string]any{
							"hasUpperLimit": true,
							"upperScore":    89.99,
							"upperOperator": "<=",
							"hasLowerLimit": true,
							"lowerScore":    80,
							"lowerOperator": ">=",
						},
						"extraConditionScript": "",
						"conditionLogic":       "and",
						"maxRatioPercent":      nil,
					},
					{
						"id":    "grade_c",
						"title": "C",
						"scoreNode": map[string]any{
							"hasUpperLimit": true,
							"upperScore":    79.99,
							"upperOperator": "<=",
							"hasLowerLimit": false,
							"lowerScore":    nil,
							"lowerOperator": ">=",
						},
						"extraConditionScript": "",
						"conditionLogic":       "and",
						"maxRatioPercent":      nil,
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}

func ensureRuleFilePath(assessmentName string, fileName string) (string, error) {
	root := strings.TrimSpace(os.Getenv("ASSESS_DATA_ROOT"))
	if root == "" {
		root = "data"
	}
	assessmentDir := filepath.Join(root, assessmentName)
	if err := os.MkdirAll(assessmentDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(assessmentDir, fileName), nil
}

func buildRuleFileName(ruleName string) string {
	base := buildAssessmentName(ruleName)
	if base == "" {
		base = "rule"
	}
	return base + "_" + strconv.FormatInt(time.Now().UnixNano(), 10) + ".json"
}
