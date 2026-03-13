package model

import "time"

type AssessmentYear struct {
	ID             uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	Year           int        `gorm:"not null;uniqueIndex" json:"year"`
	YearName       string     `gorm:"size:100;not null" json:"yearName"`
	Status         string     `gorm:"size:20;not null;default:preparing;index" json:"status"`
	StartDate      *time.Time `gorm:"type:date" json:"startDate,omitempty"`
	EndDate        *time.Time `gorm:"type:date" json:"endDate,omitempty"`
	Description    string     `gorm:"type:text" json:"description"`
	PermissionMode uint16     `gorm:"not null;default:420" json:"permissionMode"` // 0644: Owner(RW), Group(R), Others(R)
	CreatedBy      *uint      `json:"createdBy,omitempty"`
	CreatedAt      int64      `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy      *uint      `json:"updatedBy,omitempty"`
	UpdatedAt      int64      `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (AssessmentYear) TableName() string {
	return "assessment_years"
}
