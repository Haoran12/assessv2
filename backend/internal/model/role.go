package model

type Role struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	RoleCode    string `gorm:"size:50;not null;uniqueIndex" json:"roleCode"`
	RoleName    string `gorm:"size:100;not null" json:"roleName"`
	Description string `gorm:"type:text" json:"description"`
	Permissions string `gorm:"type:text;not null" json:"permissions"`
	IsSystem    bool   `gorm:"not null;default:false" json:"isSystem"`
	CreatedAt   int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedAt   int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (Role) TableName() string {
	return "roles"
}
