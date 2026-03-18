package model

type AssessmentObjectGroup struct {
	ID            uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	AssessmentID  uint   `gorm:"not null;index;uniqueIndex:uk_assessment_group_code,priority:1" json:"assessmentId"`
	ObjectType    string `gorm:"size:20;not null;index" json:"objectType"`
	GroupCode     string `gorm:"size:80;not null;uniqueIndex:uk_assessment_group_code,priority:2" json:"groupCode"`
	GroupName     string `gorm:"size:120;not null" json:"groupName"`
	SortOrder     int    `gorm:"not null;default:0;index" json:"sortOrder"`
	IsSystem      bool   `gorm:"not null;default:false" json:"isSystem"`
	CreatedBy     *uint  `json:"createdBy,omitempty"`
	CreatedAt     int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy     *uint  `json:"updatedBy,omitempty"`
	UpdatedAt     int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (AssessmentObjectGroup) TableName() string {
	return "assessment_object_groups"
}
