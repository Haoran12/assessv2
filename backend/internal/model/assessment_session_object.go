package model

type AssessmentSessionObject struct {
	ID            uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	AssessmentID  uint   `gorm:"not null;index;uniqueIndex:uk_assessment_target,priority:1" json:"assessmentId"`
	ObjectType    string `gorm:"size:20;not null;index" json:"objectType"`
	GroupCode     string `gorm:"size:80;not null;index" json:"groupCode"`
	TargetID      uint   `gorm:"not null;uniqueIndex:uk_assessment_target,priority:2" json:"targetId"`
	TargetType    string `gorm:"size:20;not null;index;uniqueIndex:uk_assessment_target,priority:3" json:"targetType"`
	ObjectName    string `gorm:"size:200;not null" json:"objectName"`
	ParentObjectID *uint `gorm:"index" json:"parentObjectId,omitempty"`
	SortOrder     int    `gorm:"not null;default:0;index" json:"sortOrder"`
	IsActive      bool   `gorm:"not null;default:true;index" json:"isActive"`
	CreatedBy     *uint  `json:"createdBy,omitempty"`
	CreatedAt     int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy     *uint  `json:"updatedBy,omitempty"`
	UpdatedAt     int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (AssessmentSessionObject) TableName() string {
	return "assessment_session_objects"
}
