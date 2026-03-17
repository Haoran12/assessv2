package model

type RuleBinding struct {
	ID           uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	YearID       uint   `gorm:"not null;index:idx_rule_bindings_lookup,priority:1" json:"yearId"`
	PeriodCode   string `gorm:"size:20;not null;index:idx_rule_bindings_lookup,priority:2" json:"periodCode"`
	ObjectType   string `gorm:"size:20;not null;index:idx_rule_bindings_lookup,priority:3" json:"objectType"`
	SegmentCode  string `gorm:"size:80;not null;index:idx_rule_bindings_lookup,priority:4" json:"segmentCode"`
	OwnerScope   string `gorm:"size:30;not null;default:global;index:idx_rule_bindings_lookup,priority:5" json:"ownerScope"`
	OwnerOrgType string `gorm:"size:20;index:idx_rule_bindings_lookup,priority:6" json:"ownerOrgType"`
	OwnerOrgID   *uint  `gorm:"index:idx_rule_bindings_lookup,priority:7" json:"ownerOrgId,omitempty"`
	RuleID       uint   `gorm:"not null;index" json:"ruleId"`
	Priority     int    `gorm:"not null;default:0;index:idx_rule_bindings_lookup,priority:8" json:"priority"`
	Description  string `gorm:"type:text" json:"description"`
	IsActive     bool   `gorm:"not null;default:true;index:idx_rule_bindings_lookup,priority:9" json:"isActive"`
	CreatedBy    *uint  `json:"createdBy,omitempty"`
	CreatedAt    int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy    *uint  `json:"updatedBy,omitempty"`
	UpdatedAt    int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (RuleBinding) TableName() string {
	return "rule_bindings"
}
