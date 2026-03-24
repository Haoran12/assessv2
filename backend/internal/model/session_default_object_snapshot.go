package model

type SessionDefaultObjectSnapshot struct {
	ID               uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	AssessmentID     uint   `gorm:"not null;index:idx_session_default_object_snapshot,priority:1" json:"assessmentId"`
	ObjectType       string `gorm:"size:20;not null" json:"objectType"`
	GroupCode        string `gorm:"size:80;not null" json:"groupCode"`
	TargetType       string `gorm:"size:20;not null" json:"targetType"`
	TargetID         uint   `gorm:"not null" json:"targetId"`
	ObjectName       string `gorm:"size:200;not null" json:"objectName"`
	ParentTargetType string `gorm:"size:20" json:"parentTargetType,omitempty"`
	ParentTargetID   uint   `gorm:"default:0" json:"parentTargetId,omitempty"`
	SortOrder        int    `gorm:"not null;default:0;index:idx_session_default_object_snapshot,priority:2" json:"sortOrder"`
	IsActive         bool   `gorm:"not null;default:true" json:"isActive"`
	CreatedAt        int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedAt        int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (SessionDefaultObjectSnapshot) TableName() string {
	return "session_default_object_snapshots"
}
