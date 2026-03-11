package model

type UserRole struct {
	ID        uint  `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint  `gorm:"not null;index;uniqueIndex:uk_user_role" json:"userId"`
	RoleID    uint  `gorm:"not null;index;uniqueIndex:uk_user_role" json:"roleId"`
	IsPrimary bool  `gorm:"not null;default:false;index" json:"isPrimary"`
	CreatedBy *uint `json:"createdBy,omitempty"`
	CreatedAt int64 `gorm:"not null;autoCreateTime" json:"createdAt"`

	Role Role `gorm:"foreignKey:RoleID" json:"-"`
}

func (UserRole) TableName() string {
	return "user_roles"
}
