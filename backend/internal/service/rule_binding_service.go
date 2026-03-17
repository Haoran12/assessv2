package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

type ListRuleBindingFilter struct {
	YearID       *uint
	PeriodCode   string
	ObjectType   string
	SegmentCode  string
	OwnerScope   string
	OwnerOrgType string
	OwnerOrgID   *uint
	RuleID       *uint
	IsActive     *bool
}

type RuleBindingDetail struct {
	model.RuleBinding
	RuleName     string `json:"ruleName"`
	RuleCategory string `json:"ruleCategory"`
}

type CreateRuleBindingInput struct {
	YearID       uint   `json:"yearId"`
	PeriodCode   string `json:"periodCode"`
	ObjectType   string `json:"objectType"`
	SegmentCode  string `json:"segmentCode"`
	OwnerScope   string `json:"ownerScope"`
	OwnerOrgType string `json:"ownerOrgType"`
	OwnerOrgID   *uint  `json:"ownerOrgId,omitempty"`
	RuleID       uint   `json:"ruleId"`
	Priority     int    `json:"priority"`
	Description  string `json:"description"`
	IsActive     bool   `json:"isActive"`
}

type UpdateRuleBindingInput struct {
	YearID       uint   `json:"yearId"`
	PeriodCode   string `json:"periodCode"`
	ObjectType   string `json:"objectType"`
	SegmentCode  string `json:"segmentCode"`
	OwnerScope   string `json:"ownerScope"`
	OwnerOrgType string `json:"ownerOrgType"`
	OwnerOrgID   *uint  `json:"ownerOrgId,omitempty"`
	RuleID       uint   `json:"ruleId"`
	Priority     int    `json:"priority"`
	Description  string `json:"description"`
	IsActive     bool   `json:"isActive"`
}

type normalizedRuleBindingInput struct {
	YearID       uint
	PeriodCode   string
	ObjectType   string
	SegmentCode  string
	OwnerScope   string
	OwnerOrgType string
	OwnerOrgID   *uint
	RuleID       uint
	Priority     int
	Description  string
	IsActive     bool
}

func (s *RuleService) ListRuleBindings(ctx context.Context, filter ListRuleBindingFilter) ([]RuleBindingDetail, error) {
	type row struct {
		model.RuleBinding
		RuleName     string
		RuleCategory string
	}

	query := s.db.WithContext(ctx).Table("rule_bindings rb").
		Select("rb.*, ar.rule_name, ar.object_category AS rule_category").
		Joins("LEFT JOIN assessment_rules ar ON ar.id = rb.rule_id")
	if filter.YearID != nil {
		query = query.Where("rb.year_id = ?", *filter.YearID)
	}
	if periodCode := normalizePeriodCode(filter.PeriodCode); periodCode != "" {
		query = query.Where("rb.period_code = ?", periodCode)
	}
	if objectType := strings.TrimSpace(filter.ObjectType); objectType != "" {
		if normalizedObjectType, ok := normalizeObjectType(objectType); ok {
			query = query.Where("rb.object_type = ?", normalizedObjectType)
		} else {
			query = query.Where("1 = 0")
		}
	}
	if segmentCode := strings.TrimSpace(filter.SegmentCode); segmentCode != "" {
		if normalizedSegment := normalizeSegmentCode(segmentCode); normalizedSegment != "" {
			query = query.Where("rb.segment_code = ?", normalizedSegment)
		} else {
			query = query.Where("1 = 0")
		}
	}
	if ownerScope := strings.TrimSpace(filter.OwnerScope); ownerScope != "" {
		query = query.Where("rb.owner_scope = ?", normalizeBindingOwnerScope(ownerScope))
	}
	if ownerOrgType := strings.ToLower(strings.TrimSpace(filter.OwnerOrgType)); ownerOrgType != "" {
		query = query.Where("rb.owner_org_type = ?", ownerOrgType)
	}
	if filter.OwnerOrgID != nil {
		query = query.Where("rb.owner_org_id = ?", *filter.OwnerOrgID)
	}
	if filter.RuleID != nil {
		query = query.Where("rb.rule_id = ?", *filter.RuleID)
	}
	if filter.IsActive != nil {
		query = query.Where("rb.is_active = ?", *filter.IsActive)
	}

	var rows []row
	if err := query.
		Order("rb.year_id DESC, rb.period_code ASC, rb.object_type ASC, rb.segment_code ASC, rb.priority DESC, rb.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list rule bindings: %w", err)
	}

	result := make([]RuleBindingDetail, 0, len(rows))
	for _, item := range rows {
		result = append(result, RuleBindingDetail{
			RuleBinding:  item.RuleBinding,
			RuleName:     item.RuleName,
			RuleCategory: item.RuleCategory,
		})
	}
	return result, nil
}

func (s *RuleService) CreateRuleBinding(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	input CreateRuleBindingInput,
	ipAddress string,
	userAgent string,
) (*RuleBindingDetail, error) {
	normalized, err := normalizeRuleBindingInput(input)
	if err != nil {
		return nil, err
	}
	if err := requireRuleDimensionWriteScope(ctx, s.db, claims, normalized.YearID, normalized.ObjectType, normalized.SegmentCode); err != nil {
		return nil, err
	}

	operator := operatorID
	bindingID := uint(0)
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensurePeriodConfigWritableTx(tx, normalized.YearID, normalized.PeriodCode); err != nil {
			return err
		}
		if err := s.validateRuleBindingReferencesTx(tx, &normalized); err != nil {
			return err
		}
		record := model.RuleBinding{
			YearID:       normalized.YearID,
			PeriodCode:   normalized.PeriodCode,
			ObjectType:   normalized.ObjectType,
			SegmentCode:  normalized.SegmentCode,
			OwnerScope:   normalized.OwnerScope,
			OwnerOrgType: normalized.OwnerOrgType,
			OwnerOrgID:   normalized.OwnerOrgID,
			RuleID:       normalized.RuleID,
			Priority:     normalized.Priority,
			Description:  normalized.Description,
			IsActive:     normalized.IsActive,
			CreatedBy:    &operator,
			UpdatedBy:    &operator,
		}
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("failed to create rule binding: %w", err)
		}
		bindingID = record.ID
		return nil
	})
	if err != nil {
		return nil, err
	}

	result, err := s.loadRuleBindingDetail(ctx, bindingID)
	if err != nil {
		return nil, err
	}
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "create", "rule_bindings", &bindingID, map[string]any{
		"event":       "create_rule_binding",
		"yearId":      result.YearID,
		"periodCode":  result.PeriodCode,
		"objectType":  result.ObjectType,
		"segmentCode": result.SegmentCode,
		"ownerScope":  result.OwnerScope,
		"ownerOrgId":  result.OwnerOrgID,
		"ruleId":      result.RuleID,
		"priority":    result.Priority,
	}, ipAddress, userAgent))
	return result, nil
}

func (s *RuleService) UpdateRuleBinding(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	bindingID uint,
	input UpdateRuleBindingInput,
	ipAddress string,
	userAgent string,
) (*RuleBindingDetail, error) {
	if bindingID == 0 {
		return nil, ErrInvalidParam
	}
	normalized, err := normalizeRuleBindingInput(CreateRuleBindingInput{
		YearID:       input.YearID,
		PeriodCode:   input.PeriodCode,
		ObjectType:   input.ObjectType,
		SegmentCode:  input.SegmentCode,
		OwnerScope:   input.OwnerScope,
		OwnerOrgType: input.OwnerOrgType,
		OwnerOrgID:   input.OwnerOrgID,
		RuleID:       input.RuleID,
		Priority:     input.Priority,
		Description:  input.Description,
		IsActive:     input.IsActive,
	})
	if err != nil {
		return nil, err
	}

	operator := operatorID
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.RuleBinding
		if err := tx.Where("id = ?", bindingID).First(&existing).Error; err != nil {
			if repository.IsRecordNotFound(err) {
				return ErrRuleBindingNotFound
			}
			return fmt.Errorf("failed to query rule binding: %w", err)
		}

		if err := requireRuleDimensionWriteScope(ctx, s.db, claims, existing.YearID, existing.ObjectType, existing.SegmentCode); err != nil {
			return err
		}
		if err := requireRuleDimensionWriteScope(ctx, s.db, claims, normalized.YearID, normalized.ObjectType, normalized.SegmentCode); err != nil {
			return err
		}

		if err := ensurePeriodConfigWritableTx(tx, existing.YearID, existing.PeriodCode); err != nil {
			return err
		}
		if existing.YearID != normalized.YearID || existing.PeriodCode != normalized.PeriodCode {
			if err := ensurePeriodConfigWritableTx(tx, normalized.YearID, normalized.PeriodCode); err != nil {
				return err
			}
		}
		if err := s.validateRuleBindingReferencesTx(tx, &normalized); err != nil {
			return err
		}

		if err := tx.Model(&model.RuleBinding{}).
			Where("id = ?", bindingID).
			Updates(map[string]any{
				"year_id":        normalized.YearID,
				"period_code":    normalized.PeriodCode,
				"object_type":    normalized.ObjectType,
				"segment_code":   normalized.SegmentCode,
				"owner_scope":    normalized.OwnerScope,
				"owner_org_type": normalized.OwnerOrgType,
				"owner_org_id":   normalized.OwnerOrgID,
				"rule_id":        normalized.RuleID,
				"priority":       normalized.Priority,
				"description":    normalized.Description,
				"is_active":      normalized.IsActive,
				"updated_by":     &operator,
				"updated_at":     time.Now().Unix(),
			}).Error; err != nil {
			return fmt.Errorf("failed to update rule binding: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	result, err := s.loadRuleBindingDetail(ctx, bindingID)
	if err != nil {
		return nil, err
	}
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "rule_bindings", &bindingID, map[string]any{
		"event":       "update_rule_binding",
		"yearId":      result.YearID,
		"periodCode":  result.PeriodCode,
		"objectType":  result.ObjectType,
		"segmentCode": result.SegmentCode,
		"ownerScope":  result.OwnerScope,
		"ownerOrgId":  result.OwnerOrgID,
		"ruleId":      result.RuleID,
		"priority":    result.Priority,
		"isActive":    result.IsActive,
	}, ipAddress, userAgent))
	return result, nil
}

func (s *RuleService) DeleteRuleBinding(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	bindingID uint,
	ipAddress string,
	userAgent string,
) error {
	if bindingID == 0 {
		return ErrInvalidParam
	}

	operator := operatorID
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.RuleBinding
		if err := tx.Where("id = ?", bindingID).First(&existing).Error; err != nil {
			if repository.IsRecordNotFound(err) {
				return ErrRuleBindingNotFound
			}
			return fmt.Errorf("failed to query rule binding: %w", err)
		}
		if err := requireRuleDimensionWriteScope(ctx, s.db, claims, existing.YearID, existing.ObjectType, existing.SegmentCode); err != nil {
			return err
		}
		if err := ensurePeriodConfigWritableTx(tx, existing.YearID, existing.PeriodCode); err != nil {
			return err
		}
		if err := tx.Delete(&model.RuleBinding{}, bindingID).Error; err != nil {
			return fmt.Errorf("failed to delete rule binding: %w", err)
		}
		_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "delete", "rule_bindings", &bindingID, map[string]any{
			"event":       "delete_rule_binding",
			"yearId":      existing.YearID,
			"periodCode":  existing.PeriodCode,
			"objectType":  existing.ObjectType,
			"segmentCode": existing.SegmentCode,
		}, ipAddress, userAgent))
		return nil
	})
}

func normalizeRuleBindingInput(input CreateRuleBindingInput) (normalizedRuleBindingInput, error) {
	if input.YearID == 0 || input.RuleID == 0 {
		return normalizedRuleBindingInput{}, ErrInvalidParam
	}

	periodCode := normalizePeriodCode(input.PeriodCode)
	if !isValidPeriodCode(periodCode) {
		return normalizedRuleBindingInput{}, ErrInvalidRulePeriodCode
	}
	objectType, ok := normalizeObjectType(input.ObjectType)
	if !ok {
		return normalizedRuleBindingInput{}, ErrInvalidRuleObjectType
	}

	segmentCode := normalizeSegmentCode(input.SegmentCode)
	if segmentCode == "" {
		return normalizedRuleBindingInput{}, ErrInvalidRuleObjectCategory
	}
	if segments, ok := segmentSetByObjectType[objectType]; !ok {
		return normalizedRuleBindingInput{}, ErrInvalidRuleObjectType
	} else {
		if _, exists := segments[segmentCode]; !exists {
			return normalizedRuleBindingInput{}, ErrInvalidRuleObjectCategory
		}
	}

	ownerScope := normalizeBindingOwnerScope(input.OwnerScope)
	ownerOrgType := strings.ToLower(strings.TrimSpace(input.OwnerOrgType))
	var ownerOrgID *uint
	if input.OwnerOrgID != nil {
		value := *input.OwnerOrgID
		ownerOrgID = &value
	}

	switch ownerScope {
	case ruleBindingOwnerScopeGlobal:
		ownerOrgType = ""
		ownerOrgID = nil
	case ruleBindingOwnerScopeOrganizationType:
		if ownerOrgType != "group" && ownerOrgType != "company" {
			return normalizedRuleBindingInput{}, ErrInvalidOrganizationType
		}
		ownerOrgID = nil
	case ruleBindingOwnerScopeOrganization:
		if ownerOrgID == nil || *ownerOrgID == 0 {
			return normalizedRuleBindingInput{}, ErrInvalidParam
		}
	default:
		return normalizedRuleBindingInput{}, ErrInvalidRuleBindingScope
	}

	return normalizedRuleBindingInput{
		YearID:       input.YearID,
		PeriodCode:   periodCode,
		ObjectType:   objectType,
		SegmentCode:  segmentCode,
		OwnerScope:   ownerScope,
		OwnerOrgType: ownerOrgType,
		OwnerOrgID:   ownerOrgID,
		RuleID:       input.RuleID,
		Priority:     input.Priority,
		Description:  strings.TrimSpace(input.Description),
		IsActive:     input.IsActive,
	}, nil
}

func (s *RuleService) validateRuleBindingReferencesTx(tx *gorm.DB, normalized *normalizedRuleBindingInput) error {
	if normalized == nil {
		return ErrInvalidParam
	}
	if err := ensureAssessmentYearExists(tx, normalized.YearID); err != nil {
		return err
	}

	switch normalized.OwnerScope {
	case ruleBindingOwnerScopeOrganization:
		if normalized.OwnerOrgID == nil {
			return ErrInvalidParam
		}
		var organization model.Organization
		if err := tx.
			Where("id = ? AND deleted_at IS NULL", *normalized.OwnerOrgID).
			First(&organization).Error; err != nil {
			if repository.IsRecordNotFound(err) {
				return ErrOrganizationNotFound
			}
			return fmt.Errorf("failed to query owner organization for rule binding: %w", err)
		}
		normalized.OwnerOrgType = strings.ToLower(strings.TrimSpace(organization.OrgType))
	case ruleBindingOwnerScopeOrganizationType:
		if normalized.OwnerOrgType != "group" && normalized.OwnerOrgType != "company" {
			return ErrInvalidOrganizationType
		}
	case ruleBindingOwnerScopeGlobal:
	default:
		return ErrInvalidRuleBindingScope
	}

	var rule model.AssessmentRule
	if err := tx.Where("id = ?", normalized.RuleID).First(&rule).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return ErrRuleNotFound
		}
		return fmt.Errorf("failed to query assessment rule for binding: %w", err)
	}

	if rule.YearID != normalized.YearID ||
		normalizePeriodCode(rule.PeriodCode) != normalized.PeriodCode ||
		strings.ToLower(strings.TrimSpace(rule.ObjectType)) != normalized.ObjectType {
		return ErrInvalidParam
	}

	return nil
}

func (s *RuleService) loadRuleBindingDetail(ctx context.Context, bindingID uint) (*RuleBindingDetail, error) {
	type row struct {
		model.RuleBinding
		RuleName     string
		RuleCategory string
	}
	var item row
	err := s.db.WithContext(ctx).Table("rule_bindings rb").
		Select("rb.*, ar.rule_name, ar.object_category AS rule_category").
		Joins("LEFT JOIN assessment_rules ar ON ar.id = rb.rule_id").
		Where("rb.id = ?", bindingID).
		First(&item).Error
	if err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrRuleBindingNotFound
		}
		return nil, fmt.Errorf("failed to load rule binding detail: %w", err)
	}

	return &RuleBindingDetail{
		RuleBinding:  item.RuleBinding,
		RuleName:     item.RuleName,
		RuleCategory: item.RuleCategory,
	}, nil
}
