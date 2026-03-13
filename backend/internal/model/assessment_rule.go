package model

type AssessmentRule struct {
	ID             uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	YearID         uint   `gorm:"not null;index;uniqueIndex:uk_rule_dimension" json:"yearId"`
	PeriodCode     string `gorm:"size:20;not null;index;uniqueIndex:uk_rule_dimension" json:"periodCode"`
	ObjectType     string `gorm:"size:20;not null;index;uniqueIndex:uk_rule_dimension" json:"objectType"`
	ObjectCategory string `gorm:"size:50;not null;index;uniqueIndex:uk_rule_dimension" json:"objectCategory"`
	RuleName       string `gorm:"size:200;not null" json:"ruleName"`
	Description    string `gorm:"type:text" json:"description"`
	IsActive       bool   `gorm:"not null;default:true;index" json:"isActive"`
	PermissionMode uint16 `gorm:"not null;default:420" json:"permissionMode"` // 0644: Owner(RW), Group(R), Others(R)
	CreatedBy      *uint  `json:"createdBy,omitempty"`
	CreatedAt      int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy      *uint  `json:"updatedBy,omitempty"`
	UpdatedAt      int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (AssessmentRule) TableName() string {
	return "assessment_rules"
}
