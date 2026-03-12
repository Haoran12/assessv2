package model

type Department struct {
	ID             uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	DeptName       string `gorm:"size:200;not null" json:"deptName"`
	OrganizationID uint   `gorm:"not null;index" json:"organizationId"`
	ParentDeptID   *uint  `gorm:"index" json:"parentDeptId,omitempty"`
	LeaderID       *uint  `gorm:"index" json:"leaderId,omitempty"`
	SortOrder      int    `gorm:"not null;default:0" json:"sortOrder"`
	Status         string `gorm:"size:20;not null;default:active;index" json:"status"`
	CreatedBy      *uint  `json:"createdBy,omitempty"`
	CreatedAt      int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy      *uint  `json:"updatedBy,omitempty"`
	UpdatedAt      int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
	DeletedAt      *int64 `gorm:"index" json:"-"`
}

func (Department) TableName() string {
	return "departments"
}
