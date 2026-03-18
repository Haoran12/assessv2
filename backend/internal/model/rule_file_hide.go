package model

type RuleFileHide struct {
	ID             uint  `gorm:"primaryKey;autoIncrement" json:"id"`
	RuleFileID     uint  `gorm:"not null;index;uniqueIndex:uk_rule_file_hide,priority:1" json:"ruleFileId"`
	OrganizationID uint  `gorm:"not null;index;uniqueIndex:uk_rule_file_hide,priority:2" json:"organizationId"`
	CreatedBy      *uint `json:"createdBy,omitempty"`
	CreatedAt      int64 `gorm:"not null;autoCreateTime" json:"createdAt"`
}

func (RuleFileHide) TableName() string {
	return "rule_file_hides"
}
