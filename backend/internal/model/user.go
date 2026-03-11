package model

type User struct {
	ID                 uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	Username           string  `gorm:"size:50;not null;uniqueIndex" json:"username"`
	PasswordHash       string  `gorm:"size:255;not null" json:"-"`
	RealName           string  `gorm:"size:100;not null" json:"realName"`
	Email              *string `gorm:"size:100" json:"email,omitempty"`
	Phone              *string `gorm:"size:20" json:"phone,omitempty"`
	Status             string  `gorm:"size:20;not null;default:active" json:"status"`
	MustChangePassword bool    `gorm:"not null;default:false" json:"mustChangePassword"`
	LastLoginAt        *int64  `json:"lastLoginAt,omitempty"`
	LastLoginIP        *string `gorm:"size:50" json:"lastLoginIp,omitempty"`
	CreatedAt          int64   `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedAt          int64   `gorm:"not null;autoUpdateTime" json:"updatedAt"`
	DeletedAt          *int64  `gorm:"index" json:"-"`

	UserRoles         []UserRole         `gorm:"foreignKey:UserID" json:"-"`
	UserOrganizations []UserOrganization `gorm:"foreignKey:UserID" json:"-"`
}

func (User) TableName() string {
	return "users"
}
