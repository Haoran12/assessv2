package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

type RuleManagementService struct {
	db        *gorm.DB
	auditRepo *repository.AuditRepository
}

type RuleFileListFilter struct {
	AssessmentID  uint
	IncludeHidden bool
}

type RuleFileSummary struct {
	model.RuleFile
	HiddenByCurrentOrg bool `json:"hiddenByCurrentOrg"`
	CanEdit            bool `json:"canEdit"`
	CanDelete          bool `json:"canDelete"`
}

type RuleFileInput struct {
	AssessmentID uint
	RuleName     string
	Description  string
	ContentJSON  string
}

func NewRuleManagementService(db *gorm.DB, auditRepo *repository.AuditRepository) *RuleManagementService {
	return &RuleManagementService{db: db, auditRepo: auditRepo}
}

func sessionRuleDefaultName(session *AssessmentSessionSummary) string {
	name := strings.TrimSpace(session.DisplayName)
	if name == "" {
		name = strings.TrimSpace(session.AssessmentName)
	}
	if name == "" {
		return "场次规则"
	}
	return name + "-规则"
}

func (s *RuleManagementService) ensureSessionRuleFile(
	ctx context.Context,
	session *AssessmentSessionSummary,
	operatorRef *uint,
) (*model.RuleFile, error) {
	items := make([]model.RuleFile, 0, 8)
	if err := withSessionBusinessDB(ctx, session, func(sessionDB *gorm.DB) error {
		if err := sessionDB.
			Where("assessment_id = ?", session.ID).
			Order("updated_at DESC, id DESC").
			Find(&items).Error; err != nil {
			return fmt.Errorf("failed to query rule files: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if len(items) == 0 {
		ruleName := sessionRuleDefaultName(session)
		contentJSON := buildDefaultRuleTemplateJSON()
		fileName := buildRuleFileName(ruleName)
		filePath, err := ensureRuleFilePath(session.AssessmentName, fileName)
		if err != nil {
			return nil, err
		}
		if writeErr := os.WriteFile(filePath, []byte(contentJSON), 0o644); writeErr != nil {
			return nil, fmt.Errorf("failed to write rule file content: %w", writeErr)
		}
		record := model.RuleFile{
			AssessmentID: session.ID,
			RuleName:     ruleName,
			Description:  "场次专属规则文件",
			ContentJSON:  contentJSON,
			FilePath:     filePath,
			IsCopy:       false,
			CreatedBy:    operatorRef,
			UpdatedBy:    operatorRef,
		}
		if err := withSessionBusinessDB(ctx, session, func(sessionDB *gorm.DB) error {
			if err := sessionDB.Create(&record).Error; err != nil {
				return fmt.Errorf("failed to create session rule file: %w", err)
			}
			return nil
		}); err != nil {
			return nil, err
		}
		if err := syncSessionBusinessDataFile(ctx, session); err != nil {
			return nil, err
		}
		return &record, nil
	}

	var picked *model.RuleFile
	for i := range items {
		if !items[i].IsCopy {
			picked = &items[i]
			break
		}
	}
	if picked == nil {
		picked = &items[0]
	}

	if picked.IsCopy || picked.SourceRuleID != nil || picked.OwnerOrgID != nil {
		updates := map[string]any{
			"is_copy":        false,
			"source_rule_id": nil,
			"owner_org_id":   nil,
		}
		if operatorRef != nil {
			updates["updated_by"] = operatorRef
			updates["updated_at"] = time.Now().Unix()
		}
		if err := withSessionBusinessDB(ctx, session, func(sessionDB *gorm.DB) error {
			if err := sessionDB.Model(&model.RuleFile{}).Where("id = ?", picked.ID).Updates(updates).Error; err != nil {
				return fmt.Errorf("failed to normalize session rule file: %w", err)
			}
			if err := sessionDB.Where("id = ?", picked.ID).First(picked).Error; err != nil {
				return fmt.Errorf("failed to reload session rule file: %w", err)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return picked, nil
}

func (s *RuleManagementService) ListRuleFiles(ctx context.Context, claims *auth.Claims, filter RuleFileListFilter) ([]RuleFileSummary, error) {
	if filter.AssessmentID == 0 {
		return nil, ErrInvalidParam
	}
	session, err := s.loadSessionSummary(ctx, filter.AssessmentID)
	if err != nil {
		return nil, err
	}
	if err := ensureAssessmentOrganizationScope(claims, session.OrganizationID); err != nil {
		return nil, err
	}

	record, err := s.ensureSessionRuleFile(ctx, session, nil)
	if err != nil {
		return nil, err
	}
	return []RuleFileSummary{
		{
			RuleFile:           *record,
			HiddenByCurrentOrg: false,
			CanEdit:            true,
			CanDelete:          false,
		},
	}, nil
}

func (s *RuleManagementService) CreateRuleFile(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	input RuleFileInput,
	ipAddress string,
	userAgent string,
) (*RuleFileSummary, error) {
	if input.AssessmentID == 0 {
		return nil, ErrInvalidParam
	}
	session, err := s.loadSessionSummary(ctx, input.AssessmentID)
	if err != nil {
		return nil, err
	}
	if err := ensureAssessmentOrganizationScope(claims, session.OrganizationID); err != nil {
		return nil, err
	}
	ruleName := strings.TrimSpace(input.RuleName)
	if ruleName == "" {
		ruleName = sessionRuleDefaultName(session)
	}
	contentJSON := strings.TrimSpace(input.ContentJSON)
	if contentJSON == "" {
		contentJSON = buildDefaultRuleTemplateJSON()
	}
	contentJSON, err = s.normalizeRuleContentByPeriodBindings(ctx, session.ID, contentJSON)
	if err != nil {
		return nil, err
	}
	if err := validateRuleExpressions(contentJSON); err != nil {
		return nil, err
	}

	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID)
	record, err := s.ensureSessionRuleFile(ctx, session, operatorRef)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{
		"rule_name":    ruleName,
		"description":  strings.TrimSpace(input.Description),
		"content_json": contentJSON,
		"updated_by":   operatorRef,
		"updated_at":   time.Now().Unix(),
	}
	if err := withSessionBusinessDB(ctx, session, func(sessionDB *gorm.DB) error {
		if err := sessionDB.Model(&model.RuleFile{}).Where("id = ?", record.ID).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update session rule file metadata: %w", err)
		}
		if writeErr := os.WriteFile(record.FilePath, []byte(contentJSON), 0o644); writeErr != nil {
			return fmt.Errorf("failed to write rule file content: %w", writeErr)
		}
		if err := sessionDB.Where("id = ?", record.ID).First(record).Error; err != nil {
			return fmt.Errorf("failed to reload rule file: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(operatorRef, "create", "rule_files", &targetID, map[string]any{
		"event":        "upsert_session_rule_file",
		"assessmentId": input.AssessmentID,
		"ruleName":     ruleName,
	}, ipAddress, userAgent))
	if err := syncSessionBusinessDataFile(ctx, session); err != nil {
		return nil, err
	}

	return &RuleFileSummary{
		RuleFile:           *record,
		HiddenByCurrentOrg: false,
		CanEdit:            true,
		CanDelete:          false,
	}, nil
}

func (s *RuleManagementService) UpdateRuleFile(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	ruleID uint,
	input RuleFileInput,
	ipAddress string,
	userAgent string,
) (*RuleFileSummary, error) {
	if ruleID == 0 {
		return nil, ErrInvalidParam
	}
	if input.AssessmentID == 0 {
		return nil, ErrInvalidParam
	}

	session, err := s.loadSessionSummary(ctx, input.AssessmentID)
	if err != nil {
		return nil, err
	}
	if err := ensureAssessmentOrganizationScope(claims, session.OrganizationID); err != nil {
		return nil, err
	}

	var record model.RuleFile
	if err := withSessionBusinessDB(ctx, session, func(sessionDB *gorm.DB) error {
		if err := sessionDB.Where("id = ?", ruleID).First(&record).Error; err != nil {
			if repository.IsRecordNotFound(err) {
				return ErrRuleNotFound
			}
			return fmt.Errorf("failed to query rule file: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	contentJSON := strings.TrimSpace(input.ContentJSON)
	if contentJSON == "" {
		contentJSON = record.ContentJSON
	}
	contentJSON, err = s.normalizeRuleContentByPeriodBindings(ctx, session.ID, contentJSON)
	if err != nil {
		return nil, err
	}
	if err := validateRuleExpressions(contentJSON); err != nil {
		return nil, err
	}
	ruleName := strings.TrimSpace(input.RuleName)
	if ruleName == "" {
		ruleName = record.RuleName
	}

	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID)
	updates := map[string]any{
		"rule_name":    ruleName,
		"description":  strings.TrimSpace(input.Description),
		"content_json": contentJSON,
		"updated_by":   operatorRef,
		"updated_at":   time.Now().Unix(),
	}
	if err := withSessionBusinessDB(ctx, session, func(sessionDB *gorm.DB) error {
		if err := sessionDB.Model(&model.RuleFile{}).Where("id = ?", ruleID).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update rule file metadata: %w", err)
		}
		if writeErr := os.WriteFile(record.FilePath, []byte(contentJSON), 0o644); writeErr != nil {
			return fmt.Errorf("failed to update rule file: %w", writeErr)
		}

		if err := sessionDB.Where("id = ?", ruleID).First(&record).Error; err != nil {
			return fmt.Errorf("failed to reload rule file: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(operatorRef, "update", "rule_files", &targetID, map[string]any{
		"event": "update_rule_file",
	}, ipAddress, userAgent))
	if err := syncSessionBusinessDataFile(ctx, session); err != nil {
		return nil, err
	}

	return &RuleFileSummary{
		RuleFile:           record,
		HiddenByCurrentOrg: false,
		CanEdit:            true,
		CanDelete:          false,
	}, nil
}

func (s *RuleManagementService) loadSessionSummary(ctx context.Context, sessionID uint) (*AssessmentSessionSummary, error) {
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

func (s *RuleManagementService) findRuleFileAcrossSessions(
	ctx context.Context,
	ruleID uint,
) (*AssessmentSessionSummary, *model.RuleFile, error) {
	sessions := make([]AssessmentSessionSummary, 0, 16)
	if err := s.db.WithContext(ctx).
		Table("assessment_sessions AS a").
		Select("a.*, o.org_name AS organization_name").
		Joins("JOIN organizations o ON o.id = a.organization_id").
		Order("a.id ASC").
		Scan(&sessions).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to list sessions for rule lookup: %w", err)
	}

	var (
		matchedSession *AssessmentSessionSummary
		matchedRecord  *model.RuleFile
	)
	for index := range sessions {
		summary := sessions[index]
		record := model.RuleFile{}
		err := withSessionBusinessDB(ctx, &summary, func(sessionDB *gorm.DB) error {
			if err := sessionDB.Where("id = ?", ruleID).First(&record).Error; err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			if repository.IsRecordNotFound(err) {
				continue
			}
			return nil, nil, err
		}
		if matchedSession != nil {
			return nil, nil, fmt.Errorf("%w: ambiguous rule id, provide assessmentId", ErrInvalidParam)
		}
		currentSession := summary
		currentRecord := record
		matchedSession = &currentSession
		matchedRecord = &currentRecord
	}
	if matchedSession == nil {
		return nil, nil, ErrRuleNotFound
	}
	return matchedSession, matchedRecord, nil
}

func (s *RuleManagementService) findRuleFileInSession(
	ctx context.Context,
	sessionID uint,
	ruleID uint,
) (*AssessmentSessionSummary, *model.RuleFile, error) {
	summary, err := s.loadSessionSummary(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	record := model.RuleFile{}
	if err := withSessionBusinessDB(ctx, summary, func(sessionDB *gorm.DB) error {
		if err := sessionDB.Where("id = ? AND assessment_id = ?", ruleID, sessionID).First(&record).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, nil, ErrRuleNotFound
		}
		return nil, nil, err
	}
	return summary, &record, nil
}

func validateRuleExpressions(contentJSON string) error {
	content, err := parseCalculationRuleContent(contentJSON)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidExpression, err)
	}
	for scopedIndex, scoped := range content.ScopedRules {
		for gradeIndex, grade := range scoped.Grades {
			if !resolveGradeExtraConditionEnabled(grade) {
				continue
			}
			script := strings.TrimSpace(grade.ExtraConditionScript)
			if script == "" {
				return fmt.Errorf(
					"%w: scopedRules[%d].grades[%d].extraConditionScript is required when extraConditionEnabled is true",
					ErrInvalidExpression,
					scopedIndex,
					gradeIndex,
				)
			}
			if _, err := CompileBool(script); err != nil {
				return err
			}
		}
	}
	return nil
}
