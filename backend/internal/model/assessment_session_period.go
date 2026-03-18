package model

type AssessmentSessionPeriod struct {
	ID           uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	AssessmentID uint   `gorm:"not null;index;uniqueIndex:uk_assessment_period_code,priority:1" json:"assessmentId"`
	PeriodCode   string `gorm:"size:20;not null;uniqueIndex:uk_assessment_period_code,priority:2" json:"periodCode"`
	PeriodName   string `gorm:"size:100;not null" json:"periodName"`
	SortOrder    int    `gorm:"not null;default:0;index" json:"sortOrder"`
	CreatedBy    *uint  `json:"createdBy,omitempty"`
	CreatedAt    int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy    *uint  `json:"updatedBy,omitempty"`
	UpdatedAt    int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (AssessmentSessionPeriod) TableName() string {
	return "assessment_session_periods"
}
