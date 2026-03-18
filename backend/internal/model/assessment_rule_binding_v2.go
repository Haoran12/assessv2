package model

type AssessmentRuleBindingV2 struct {
	ID              uint  `gorm:"primaryKey;autoIncrement" json:"id"`
	AssessmentID    uint  `gorm:"not null;index;uniqueIndex:uk_assessment_rule_binding,priority:1" json:"assessmentId"`
	PeriodCode      string `gorm:"size:20;not null;index;uniqueIndex:uk_assessment_rule_binding,priority:2" json:"periodCode"`
	ObjectGroupCode string `gorm:"size:80;not null;index;uniqueIndex:uk_assessment_rule_binding,priority:3" json:"objectGroupCode"`
	OrganizationID  uint  `gorm:"not null;index;uniqueIndex:uk_assessment_rule_binding,priority:4" json:"organizationId"`
	RuleFileID      uint  `gorm:"not null;index" json:"ruleFileId"`
	CreatedBy       *uint `json:"createdBy,omitempty"`
	CreatedAt       int64 `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy       *uint `json:"updatedBy,omitempty"`
	UpdatedAt       int64 `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (AssessmentRuleBindingV2) TableName() string {
	return "assessment_rule_bindings"
}
