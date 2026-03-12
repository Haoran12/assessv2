package model

import "time"

type Employee struct {
	ID              uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	EmpName         string     `gorm:"size:100;not null" json:"empName"`
	OrganizationID  uint       `gorm:"not null;index" json:"organizationId"`
	DepartmentID    *uint      `gorm:"index" json:"departmentId,omitempty"`
	PositionLevelID uint       `gorm:"not null;index" json:"positionLevelId"`
	PositionTitle   string     `gorm:"size:100" json:"positionTitle"`
	HireDate        *time.Time `gorm:"type:date" json:"hireDate,omitempty"`
	Status          string     `gorm:"size:20;not null;default:active;index" json:"status"`
	CreatedBy       *uint      `json:"createdBy,omitempty"`
	CreatedAt       int64      `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy       *uint      `json:"updatedBy,omitempty"`
	UpdatedAt       int64      `gorm:"not null;autoUpdateTime" json:"updatedAt"`
	DeletedAt       *int64     `gorm:"index" json:"-"`
}

func (Employee) TableName() string {
	return "employees"
}
