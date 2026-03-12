package model

type AssessmentObject struct {
	ID             uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	YearID         uint   `gorm:"not null;index;uniqueIndex:uk_year_target" json:"yearId"`
	ObjectType     string `gorm:"size:20;not null;index" json:"objectType"`
	ObjectCategory string `gorm:"size:50;not null;index" json:"objectCategory"`
	TargetID       uint   `gorm:"not null;uniqueIndex:uk_year_target" json:"targetId"`
	TargetType     string `gorm:"size:20;not null;index;uniqueIndex:uk_year_target" json:"targetType"`
	ObjectName     string `gorm:"size:200;not null" json:"objectName"`
	ParentObjectID *uint  `gorm:"index" json:"parentObjectId,omitempty"`
	IsActive       bool   `gorm:"not null;default:true;index" json:"isActive"`
	CreatedBy      *uint  `json:"createdBy,omitempty"`
	CreatedAt      int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy      *uint  `json:"updatedBy,omitempty"`
	UpdatedAt      int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (AssessmentObject) TableName() string {
	return "assessment_objects"
}
