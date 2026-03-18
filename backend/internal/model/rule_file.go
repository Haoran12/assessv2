package model

type RuleFile struct {
	ID           uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	AssessmentID uint   `gorm:"not null;index" json:"assessmentId"`
	RuleName     string `gorm:"size:200;not null" json:"ruleName"`
	Description  string `gorm:"type:text" json:"description"`
	ContentJSON  string `gorm:"type:text;not null" json:"contentJson"`
	FilePath     string `gorm:"size:500;not null" json:"filePath"`
	IsCopy       bool   `gorm:"not null;default:false;index" json:"isCopy"`
	SourceRuleID *uint  `gorm:"index" json:"sourceRuleId,omitempty"`
	OwnerOrgID   *uint  `gorm:"index" json:"ownerOrgId,omitempty"`
	CreatedBy    *uint  `json:"createdBy,omitempty"`
	CreatedAt    int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy    *uint  `json:"updatedBy,omitempty"`
	UpdatedAt    int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (RuleFile) TableName() string {
	return "rule_files"
}
