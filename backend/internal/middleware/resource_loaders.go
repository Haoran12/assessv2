package middleware

import (
	"context"

	"assessv2/backend/internal/model"
	"gorm.io/gorm"
)

// Common resource loaders for different resource types

// LoadAssessmentYearResource loads assessment year resource information
func LoadAssessmentYearResource(ctx context.Context, db *gorm.DB, yearID uint) (*ResourceInfo, error) {
	var year model.AssessmentYear
	if err := db.WithContext(ctx).First(&year, yearID).Error; err != nil {
		return nil, err
	}

	ownerID := uint(0)
	if year.CreatedBy != nil {
		ownerID = *year.CreatedBy
	}

	return &ResourceInfo{
		OwnerID:        ownerID,
		PermissionMode: year.PermissionMode,
		OrgType:        "",  // Assessment years are global, not org-specific
		OrgID:          0,
	}, nil
}

// LoadAssessmentRuleResource loads assessment rule resource information
func LoadAssessmentRuleResource(ctx context.Context, db *gorm.DB, ruleID uint) (*ResourceInfo, error) {
	var rule model.AssessmentRule
	if err := db.WithContext(ctx).First(&rule, ruleID).Error; err != nil {
		return nil, err
	}

	ownerID := uint(0)
	if rule.CreatedBy != nil {
		ownerID = *rule.CreatedBy
	}

	return &ResourceInfo{
		OwnerID:        ownerID,
		PermissionMode: rule.PermissionMode,
		OrgType:        "",
		OrgID:          0,
	}, nil
}

// LoadDirectScoreResource loads direct score resource information
func LoadDirectScoreResource(ctx context.Context, db *gorm.DB, scoreID uint) (*ResourceInfo, error) {
	var score model.DirectScore
	if err := db.WithContext(ctx).First(&score, scoreID).Error; err != nil {
		return nil, err
	}

	return &ResourceInfo{
		OwnerID:        score.InputBy,
		PermissionMode: score.PermissionMode,
		OrgType:        "",
		OrgID:          0,
	}, nil
}

// LoadExtraPointResource loads extra point resource information
func LoadExtraPointResource(ctx context.Context, db *gorm.DB, pointID uint) (*ResourceInfo, error) {
	var point model.ExtraPoint
	if err := db.WithContext(ctx).First(&point, pointID).Error; err != nil {
		return nil, err
	}

	return &ResourceInfo{
		OwnerID:        point.InputBy,
		PermissionMode: point.PermissionMode,
		OrgType:        "",
		OrgID:          0,
	}, nil
}

// LoadRuleTemplateResource loads rule template resource information
func LoadRuleTemplateResource(ctx context.Context, db *gorm.DB, templateID uint) (*ResourceInfo, error) {
	var template model.RuleTemplate
	if err := db.WithContext(ctx).First(&template, templateID).Error; err != nil {
		return nil, err
	}

	ownerID := uint(0)
	if template.CreatedBy != nil {
		ownerID = *template.CreatedBy
	}

	return &ResourceInfo{
		OwnerID:        ownerID,
		PermissionMode: template.PermissionMode,
		OrgType:        "",
		OrgID:          0,
	}, nil
}
