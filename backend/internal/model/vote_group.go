package model

type VoteGroup struct {
	ID         uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	ModuleID   uint    `gorm:"not null;index;uniqueIndex:uk_module_group_code" json:"moduleId"`
	GroupCode  string  `gorm:"size:50;not null;uniqueIndex:uk_module_group_code" json:"groupCode"`
	GroupName  string  `gorm:"size:100;not null" json:"groupName"`
	Weight     float64 `gorm:"type:decimal(5,4);not null" json:"weight"`
	VoterType  string  `gorm:"size:50;not null;index" json:"voterType"`
	VoterScope string  `gorm:"type:text" json:"voterScope"`
	MaxScore   float64 `gorm:"type:decimal(10,6);not null" json:"maxScore"`
	SortOrder  int     `gorm:"not null;default:0;index" json:"sortOrder"`
	IsActive   bool    `gorm:"not null;default:true;index" json:"isActive"`
	CreatedBy  *uint   `json:"createdBy,omitempty"`
	CreatedAt  int64   `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy  *uint   `json:"updatedBy,omitempty"`
	UpdatedAt  int64   `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (VoteGroup) TableName() string {
	return "vote_groups"
}
