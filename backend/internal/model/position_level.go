package model

type PositionLevel struct {
	ID              uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	LevelCode       string `gorm:"size:50;not null;uniqueIndex" json:"levelCode"`
	LevelName       string `gorm:"size:100;not null" json:"levelName"`
	Description     string `gorm:"type:text" json:"description"`
	IsSystem        bool   `gorm:"not null;default:false" json:"isSystem"`
	IsForAssessment bool   `gorm:"not null;default:true" json:"isForAssessment"`
	SortOrder       int    `gorm:"not null;default:0" json:"sortOrder"`
	Status          string `gorm:"size:20;not null;default:active;index" json:"status"`
	CreatedBy       *uint  `json:"createdBy,omitempty"`
	CreatedAt       int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy       *uint  `json:"updatedBy,omitempty"`
	UpdatedAt       int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (PositionLevel) TableName() string {
	return "position_levels"
}
