package model

type AssessmentObjectUserLink struct {
	ID                 uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID             uint   `gorm:"not null;index" json:"userId"`
	AssessmentObjectID uint   `gorm:"not null;index" json:"assessmentObjectId"`
	LinkType           string `gorm:"size:30;not null;default:member" json:"linkType"`
	AccessLevel        string `gorm:"size:20;not null;default:detail" json:"accessLevel"`
	IsPrimary          bool   `gorm:"not null;default:false;index" json:"isPrimary"`
	EffectiveFrom      *int64 `json:"effectiveFrom,omitempty"`
	EffectiveTo        *int64 `json:"effectiveTo,omitempty"`
	IsActive           bool   `gorm:"not null;default:true;index" json:"isActive"`
	CreatedBy          *uint  `json:"createdBy,omitempty"`
	CreatedAt          int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy          *uint  `json:"updatedBy,omitempty"`
	UpdatedAt          int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (AssessmentObjectUserLink) TableName() string {
	return "assessment_object_user_links"
}
