package model

type ScoreModule struct {
	ID                uint     `gorm:"primaryKey;autoIncrement" json:"id"`
	RuleID            uint     `gorm:"not null;index;uniqueIndex:uk_rule_module_key" json:"ruleId"`
	ModuleCode        string   `gorm:"size:50;not null;index" json:"moduleCode"`
	ModuleKey         string   `gorm:"size:100;not null;uniqueIndex:uk_rule_module_key" json:"moduleKey"`
	ModuleName        string   `gorm:"size:100;not null" json:"moduleName"`
	Weight            *float64 `gorm:"type:decimal(5,4)" json:"weight,omitempty"`
	MaxScore          *float64 `gorm:"type:decimal(10,6)" json:"maxScore,omitempty"`
	CalculationMethod string   `gorm:"size:50" json:"calculationMethod"`
	Expression        string   `gorm:"type:text" json:"expression"`
	ContextScope      string   `gorm:"type:text" json:"contextScope"`
	SortOrder         int      `gorm:"not null;default:0;index" json:"sortOrder"`
	IsActive          bool     `gorm:"not null;default:true;index" json:"isActive"`
	CreatedBy         *uint    `json:"createdBy,omitempty"`
	CreatedAt         int64    `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy         *uint    `json:"updatedBy,omitempty"`
	UpdatedAt         int64    `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (ScoreModule) TableName() string {
	return "score_modules"
}
