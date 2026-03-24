package model

type AssessmentObjectModuleScore struct {
	ID           uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	AssessmentID uint    `gorm:"not null;index;uniqueIndex:uk_assessment_period_object_module,priority:1" json:"assessmentId"`
	PeriodCode   string  `gorm:"size:20;not null;uniqueIndex:uk_assessment_period_object_module,priority:2;index" json:"periodCode"`
	ObjectID     uint    `gorm:"not null;index;uniqueIndex:uk_assessment_period_object_module,priority:3" json:"objectId"`
	ModuleKey    string  `gorm:"size:120;not null;uniqueIndex:uk_assessment_period_object_module,priority:4;index" json:"moduleKey"`
	Score        float64 `gorm:"not null;default:0" json:"score"`
	DetailJSON   string  `gorm:"type:TEXT;not null;default:''" json:"detailJson,omitempty"`
	CreatedBy    *uint   `json:"createdBy,omitempty"`
	CreatedAt    int64   `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy    *uint   `json:"updatedBy,omitempty"`
	UpdatedAt    int64   `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (AssessmentObjectModuleScore) TableName() string {
	return "assessment_object_module_scores"
}
