package model

type UserOrganization struct {
	ID               uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID           uint   `gorm:"not null;index;uniqueIndex:uk_user_org" json:"userId"`
	OrganizationType string `gorm:"size:20;not null;index;uniqueIndex:uk_user_org" json:"organizationType"`
	OrganizationID   uint   `gorm:"not null;index;uniqueIndex:uk_user_org" json:"organizationId"`
	RoleInOrg        string `gorm:"size:50" json:"roleInOrg"`
	IsPrimary        bool   `gorm:"not null;default:false;index" json:"isPrimary"`
	CreatedBy        *uint  `json:"createdBy,omitempty"`
	CreatedAt        int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
}

func (UserOrganization) TableName() string {
	return "user_organizations"
}
