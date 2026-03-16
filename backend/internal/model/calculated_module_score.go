package model

type CalculatedModuleScore struct {
	ID                uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	CalculatedScoreID uint    `gorm:"not null;index;uniqueIndex:uk_calc_module" json:"calculatedScoreId"`
	ModuleID          uint    `gorm:"not null;uniqueIndex:uk_calc_module" json:"moduleId"`
	ModuleCode        string  `gorm:"size:50;not null" json:"moduleCode"`
	ModuleKey         string  `gorm:"size:100;not null;index" json:"moduleKey"`
	ModuleName        string  `gorm:"size:100;not null" json:"moduleName"`
	SortOrder         int     `gorm:"not null;default:0;index" json:"sortOrder"`
	RawScore          float64 `gorm:"type:decimal(10,6);not null;default:0" json:"rawScore"`
	WeightedScore     float64 `gorm:"type:decimal(10,6);not null;default:0" json:"weightedScore"`
	ScoreDetail       string  `gorm:"type:text" json:"scoreDetail"`
	CreatedAt         int64   `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedAt         int64   `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (CalculatedModuleScore) TableName() string {
	return "calculated_module_scores"
}
