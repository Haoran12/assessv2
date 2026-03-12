package model

import "time"

type EmployeeHistory struct {
	ID                 uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	EmployeeID         uint      `gorm:"not null;index" json:"employeeId"`
	ChangeType         string    `gorm:"size:50;not null;index" json:"changeType"`
	OldOrganizationID  *uint     `gorm:"index" json:"oldOrganizationId,omitempty"`
	NewOrganizationID  *uint     `gorm:"index" json:"newOrganizationId,omitempty"`
	OldDepartmentID    *uint     `gorm:"index" json:"oldDepartmentId,omitempty"`
	NewDepartmentID    *uint     `gorm:"index" json:"newDepartmentId,omitempty"`
	OldPositionLevelID *uint     `gorm:"index" json:"oldPositionLevelId,omitempty"`
	NewPositionLevelID *uint     `gorm:"index" json:"newPositionLevelId,omitempty"`
	OldPositionTitle   string    `gorm:"size:100" json:"oldPositionTitle"`
	NewPositionTitle   string    `gorm:"size:100" json:"newPositionTitle"`
	ChangeReason       string    `gorm:"type:text" json:"changeReason"`
	EffectiveDate      time.Time `gorm:"type:date;not null" json:"effectiveDate"`
	CreatedBy          *uint     `json:"createdBy,omitempty"`
	CreatedAt          int64     `gorm:"not null;autoCreateTime" json:"createdAt"`
}

func (EmployeeHistory) TableName() string {
	return "employee_history"
}
