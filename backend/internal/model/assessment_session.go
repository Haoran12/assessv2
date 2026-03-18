package model

type AssessmentSession struct {
	ID             uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	AssessmentName string `gorm:"size:200;not null;uniqueIndex" json:"assessmentName"`
	DisplayName    string `gorm:"size:200;not null" json:"displayName"`
	Year           int    `gorm:"not null;index" json:"year"`
	OrganizationID uint   `gorm:"not null;index" json:"organizationId"`
	Description    string `gorm:"type:text" json:"description"`
	DataDir        string `gorm:"size:500;not null" json:"dataDir"`
	CreatedBy      *uint  `json:"createdBy,omitempty"`
	CreatedAt      int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy      *uint  `json:"updatedBy,omitempty"`
	UpdatedAt      int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (AssessmentSession) TableName() string {
	return "assessment_sessions"
}
