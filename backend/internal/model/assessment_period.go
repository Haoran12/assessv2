package model

import "time"

type AssessmentPeriod struct {
	ID         uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	YearID     uint       `gorm:"not null;index;uniqueIndex:uk_year_period" json:"yearId"`
	PeriodCode string     `gorm:"size:20;not null;uniqueIndex:uk_year_period" json:"periodCode"`
	PeriodName string     `gorm:"size:100;not null" json:"periodName"`
	Status     string     `gorm:"size:20;not null;default:not_started;index" json:"status"`
	StartDate  *time.Time `gorm:"type:date" json:"startDate,omitempty"`
	EndDate    *time.Time `gorm:"type:date" json:"endDate,omitempty"`
	CreatedBy  *uint      `json:"createdBy,omitempty"`
	CreatedAt  int64      `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy  *uint      `json:"updatedBy,omitempty"`
	UpdatedAt  int64      `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (AssessmentPeriod) TableName() string {
	return "assessment_periods"
}
