package model

type Organization struct {
	ID        uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	OrgName   string `gorm:"size:200;not null" json:"orgName"`
	OrgType   string `gorm:"size:20;not null;index" json:"orgType"`
	ParentID  *uint  `gorm:"index" json:"parentId,omitempty"`
	LeaderID  *uint  `gorm:"index" json:"leaderId,omitempty"`
	SortOrder int    `gorm:"not null;default:0" json:"sortOrder"`
	Status    string `gorm:"size:20;not null;default:active;index" json:"status"`
	CreatedBy *uint  `json:"createdBy,omitempty"`
	CreatedAt int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy *uint  `json:"updatedBy,omitempty"`
	UpdatedAt int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
	DeletedAt *int64 `gorm:"index" json:"-"`
}

func (Organization) TableName() string {
	return "organizations"
}
