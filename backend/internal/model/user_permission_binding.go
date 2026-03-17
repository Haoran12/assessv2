package model

type UserPermissionBinding struct {
	ID             uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         uint   `gorm:"not null;index" json:"userId"`
	RoleCode       string `gorm:"size:50;index" json:"roleCode"`
	ScopeOrgType   string `gorm:"size:20;index" json:"scopeOrgType"`
	ScopeOrgID     *uint  `gorm:"index" json:"scopeOrgId,omitempty"`
	PersonObjectID *uint  `gorm:"index" json:"personObjectId,omitempty"`
	TeamObjectID   *uint  `gorm:"index" json:"teamObjectId,omitempty"`
	IsPrimary      bool   `gorm:"not null;default:false;index" json:"isPrimary"`
	CreatedAt      int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedAt      int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (UserPermissionBinding) TableName() string {
	return "user_permission_bindings"
}
