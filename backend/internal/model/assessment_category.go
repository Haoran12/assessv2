package model

type AssessmentCategory struct {
	ID           uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	CategoryCode string `gorm:"size:50;not null;uniqueIndex" json:"categoryCode"`
	CategoryName string `gorm:"size:100;not null" json:"categoryName"`
	ObjectType   string `gorm:"size:20;not null;index" json:"objectType"`
	SortOrder    int    `gorm:"not null;default:0" json:"sortOrder"`
	IsSystem     bool   `gorm:"not null;default:true" json:"isSystem"`
	Status       string `gorm:"size:20;not null;default:active;index" json:"status"`
	CreatedAt    int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedAt    int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (AssessmentCategory) TableName() string {
	return "assessment_categories"
}
