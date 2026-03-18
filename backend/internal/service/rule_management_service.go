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

type SelectRuleBindingInput struct {
	AssessmentID    uint
	PeriodCode      string
	ObjectGroupCode string
	SourceRuleID    uint
}

type AssessmentRuleBindingDetail struct {
	model.AssessmentRuleBindingV2
	RuleFile RuleFileSummary `json:"ruleFile"`
}

type RuleBindingListFilter struct {
	AssessmentID uint
	PeriodCode   string
}

func NewRuleManagementService(db *gorm.DB, auditRepo *repository.AuditRepository) *RuleManagementService {
	return &RuleManagementService{db: db, auditRepo: auditRepo}
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

	query := s.db.WithContext(ctx).Model(&model.RuleFile{}).Where("assessment_id = ?", filter.AssessmentID)
	items := make([]model.RuleFile, 0, 32)
	if err := query.Order("is_copy ASC, id DESC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list rule files: %w", err)
	}

	adminOrgID := uint(0)
	if !isRootClaims(claims) {
		adminOrgID, err = resolveAdminOrganizationID(claims)
		if err != nil {
			return nil, err
		}
	}

	hiddenSet := map[uint]struct{}{}
	if adminOrgID > 0 {
		var hiddenIDs []uint
		if err := s.db.WithContext(ctx).
			Table("rule_file_hides").
			Where("organization_id = ?", adminOrgID).
			Pluck("rule_file_id", &hiddenIDs).Error; err != nil {
			return nil, fmt.Errorf("failed to load hidden rule files: %w", err)
		}
		for _, id := range hiddenIDs {
			hiddenSet[id] = struct{}{}
		}
	}

	result := make([]RuleFileSummary, 0, len(items))
	for _, item := range items {
		_, hidden := hiddenSet[item.ID]
		if hidden && !filter.IncludeHidden {
			continue
		}
		canEdit := false
		canDelete := false
		if item.IsCopy {
			if isRootClaims(claims) {
				canEdit = true
				canDelete = true
			} else if item.OwnerOrgID != nil && *item.OwnerOrgID == adminOrgID {
				canEdit = true
				canDelete = true
			}
		} else {
			canDelete = isRootClaims(claims)
		}

		result = append(result, RuleFileSummary{
			RuleFile:            item,
			HiddenByCurrentOrg:  hidden,
			CanEdit:             canEdit,
			CanDelete:           canDelete,
		})
	}
	return result, nil
}

func (s *RuleManagementService) CreateRuleFile(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	input RuleFileInput,
	ipAddress string,
	userAgent string,
) (*RuleFileSummary, error) {
	if !isRootClaims(claims) {
		return nil, ErrForbidden
	}
	if input.AssessmentID == 0 {
		return nil, ErrInvalidParam
	}
	session, err := s.loadSessionSummary(ctx, input.AssessmentID)
	if err != nil {
		return nil, err
	}
	ruleName := strings.TrimSpace(input.RuleName)
	if ruleName == "" {
		return nil, ErrInvalidRuleName
	}
	contentJSON := strings.TrimSpace(input.ContentJSON)
	if contentJSON == "" {
		contentJSON = buildDefaultRuleTemplateJSON()
	}

	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID)
	record := model.RuleFile{}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		operatorRef = resolveBusinessWriteOperatorRefTx(tx, operatorID)
		fileName := buildRuleFileName(ruleName)
		filePath, err := ensureRuleFilePath(session.AssessmentName, fileName)
		if err != nil {
			return err
		}
		if writeErr := os.WriteFile(filePath, []byte(contentJSON), 0o644); writeErr != nil {
			return fmt.Errorf("failed to write rule file content: %w", writeErr)
		}
		record = model.RuleFile{
			AssessmentID: input.AssessmentID,
			RuleName:     ruleName,
			Description:  strings.TrimSpace(input.Description),
			ContentJSON:  contentJSON,
			FilePath:     filePath,
			IsCopy:       false,
			CreatedBy:    operatorRef,
			UpdatedBy:    operatorRef,
		}
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("failed to create rule file metadata: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(operatorRef, "create", "rule_files", &targetID, map[string]any{
		"event":         "create_rule_file",
		"assessmentId":  input.AssessmentID,
		"ruleName":      ruleName,
		"isCopy":        false,
	}, ipAddress, userAgent))

	return &RuleFileSummary{
		RuleFile:            record,
		HiddenByCurrentOrg:  false,
		CanEdit:             false,
		CanDelete:           true,
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

	var record model.RuleFile
	if err := s.db.WithContext(ctx).Where("id = ?", ruleID).First(&record).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrRuleNotFound
		}
		return nil, fmt.Errorf("failed to query rule file: %w", err)
	}

	session, err := s.loadSessionSummary(ctx, record.AssessmentID)
	if err != nil {
		return nil, err
	}
	if err := ensureAssessmentOrganizationScope(claims, session.OrganizationID); err != nil {
		return nil, err
	}
	if !record.IsCopy {
		return nil, ErrForbidden
	}
	if !isRootClaims(claims) {
		adminOrgID, err := resolveAdminOrganizationID(claims)
		if err != nil {
			return nil, err
		}
		if record.OwnerOrgID == nil || *record.OwnerOrgID != adminOrgID {
			return nil, ErrForbidden
		}
	}

	contentJSON := strings.TrimSpace(input.ContentJSON)
	if contentJSON == "" {
		contentJSON = record.ContentJSON
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
	if err := s.db.WithContext(ctx).Model(&model.RuleFile{}).Where("id = ?", ruleID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update rule file metadata: %w", err)
	}
	if writeErr := os.WriteFile(record.FilePath, []byte(contentJSON), 0o644); writeErr != nil {
		return nil, fmt.Errorf("failed to update rule file: %w", writeErr)
	}

	if err := s.db.WithContext(ctx).Where("id = ?", ruleID).First(&record).Error; err != nil {
		return nil, fmt.Errorf("failed to reload rule file: %w", err)
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(operatorRef, "update", "rule_files", &targetID, map[string]any{
		"event": "update_rule_file",
	}, ipAddress, userAgent))

	return &RuleFileSummary{
		RuleFile:            record,
		HiddenByCurrentOrg:  false,
		CanEdit:             true,
		CanDelete:           true,
	}, nil
}

func (s *RuleManagementService) DeleteRuleFile(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	ruleID uint,
	ipAddress string,
	userAgent string,
) error {
	if ruleID == 0 {
		return ErrInvalidParam
	}
	var record model.RuleFile
	if err := s.db.WithContext(ctx).Where("id = ?", ruleID).First(&record).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return ErrRuleNotFound
		}
		return fmt.Errorf("failed to query rule file: %w", err)
	}
	session, err := s.loadSessionSummary(ctx, record.AssessmentID)
	if err != nil {
		return err
	}
	if err := ensureAssessmentOrganizationScope(claims, session.OrganizationID); err != nil {
		return err
	}

	if record.IsCopy {
		if !isRootClaims(claims) {
			adminOrgID, err := resolveAdminOrganizationID(claims)
			if err != nil {
				return err
			}
			if record.OwnerOrgID == nil || *record.OwnerOrgID != adminOrgID {
				return ErrForbidden
			}
		}
	} else if !isRootClaims(claims) {
		return ErrForbidden
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("rule_file_id = ?", ruleID).Delete(&model.AssessmentRuleBindingV2{}).Error; err != nil {
			return fmt.Errorf("failed to delete rule bindings: %w", err)
		}
		if err := tx.Where("rule_file_id = ?", ruleID).Delete(&model.RuleFileHide{}).Error; err != nil {
			return fmt.Errorf("failed to delete rule hides: %w", err)
		}
		if err := tx.Delete(&model.RuleFile{}, ruleID).Error; err != nil {
			return fmt.Errorf("failed to delete rule file: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	_ = os.Remove(record.FilePath)
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID)
	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(operatorRef, "delete", "rule_files", &targetID, map[string]any{
		"event": "delete_rule_file",
	}, ipAddress, userAgent))
	return nil
}

func (s *RuleManagementService) HideRuleFile(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	ruleID uint,
) error {
	if isRootClaims(claims) {
		return ErrForbidden
	}
	adminOrgID, err := resolveAdminOrganizationID(claims)
	if err != nil {
		return err
	}
	var record model.RuleFile
	if err := s.db.WithContext(ctx).Where("id = ?", ruleID).First(&record).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return ErrRuleNotFound
		}
		return err
	}
	if record.IsCopy {
		return ErrForbidden
	}
	hide := model.RuleFileHide{
		RuleFileID:     ruleID,
		OrganizationID: adminOrgID,
		CreatedBy:      resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID),
	}
	if err := s.db.WithContext(ctx).Create(&hide).Error; err != nil && !isUniqueConstraintError(err) {
		return fmt.Errorf("failed to hide rule file: %w", err)
	}
	return nil
}

func (s *RuleManagementService) UnhideRuleFile(
	ctx context.Context,
	claims *auth.Claims,
	ruleID uint,
) error {
	if isRootClaims(claims) {
		return ErrForbidden
	}
	adminOrgID, err := resolveAdminOrganizationID(claims)
	if err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).
		Where("rule_file_id = ? AND organization_id = ?", ruleID, adminOrgID).
		Delete(&model.RuleFileHide{}).Error; err != nil {
		return fmt.Errorf("failed to unhide rule file: %w", err)
	}
	return nil
}

func (s *RuleManagementService) ListBindings(
	ctx context.Context,
	claims *auth.Claims,
	filter RuleBindingListFilter,
) ([]AssessmentRuleBindingDetail, error) {
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

	query := s.db.WithContext(ctx).Model(&model.AssessmentRuleBindingV2{}).
		Where("assessment_id = ?", filter.AssessmentID)
	if period := strings.TrimSpace(filter.PeriodCode); period != "" {
		query = query.Where("period_code = ?", strings.ToUpper(period))
	}
	rows := make([]model.AssessmentRuleBindingV2, 0, 32)
	if err := query.Order("period_code ASC, object_group_code ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list rule bindings: %w", err)
	}
	if len(rows) == 0 {
		return []AssessmentRuleBindingDetail{}, nil
	}

	ruleIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		ruleIDs = append(ruleIDs, row.RuleFileID)
	}
	files := make([]model.RuleFile, 0, len(rows))
	if err := s.db.WithContext(ctx).Where("id IN ?", ruleIDs).Find(&files).Error; err != nil {
		return nil, fmt.Errorf("failed to query bound rule files: %w", err)
	}
	fileMap := map[uint]model.RuleFile{}
	for _, item := range files {
		fileMap[item.ID] = item
	}

	result := make([]AssessmentRuleBindingDetail, 0, len(rows))
	for _, row := range rows {
		ruleFile, ok := fileMap[row.RuleFileID]
		if !ok {
			continue
		}
		result = append(result, AssessmentRuleBindingDetail{
			AssessmentRuleBindingV2: row,
			RuleFile: RuleFileSummary{
				RuleFile:           ruleFile,
				CanEdit:            ruleFile.IsCopy,
				CanDelete:          ruleFile.IsCopy || isRootClaims(claims),
				HiddenByCurrentOrg: false,
			},
		})
	}
	return result, nil
}

func (s *RuleManagementService) SelectRuleForBinding(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	input SelectRuleBindingInput,
	ipAddress string,
	userAgent string,
) (*AssessmentRuleBindingDetail, error) {
	if input.AssessmentID == 0 || input.SourceRuleID == 0 || strings.TrimSpace(input.PeriodCode) == "" || strings.TrimSpace(input.ObjectGroupCode) == "" {
		return nil, ErrInvalidParam
	}
	session, err := s.loadSessionSummary(ctx, input.AssessmentID)
	if err != nil {
		return nil, err
	}
	if err := ensureAssessmentOrganizationScope(claims, session.OrganizationID); err != nil {
		return nil, err
	}

	periodCode := strings.ToUpper(strings.TrimSpace(input.PeriodCode))
	objectGroupCode := strings.TrimSpace(input.ObjectGroupCode)
	source := model.RuleFile{}
	if err := s.db.WithContext(ctx).Where("id = ? AND assessment_id = ?", input.SourceRuleID, input.AssessmentID).First(&source).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrRuleNotFound
		}
		return nil, fmt.Errorf("failed to query source rule file: %w", err)
	}

	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID)
	binding := model.AssessmentRuleBindingV2{}
	copyRule := model.RuleFile{}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		operatorRef = resolveBusinessWriteOperatorRefTx(tx, operatorID)
		if !s.periodExistsTx(tx, input.AssessmentID, periodCode) {
			return ErrPeriodNotFound
		}
		if !s.objectGroupExistsTx(tx, input.AssessmentID, objectGroupCode) {
			return ErrInvalidRuleObjectCategory
		}

		fileName := buildRuleFileName(source.RuleName + "_copy")
		filePath, err := ensureRuleFilePath(session.AssessmentName, fileName)
		if err != nil {
			return err
		}
		if writeErr := os.WriteFile(filePath, []byte(source.ContentJSON), 0o644); writeErr != nil {
			return fmt.Errorf("failed to create copied rule file: %w", writeErr)
		}

		orgID := session.OrganizationID
		copyRule = model.RuleFile{
			AssessmentID: input.AssessmentID,
			RuleName:     source.RuleName + " (拷贝)",
			Description:  source.Description,
			ContentJSON:  source.ContentJSON,
			FilePath:     filePath,
			IsCopy:       true,
			SourceRuleID: uintPtr(source.ID),
			OwnerOrgID:   uintPtr(orgID),
			CreatedBy:    operatorRef,
			UpdatedBy:    operatorRef,
		}
		if err := tx.Create(&copyRule).Error; err != nil {
			return fmt.Errorf("failed to save copied rule file metadata: %w", err)
		}

		query := tx.Where(
			"assessment_id = ? AND period_code = ? AND object_group_code = ? AND organization_id = ?",
			input.AssessmentID,
			periodCode,
			objectGroupCode,
			orgID,
		)
		err = query.First(&binding).Error
		if err == nil {
			if updateErr := query.Updates(map[string]any{
				"rule_file_id": copyRule.ID,
				"updated_by":   operatorRef,
				"updated_at":   time.Now().Unix(),
			}).Error; updateErr != nil {
				return fmt.Errorf("failed to update existing rule binding: %w", updateErr)
			}
			if reloadErr := query.First(&binding).Error; reloadErr != nil {
				return fmt.Errorf("failed to reload updated rule binding: %w", reloadErr)
			}
			return nil
		}
		if !repository.IsRecordNotFound(err) {
			return fmt.Errorf("failed to query existing rule binding: %w", err)
		}
		binding = model.AssessmentRuleBindingV2{
			AssessmentID:    input.AssessmentID,
			PeriodCode:      periodCode,
			ObjectGroupCode: objectGroupCode,
			OrganizationID:  orgID,
			RuleFileID:      copyRule.ID,
			CreatedBy:       operatorRef,
			UpdatedBy:       operatorRef,
		}
		if err := tx.Create(&binding).Error; err != nil {
			return fmt.Errorf("failed to create rule binding: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	targetID := binding.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(operatorRef, "create", "assessment_rule_bindings", &targetID, map[string]any{
		"event":          "bind_rule_with_copy",
		"assessmentId":   input.AssessmentID,
		"periodCode":     periodCode,
		"objectGroupCode": objectGroupCode,
		"sourceRuleId":   input.SourceRuleID,
		"copyRuleId":     copyRule.ID,
	}, ipAddress, userAgent))

	return &AssessmentRuleBindingDetail{
		AssessmentRuleBindingV2: binding,
		RuleFile: RuleFileSummary{
			RuleFile:            copyRule,
			HiddenByCurrentOrg:  false,
			CanEdit:             true,
			CanDelete:           true,
		},
	}, nil
}

func (s *RuleManagementService) periodExistsTx(tx *gorm.DB, assessmentID uint, periodCode string) bool {
	var count int64
	_ = tx.Model(&model.AssessmentSessionPeriod{}).
		Where("assessment_id = ? AND period_code = ?", assessmentID, periodCode).
		Count(&count).Error
	return count > 0
}

func (s *RuleManagementService) objectGroupExistsTx(tx *gorm.DB, assessmentID uint, groupCode string) bool {
	var count int64
	_ = tx.Model(&model.AssessmentObjectGroup{}).
		Where("assessment_id = ? AND group_code = ?", assessmentID, groupCode).
		Count(&count).Error
	return count > 0
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
