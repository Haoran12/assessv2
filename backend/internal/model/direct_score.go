package model

type DirectScore struct {
	ID         uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	YearID     uint    `gorm:"not null;index;uniqueIndex:uk_year_period_module_object" json:"yearId"`
	PeriodCode string  `gorm:"size:20;not null;index;uniqueIndex:uk_year_period_module_object" json:"periodCode"`
	ModuleID   uint    `gorm:"not null;index;uniqueIndex:uk_year_period_module_object" json:"moduleId"`
	ObjectID   uint    `gorm:"not null;index;uniqueIndex:uk_year_period_module_object" json:"objectId"`
	Score      float64 `gorm:"type:decimal(10,6);not null" json:"score"`
	Remark     string  `gorm:"type:text" json:"remark"`
	InputBy    uint    `gorm:"not null;index" json:"inputBy"`
	InputAt    int64   `gorm:"not null" json:"inputAt"`
	UpdatedBy  *uint   `json:"updatedBy,omitempty"`
	UpdatedAt  *int64  `gorm:"autoUpdateTime:false" json:"updatedAt,omitempty"`
}

func (DirectScore) TableName() string {
	return "direct_scores"
}
