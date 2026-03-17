package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

type RuleService struct {
	db        *gorm.DB
	auditRepo *repository.AuditRepository
}

type ListRuleFilter struct {
	YearID         *uint
	PeriodCode     string
	ObjectType     string
	ObjectCategory string
}

type RuleSummary struct {
	model.AssessmentRule
	ModuleCount int `json:"moduleCount"`
}

type RuleVoteGroupInput struct {
	GroupCode  string  `json:"groupCode"`
	GroupName  string  `json:"groupName"`
	Weight     float64 `json:"weight"`
	VoterType  string  `json:"voterType"`
	VoterScope string  `json:"voterScope"`
	MaxScore   float64 `json:"maxScore"`
	SortOrder  int     `json:"sortOrder"`
	IsActive   bool    `json:"isActive"`
}

type RuleModuleInput struct {
	ModuleCode        string               `json:"moduleCode"`
	ModuleKey         string               `json:"moduleKey"`
	ModuleName        string               `json:"moduleName"`
	Weight            *float64             `json:"weight,omitempty"`
	MaxScore          *float64             `json:"maxScore,omitempty"`
	CalculationMethod string               `json:"calculationMethod"`
	Expression        string               `json:"expression"`
	ContextScope      string               `json:"contextScope"`
	SortOrder         int                  `json:"sortOrder"`
	IsActive          bool                 `json:"isActive"`
	VoteGroups        []RuleVoteGroupInput `json:"voteGroups,omitempty"`
}

type RuleModuleDetail struct {
	model.ScoreModule
	VoteGroups []model.VoteGroup `json:"voteGroups"`
}

type RuleDetail struct {
	Rule    model.AssessmentRule `json:"rule"`
	Modules []RuleModuleDetail   `json:"modules"`
}

type CreateRuleInput struct {
	YearID         uint              `json:"yearId"`
	PeriodCode     string            `json:"periodCode"`
	ObjectType     string            `json:"objectType"`
	ObjectCategory string            `json:"objectCategory"`
	RuleName       string            `json:"ruleName"`
	Description    string            `json:"description"`
	IsActive       bool              `json:"isActive"`
	SyncQuarterly  bool              `json:"syncQuarterly"`
	Modules        []RuleModuleInput `json:"modules"`
}

type UpdateRuleInput struct {
	RuleName      string            `json:"ruleName"`
	Description   string            `json:"description"`
	IsActive      bool              `json:"isActive"`
	SyncQuarterly bool              `json:"syncQuarterly"`
	Modules       []RuleModuleInput `json:"modules"`
}

type RuleTemplateConfig struct {
	RuleName    string            `json:"ruleName"`
	Description string            `json:"description"`
	Modules     []RuleModuleInput `json:"modules"`
}

type CreateRuleTemplateInput struct {
	TemplateName   string             `json:"templateName"`
	ObjectType     string             `json:"objectType"`
	ObjectCategory string             `json:"objectCategory"`
	Description    string             `json:"description"`
	Config         RuleTemplateConfig `json:"config"`
}

type ListRuleTemplateFilter struct {
	ObjectType     string
	ObjectCategory string
}

type RuleTemplateSummary struct {
	model.RuleTemplate
	Config RuleTemplateConfig `json:"config"`
}

type ApplyRuleTemplateInput struct {
	YearID         uint   `json:"yearId"`
	PeriodCode     string `json:"periodCode"`
	ObjectType     string `json:"objectType"`
	ObjectCategory string `json:"objectCategory"`
	RuleName       string `json:"ruleName"`
	Description    string `json:"description"`
	SyncQuarterly  bool   `json:"syncQuarterly"`
	IsActive       bool   `json:"isActive"`
	Overwrite      bool   `json:"overwrite"`
}

type upsertRuleMode struct {
	AllowCreate bool
	AllowUpdate bool
}

type ruleDimension struct {
	YearID         uint
	PeriodCode     string
	ObjectType     string
	ObjectCategory string
}

var (
	ruleObjectTypeSet = categorySetByObjectType
	moduleCodeSet     = map[string]struct{}{
		"direct": {}, "vote": {}, "custom": {}, "extra": {},
	}
	voterTypeSet = map[string]struct{}{
		"leadership_main": {}, "leadership_deputy": {},
		"group_leader": {}, "company_leader": {}, // legacy compatibility
		"dept_leader": {}, "peer": {}, "subordinate": {}, "custom": {},
	}
	expressionFuncSet = map[string]struct{}{
		"abs": {}, "round": {}, "ceil": {}, "floor": {}, "max": {}, "min": {}, "if": {}, "avg": {}, "sum": {},
	}
	expressionVarSet = map[string]struct{}{
		"team.score": {}, "team.rank": {},
		"q1.score": {}, "q2.score": {}, "q3.score": {}, "q4.score": {},
		"extra_points": {},
	}
	expressionIdentifierPattern  = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_.]*`)
	expressionInvalidCharPattern = regexp.MustCompile(`[^A-Za-z0-9_.,()+\-*/%<>=!&|\s]`)
)

func NewRuleService(db *gorm.DB, auditRepo *repository.AuditRepository) *RuleService {
	return &RuleService{db: db, auditRepo: auditRepo}
}

func (s *RuleService) ListRules(ctx context.Context, filter ListRuleFilter) ([]RuleSummary, error) {
	query := s.db.WithContext(ctx).Model(&model.AssessmentRule{})
	if filter.YearID != nil {
		query = query.Where("year_id = ?", *filter.YearID)
	}
	if periodCode := strings.TrimSpace(filter.PeriodCode); periodCode != "" {
		query = query.Where("period_code = ?", periodCode)
	}
	if objectType := strings.TrimSpace(filter.ObjectType); objectType != "" {
		normalizedType, ok := normalizeObjectType(objectType)
		if ok {
			query = query.Where("object_type = ?", normalizedType)
		} else {
			query = query.Where("1 = 0")
		}
	}
	if objectCategory := strings.TrimSpace(filter.ObjectCategory); objectCategory != "" {
		query = query.Where("object_category = ?", normalizeObjectCategory(objectCategory))
	}

	var rules []model.AssessmentRule
	if err := query.Order("year_id DESC, id DESC").Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("failed to list assessment rules: %w", err)
	}
	if len(rules) == 0 {
		return []RuleSummary{}, nil
	}

	ruleIDs := make([]uint, 0, len(rules))
	for _, item := range rules {
		ruleIDs = append(ruleIDs, item.ID)
	}
	type moduleCountRow struct {
		RuleID uint
		Count  int
	}
	var rows []moduleCountRow
	if err := s.db.WithContext(ctx).Table("score_modules").
		Select("rule_id, COUNT(1) AS count").
		Where("rule_id IN ?", ruleIDs).
		Group("rule_id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query score module count: %w", err)
	}
	countMap := make(map[uint]int, len(rows))
	for _, item := range rows {
		countMap[item.RuleID] = item.Count
	}

	result := make([]RuleSummary, 0, len(rules))
	for _, item := range rules {
		result = append(result, RuleSummary{
			AssessmentRule: item,
			ModuleCount:    countMap[item.ID],
		})
	}
	return result, nil
}

func (s *RuleService) GetRule(ctx context.Context, ruleID uint) (*RuleDetail, error) {
	if ruleID == 0 {
		return nil, ErrInvalidParam
	}
	return s.loadRuleDetail(ctx, ruleID)
}

func (s *RuleService) CreateRule(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	input CreateRuleInput,
	ipAddress string,
	userAgent string,
) (*RuleDetail, error) {
	dimension, err := normalizeRuleDimension(input.YearID, input.PeriodCode, input.ObjectType, input.ObjectCategory)
	if err != nil {
		return nil, err
	}
	if err := requireRuleDimensionWriteScope(ctx, s.db, claims, dimension.YearID, dimension.ObjectType, dimension.ObjectCategory); err != nil {
		return nil, err
	}
	ruleName := strings.TrimSpace(input.RuleName)
	if ruleName == "" {
		return nil, ErrInvalidRuleName
	}
	modules, err := normalizeAndValidateModules(input.Modules)
	if err != nil {
		return nil, err
	}

	operator := operatorID
	baseRuleID := uint(0)
	periods := targetPeriods(dimension.PeriodCode, input.SyncQuarterly)
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, periodCode := range periods {
			if err := ensurePeriodConfigWritableTx(tx, dimension.YearID, periodCode); err != nil {
				return err
			}
			currentDimension := dimension
			currentDimension.PeriodCode = periodCode
			rule, err := s.upsertRuleByDimensionTx(tx, &operator, currentDimension, ruleName, strings.TrimSpace(input.Description), input.IsActive, modules, upsertRuleMode{
				AllowCreate: true,
				AllowUpdate: false,
			})
			if err != nil {
				return err
			}
			if periodCode == dimension.PeriodCode {
				baseRuleID = rule.ID
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	result, err := s.loadRuleDetail(ctx, baseRuleID)
	if err != nil {
		return nil, err
	}
	targetID := result.Rule.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "create", "assessment_rules", &targetID, map[string]any{
		"event":          "create_assessment_rule",
		"yearId":         result.Rule.YearID,
		"periodCode":     result.Rule.PeriodCode,
		"objectType":     result.Rule.ObjectType,
		"objectCategory": result.Rule.ObjectCategory,
		"syncQuarterly":  input.SyncQuarterly,
	}, ipAddress, userAgent))

	return result, nil
}

func (s *RuleService) UpdateRule(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	ruleID uint,
	input UpdateRuleInput,
	ipAddress string,
	userAgent string,
) (*RuleDetail, error) {
	if ruleID == 0 {
		return nil, ErrInvalidParam
	}
	ruleName := strings.TrimSpace(input.RuleName)
	if ruleName == "" {
		return nil, ErrInvalidRuleName
	}
	modules, err := normalizeAndValidateModules(input.Modules)
	if err != nil {
		return nil, err
	}

	operator := operatorID
	var baseRule model.AssessmentRule
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", ruleID).First(&baseRule).Error; err != nil {
			if repository.IsRecordNotFound(err) {
				return ErrRuleNotFound
			}
			return fmt.Errorf("failed to query assessment rule: %w", err)
		}
		if err := requireRuleDimensionWriteScope(ctx, s.db, claims, baseRule.YearID, baseRule.ObjectType, baseRule.ObjectCategory); err != nil {
			return err
		}
		if err := ensurePeriodConfigWritableTx(tx, baseRule.YearID, baseRule.PeriodCode); err != nil {
			return err
		}
		if err := s.replaceRuleConfigTx(tx, &operator, baseRule.ID, ruleName, strings.TrimSpace(input.Description), input.IsActive, modules); err != nil {
			return err
		}

		if input.SyncQuarterly && isQuarterPeriod(baseRule.PeriodCode) {
			periods := targetPeriods(baseRule.PeriodCode, true)
			for _, periodCode := range periods {
				if periodCode == baseRule.PeriodCode {
					continue
				}
				if err := ensurePeriodConfigWritableTx(tx, baseRule.YearID, periodCode); err != nil {
					return err
				}
				_, err := s.upsertRuleByDimensionTx(tx, &operator, ruleDimension{
					YearID:         baseRule.YearID,
					PeriodCode:     periodCode,
					ObjectType:     baseRule.ObjectType,
					ObjectCategory: baseRule.ObjectCategory,
				}, ruleName, strings.TrimSpace(input.Description), input.IsActive, modules, upsertRuleMode{
					AllowCreate: true,
					AllowUpdate: true,
				})
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	result, err := s.loadRuleDetail(ctx, baseRule.ID)
	if err != nil {
		return nil, err
	}
	targetID := result.Rule.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "assessment_rules", &targetID, map[string]any{
		"event":         "update_assessment_rule",
		"syncQuarterly": input.SyncQuarterly,
	}, ipAddress, userAgent))
	return result, nil
}
func (s *RuleService) ListTemplates(ctx context.Context, filter ListRuleTemplateFilter) ([]RuleTemplateSummary, error) {
	query := s.db.WithContext(ctx).Model(&model.RuleTemplate{})
	if objectType := strings.TrimSpace(filter.ObjectType); objectType != "" {
		normalizedType, ok := normalizeObjectType(objectType)
		if ok {
			query = query.Where("object_type = ?", normalizedType)
		} else {
			query = query.Where("1 = 0")
		}
	}
	if objectCategory := strings.TrimSpace(filter.ObjectCategory); objectCategory != "" {
		query = query.Where("object_category = ?", normalizeObjectCategory(objectCategory))
	}

	var templates []model.RuleTemplate
	if err := query.Order("is_system DESC, id DESC").Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("failed to list rule templates: %w", err)
	}

	result := make([]RuleTemplateSummary, 0, len(templates))
	for _, item := range templates {
		config := RuleTemplateConfig{}
		if err := json.Unmarshal([]byte(item.TemplateConfig), &config); err != nil {
			return nil, fmt.Errorf("failed to parse template config id=%d: %w", item.ID, err)
		}
		result = append(result, RuleTemplateSummary{
			RuleTemplate: item,
			Config:       config,
		})
	}
	return result, nil
}

func (s *RuleService) CreateTemplate(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	input CreateRuleTemplateInput,
	ipAddress string,
	userAgent string,
) (*RuleTemplateSummary, error) {
	if err := requireRootOrAssessmentAdminClaims(claims); err != nil {
		return nil, err
	}
	templateName := strings.TrimSpace(input.TemplateName)
	if templateName == "" {
		return nil, ErrRuleTemplateNameInvalid
	}
	objectType, ok := normalizeObjectType(input.ObjectType)
	if !ok {
		return nil, ErrInvalidRuleObjectType
	}
	objectCategory := normalizeObjectCategory(input.ObjectCategory)
	if !isSupportedCategoryForObjectType(objectType, objectCategory) {
		return nil, ErrInvalidRuleObjectCategory
	}

	config := input.Config
	if strings.TrimSpace(config.RuleName) == "" {
		config.RuleName = templateName
	}
	config.Description = strings.TrimSpace(config.Description)
	modules, err := normalizeAndValidateModules(config.Modules)
	if err != nil {
		return nil, err
	}
	config.Modules = modules

	rawConfig, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to encode template config: %w", err)
	}

	operator := operatorID
	record := model.RuleTemplate{
		TemplateName:   templateName,
		ObjectType:     objectType,
		ObjectCategory: objectCategory,
		TemplateConfig: string(rawConfig),
		Description:    strings.TrimSpace(input.Description),
		IsSystem:       false,
		CreatedBy:      &operator,
		UpdatedBy:      &operator,
	}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, fmt.Errorf("failed to create rule template: %w", err)
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "create", "rule_templates", &targetID, map[string]any{
		"event":          "create_rule_template",
		"templateName":   record.TemplateName,
		"objectType":     record.ObjectType,
		"objectCategory": record.ObjectCategory,
	}, ipAddress, userAgent))

	return &RuleTemplateSummary{
		RuleTemplate: record,
		Config:       config,
	}, nil
}

func (s *RuleService) CreateTemplateFromRule(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	ruleID uint,
	templateName string,
	description string,
	ipAddress string,
	userAgent string,
) (*RuleTemplateSummary, error) {
	detail, err := s.GetRule(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	if err := requireRuleDimensionWriteScope(ctx, s.db, claims, detail.Rule.YearID, detail.Rule.ObjectType, detail.Rule.ObjectCategory); err != nil {
		return nil, err
	}
	modules := make([]RuleModuleInput, 0, len(detail.Modules))
	for _, item := range detail.Modules {
		modules = append(modules, mapModuleDetailToInput(item))
	}

	return s.CreateTemplate(ctx, claims, operatorID, CreateRuleTemplateInput{
		TemplateName:   templateName,
		ObjectType:     detail.Rule.ObjectType,
		ObjectCategory: detail.Rule.ObjectCategory,
		Description:    strings.TrimSpace(description),
		Config: RuleTemplateConfig{
			RuleName:    detail.Rule.RuleName,
			Description: detail.Rule.Description,
			Modules:     modules,
		},
	}, ipAddress, userAgent)
}

func (s *RuleService) ApplyTemplate(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	templateID uint,
	input ApplyRuleTemplateInput,
	ipAddress string,
	userAgent string,
) (*RuleDetail, error) {
	if templateID == 0 {
		return nil, ErrInvalidParam
	}
	dimension, err := normalizeRuleDimension(input.YearID, input.PeriodCode, input.ObjectType, input.ObjectCategory)
	if err != nil {
		return nil, err
	}
	if err := requireRuleDimensionWriteScope(ctx, s.db, claims, dimension.YearID, dimension.ObjectType, dimension.ObjectCategory); err != nil {
		return nil, err
	}

	var template model.RuleTemplate
	if err := s.db.WithContext(ctx).Where("id = ?", templateID).First(&template).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrRuleTemplateNotFound
		}
		return nil, fmt.Errorf("failed to query rule template: %w", err)
	}

	config := RuleTemplateConfig{}
	if err := json.Unmarshal([]byte(template.TemplateConfig), &config); err != nil {
		return nil, fmt.Errorf("failed to parse rule template config: %w", err)
	}
	modules, err := normalizeAndValidateModules(config.Modules)
	if err != nil {
		return nil, err
	}

	ruleName := strings.TrimSpace(input.RuleName)
	if ruleName == "" {
		ruleName = strings.TrimSpace(config.RuleName)
	}
	if ruleName == "" {
		ruleName = template.TemplateName
	}
	if ruleName == "" {
		return nil, ErrInvalidRuleName
	}

	description := strings.TrimSpace(input.Description)
	if description == "" {
		description = strings.TrimSpace(config.Description)
	}

	operator := operatorID
	baseRuleID := uint(0)
	periods := targetPeriods(dimension.PeriodCode, input.SyncQuarterly)
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, periodCode := range periods {
			if err := ensurePeriodConfigWritableTx(tx, dimension.YearID, periodCode); err != nil {
				return err
			}
			currentDimension := dimension
			currentDimension.PeriodCode = periodCode
			rule, err := s.upsertRuleByDimensionTx(tx, &operator, currentDimension, ruleName, description, input.IsActive, modules, upsertRuleMode{
				AllowCreate: true,
				AllowUpdate: input.Overwrite,
			})
			if err != nil {
				return err
			}
			if periodCode == dimension.PeriodCode {
				baseRuleID = rule.ID
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	result, err := s.loadRuleDetail(ctx, baseRuleID)
	if err != nil {
		return nil, err
	}
	targetID := result.Rule.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "create", "assessment_rules", &targetID, map[string]any{
		"event":         "apply_rule_template",
		"templateId":    templateID,
		"syncQuarterly": input.SyncQuarterly,
		"overwrite":     input.Overwrite,
	}, ipAddress, userAgent))

	return result, nil
}

func (s *RuleService) loadRuleDetail(ctx context.Context, ruleID uint) (*RuleDetail, error) {
	var rule model.AssessmentRule
	if err := s.db.WithContext(ctx).Where("id = ?", ruleID).First(&rule).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrRuleNotFound
		}
		return nil, fmt.Errorf("failed to query assessment rule: %w", err)
	}

	var modules []model.ScoreModule
	if err := s.db.WithContext(ctx).
		Where("rule_id = ?", ruleID).
		Order("sort_order ASC, id ASC").
		Find(&modules).Error; err != nil {
		return nil, fmt.Errorf("failed to query score modules: %w", err)
	}

	resultModules := make([]RuleModuleDetail, 0, len(modules))
	for _, item := range modules {
		var groups []model.VoteGroup
		if item.ModuleCode == "vote" {
			if err := s.db.WithContext(ctx).
				Where("module_id = ?", item.ID).
				Order("sort_order ASC, id ASC").
				Find(&groups).Error; err != nil {
				return nil, fmt.Errorf("failed to query vote groups: %w", err)
			}
		}
		resultModules = append(resultModules, RuleModuleDetail{
			ScoreModule: item,
			VoteGroups:  groups,
		})
	}

	return &RuleDetail{
		Rule:    rule,
		Modules: resultModules,
	}, nil
}

func (s *RuleService) upsertRuleByDimensionTx(
	tx *gorm.DB,
	operatorID *uint,
	dimension ruleDimension,
	ruleName string,
	description string,
	isActive bool,
	modules []RuleModuleInput,
	mode upsertRuleMode,
) (*model.AssessmentRule, error) {
	var rule model.AssessmentRule
	err := tx.Where("year_id = ? AND period_code = ? AND object_type = ? AND object_category = ?",
		dimension.YearID, dimension.PeriodCode, dimension.ObjectType, dimension.ObjectCategory).
		First(&rule).Error
	switch {
	case err == nil:
		if !mode.AllowUpdate {
			return nil, ErrRuleAlreadyExists
		}
		if err := s.replaceRuleConfigTx(tx, operatorID, rule.ID, ruleName, description, isActive, modules); err != nil {
			return nil, err
		}
		if err := tx.Where("id = ?", rule.ID).First(&rule).Error; err != nil {
			return nil, fmt.Errorf("failed to reload assessment rule after update: %w", err)
		}
		return &rule, nil
	case repository.IsRecordNotFound(err):
		if !mode.AllowCreate {
			return nil, ErrRuleNotFound
		}
		rule = model.AssessmentRule{
			YearID:         dimension.YearID,
			PeriodCode:     dimension.PeriodCode,
			ObjectType:     dimension.ObjectType,
			ObjectCategory: dimension.ObjectCategory,
			RuleName:       ruleName,
			Description:    description,
			IsActive:       isActive,
			CreatedBy:      operatorID,
			UpdatedBy:      operatorID,
		}
		if err := tx.Create(&rule).Error; err != nil {
			if isUniqueConstraintError(err) {
				return nil, ErrRuleAlreadyExists
			}
			return nil, fmt.Errorf("failed to create assessment rule: %w", err)
		}
		if err := createRuleModulesTx(tx, operatorID, rule.ID, modules); err != nil {
			return nil, err
		}
		return &rule, nil
	default:
		return nil, fmt.Errorf("failed to query assessment rule by dimension: %w", err)
	}
}

func (s *RuleService) replaceRuleConfigTx(
	tx *gorm.DB,
	operatorID *uint,
	ruleID uint,
	ruleName string,
	description string,
	isActive bool,
	modules []RuleModuleInput,
) error {
	if err := tx.Model(&model.AssessmentRule{}).
		Where("id = ?", ruleID).
		Updates(map[string]any{
			"rule_name":   ruleName,
			"description": description,
			"is_active":   isActive,
			"updated_by":  operatorID,
			"updated_at":  time.Now().Unix(),
		}).Error; err != nil {
		return fmt.Errorf("failed to update assessment rule: %w", err)
	}

	if err := tx.Where("rule_id = ?", ruleID).Delete(&model.ScoreModule{}).Error; err != nil {
		return fmt.Errorf("failed to delete score modules by rule: %w", err)
	}
	if err := createRuleModulesTx(tx, operatorID, ruleID, modules); err != nil {
		return err
	}
	return nil
}

func createRuleModulesTx(tx *gorm.DB, operatorID *uint, ruleID uint, modules []RuleModuleInput) error {
	for _, item := range modules {
		module := model.ScoreModule{
			RuleID:            ruleID,
			ModuleCode:        item.ModuleCode,
			ModuleKey:         item.ModuleKey,
			ModuleName:        item.ModuleName,
			Weight:            cloneFloat64Ptr(item.Weight),
			MaxScore:          cloneFloat64Ptr(item.MaxScore),
			CalculationMethod: item.CalculationMethod,
			Expression:        item.Expression,
			ContextScope:      item.ContextScope,
			SortOrder:         item.SortOrder,
			IsActive:          item.IsActive,
			CreatedBy:         operatorID,
			UpdatedBy:         operatorID,
		}
		if err := tx.Create(&module).Error; err != nil {
			if isUniqueConstraintError(err) {
				return ErrInvalidRuleModules
			}
			return fmt.Errorf("failed to create score module: %w", err)
		}

		for _, groupInput := range item.VoteGroups {
			group := model.VoteGroup{
				ModuleID:   module.ID,
				GroupCode:  groupInput.GroupCode,
				GroupName:  groupInput.GroupName,
				Weight:     groupInput.Weight,
				VoterType:  groupInput.VoterType,
				VoterScope: groupInput.VoterScope,
				MaxScore:   groupInput.MaxScore,
				SortOrder:  groupInput.SortOrder,
				IsActive:   groupInput.IsActive,
				CreatedBy:  operatorID,
				UpdatedBy:  operatorID,
			}
			if err := tx.Create(&group).Error; err != nil {
				if isUniqueConstraintError(err) {
					return ErrInvalidRuleModules
				}
				return fmt.Errorf("failed to create vote group: %w", err)
			}
		}
	}
	return nil
}
func normalizeRuleDimension(yearID uint, periodCode, objectType, objectCategory string) (ruleDimension, error) {
	if yearID == 0 {
		return ruleDimension{}, ErrInvalidParam
	}
	periodCode = normalizePeriodCode(periodCode)
	if !isValidPeriodCode(periodCode) {
		return ruleDimension{}, ErrInvalidRulePeriodCode
	}
	normalizedType, ok := normalizeObjectType(objectType)
	if !ok {
		return ruleDimension{}, ErrInvalidRuleObjectType
	}
	normalizedCategory := normalizeObjectCategory(objectCategory)
	if !isSupportedCategoryForObjectType(normalizedType, normalizedCategory) {
		return ruleDimension{}, ErrInvalidRuleObjectCategory
	}
	return ruleDimension{
		YearID:         yearID,
		PeriodCode:     periodCode,
		ObjectType:     normalizedType,
		ObjectCategory: normalizedCategory,
	}, nil
}

func normalizeAndValidateModules(modules []RuleModuleInput) ([]RuleModuleInput, error) {
	if len(modules) == 0 || len(modules) > 10 {
		return nil, ErrInvalidRuleModules
	}

	result := make([]RuleModuleInput, 0, len(modules))
	moduleKeys := make(map[string]struct{}, len(modules))
	weightedSum := 0.0

	for index, module := range modules {
		module.ModuleCode = strings.TrimSpace(module.ModuleCode)
		if _, ok := moduleCodeSet[module.ModuleCode]; !ok {
			return nil, ErrInvalidModuleCode
		}
		module.ModuleKey = strings.TrimSpace(module.ModuleKey)
		module.ModuleName = strings.TrimSpace(module.ModuleName)
		if module.ModuleKey == "" || module.ModuleName == "" {
			return nil, ErrInvalidRuleModules
		}
		if _, exists := moduleKeys[module.ModuleKey]; exists {
			return nil, ErrInvalidRuleModules
		}
		moduleKeys[module.ModuleKey] = struct{}{}

		if module.SortOrder == 0 {
			module.SortOrder = index + 1
		}
		module.CalculationMethod = strings.TrimSpace(module.CalculationMethod)
		module.Expression = strings.TrimSpace(module.Expression)
		module.ContextScope = normalizeJSONText(module.ContextScope)
		if module.ContextScope != "" && !json.Valid([]byte(module.ContextScope)) {
			return nil, ErrInvalidRuleModules
		}

		switch module.ModuleCode {
		case "extra":
			module.Weight = nil
			module.CalculationMethod = ""
			module.Expression = ""
			module.VoteGroups = nil
		case "direct":
			if module.Weight == nil || *module.Weight <= 0 || *module.Weight > 1 {
				return nil, ErrInvalidRuleModules
			}
			if module.MaxScore == nil || *module.MaxScore <= 0 {
				return nil, ErrInvalidRuleModules
			}
			module.VoteGroups = nil
			module.Expression = ""
			module.CalculationMethod = ""
			weightedSum += roundToScale(*module.Weight, 4)
		case "vote":
			if module.Weight == nil || *module.Weight <= 0 || *module.Weight > 1 {
				return nil, ErrInvalidRuleModules
			}
			if module.CalculationMethod == "" {
				module.CalculationMethod = "grade_mapping"
			}
			if module.CalculationMethod != "grade_mapping" {
				return nil, ErrInvalidRuleModules
			}
			if len(module.VoteGroups) == 0 {
				return nil, ErrInvalidRuleModules
			}
			groups, err := normalizeAndValidateVoteGroups(module.VoteGroups)
			if err != nil {
				return nil, err
			}
			module.VoteGroups = groups
			module.Expression = ""
			weightedSum += roundToScale(*module.Weight, 4)
		case "custom":
			if module.Weight == nil || *module.Weight <= 0 || *module.Weight > 1 {
				return nil, ErrInvalidRuleModules
			}
			if module.CalculationMethod == "" {
				module.CalculationMethod = "formula"
			}
			if module.CalculationMethod != "formula" {
				return nil, ErrInvalidRuleModules
			}
			if err := validateExpression(module.Expression); err != nil {
				return nil, err
			}
			module.VoteGroups = nil
			weightedSum += roundToScale(*module.Weight, 4)
		}
		result = append(result, module)
	}

	if math.Abs(roundToScale(weightedSum, 4)-1) > 0.00001 {
		return nil, ErrRuleWeightSumInvalid
	}
	return result, nil
}

func normalizeAndValidateVoteGroups(groups []RuleVoteGroupInput) ([]RuleVoteGroupInput, error) {
	result := make([]RuleVoteGroupInput, 0, len(groups))
	groupCodes := make(map[string]struct{}, len(groups))
	weightSum := 0.0

	for index, group := range groups {
		group.GroupCode = strings.TrimSpace(group.GroupCode)
		group.GroupName = strings.TrimSpace(group.GroupName)
		group.VoterType = strings.TrimSpace(group.VoterType)
		group.VoterScope = normalizeJSONText(group.VoterScope)

		if group.GroupCode == "" || group.GroupName == "" {
			return nil, ErrInvalidRuleModules
		}
		if _, exists := groupCodes[group.GroupCode]; exists {
			return nil, ErrInvalidRuleModules
		}
		groupCodes[group.GroupCode] = struct{}{}

		if _, ok := voterTypeSet[group.VoterType]; !ok {
			return nil, ErrInvalidRuleModules
		}
		if group.Weight <= 0 || group.Weight > 1 {
			return nil, ErrInvalidRuleModules
		}
		if group.MaxScore <= 0 {
			return nil, ErrInvalidRuleModules
		}
		if group.VoterScope != "" && !json.Valid([]byte(group.VoterScope)) {
			return nil, ErrInvalidRuleModules
		}
		if group.SortOrder == 0 {
			group.SortOrder = index + 1
		}
		weightSum += roundToScale(group.Weight, 4)
		result = append(result, group)
	}

	if math.Abs(roundToScale(weightSum, 4)-1) > 0.00001 {
		return nil, ErrVoteGroupWeightInvalid
	}
	return result, nil
}

func validateExpression(expression string) error {
	text := strings.TrimSpace(expression)
	if text == "" {
		return ErrInvalidExpression
	}
	if expressionInvalidCharPattern.MatchString(text) {
		return ErrInvalidExpression
	}
	if strings.ContainsAny(text, ";'\"`") {
		return ErrInvalidExpression
	}
	if !isParenthesesBalanced(text) {
		return ErrInvalidExpression
	}

	indices := expressionIdentifierPattern.FindAllStringIndex(text, -1)
	for _, item := range indices {
		token := text[item[0]:item[1]]
		isFunction := nextNonSpaceChar(text[item[1]:]) == '('
		if isFunction {
			if _, ok := expressionFuncSet[strings.ToLower(token)]; !ok {
				return ErrInvalidExpression
			}
			continue
		}

		if _, ok := expressionVarSet[token]; ok {
			continue
		}
		if strings.HasPrefix(token, "org.") && len(token) > len("org.") {
			continue
		}
		if strings.HasPrefix(token, "module_") && len(token) > len("module_") {
			continue
		}
		return ErrInvalidExpression
	}
	return nil
}

func nextNonSpaceChar(input string) byte {
	for i := 0; i < len(input); i++ {
		if input[i] == ' ' || input[i] == '\t' || input[i] == '\n' || input[i] == '\r' {
			continue
		}
		return input[i]
	}
	return 0
}

func isParenthesesBalanced(expression string) bool {
	balance := 0
	for i := 0; i < len(expression); i++ {
		switch expression[i] {
		case '(':
			balance++
		case ')':
			balance--
			if balance < 0 {
				return false
			}
		}
	}
	return balance == 0
}

func targetPeriods(periodCode string, syncQuarterly bool) []string {
	if syncQuarterly && isQuarterPeriod(periodCode) {
		return []string{"Q1", "Q2", "Q3", "Q4"}
	}
	return []string{periodCode}
}

func isQuarterPeriod(periodCode string) bool {
	switch normalizePeriodCode(periodCode) {
	case "Q1", "Q2", "Q3", "Q4":
		return true
	default:
		return false
	}
}

func roundToScale(value float64, scale int) float64 {
	pow := math.Pow10(scale)
	return math.Round(value*pow) / pow
}

func ensureAssessmentYearExists(tx *gorm.DB, yearID uint) error {
	var count int64
	if err := tx.Model(&model.AssessmentYear{}).Where("id = ?", yearID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to verify assessment year: %w", err)
	}
	if count == 0 {
		return ErrYearNotFound
	}
	return nil
}

func normalizeJSONText(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	return text
}

func cloneFloat64Ptr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func mapModuleDetailToInput(item RuleModuleDetail) RuleModuleInput {
	input := RuleModuleInput{
		ModuleCode:        item.ModuleCode,
		ModuleKey:         item.ModuleKey,
		ModuleName:        item.ModuleName,
		Weight:            cloneFloat64Ptr(item.Weight),
		MaxScore:          cloneFloat64Ptr(item.MaxScore),
		CalculationMethod: item.CalculationMethod,
		Expression:        item.Expression,
		ContextScope:      item.ContextScope,
		SortOrder:         item.SortOrder,
		IsActive:          item.IsActive,
	}
	if len(item.VoteGroups) > 0 {
		input.VoteGroups = make([]RuleVoteGroupInput, 0, len(item.VoteGroups))
		for _, group := range item.VoteGroups {
			input.VoteGroups = append(input.VoteGroups, RuleVoteGroupInput{
				GroupCode:  group.GroupCode,
				GroupName:  group.GroupName,
				Weight:     group.Weight,
				VoterType:  group.VoterType,
				VoterScope: group.VoterScope,
				MaxScore:   group.MaxScore,
				SortOrder:  group.SortOrder,
				IsActive:   group.IsActive,
			})
		}
	}
	return input
}
