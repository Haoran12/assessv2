package model

type ExtraPoint struct {
	ID             uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	YearID         uint    `gorm:"not null;index" json:"yearId"`
	PeriodCode     string  `gorm:"size:20;not null;index" json:"periodCode"`
	ObjectID       uint    `gorm:"not null;index" json:"objectId"`
	PointType      string  `gorm:"size:20;not null;index" json:"pointType"`
	Points         float64 `gorm:"type:decimal(10,6);not null" json:"points"`
	Reason         string  `gorm:"type:text;not null" json:"reason"`
	Evidence       string  `gorm:"type:text" json:"evidence"`
	ApprovedBy     *uint   `gorm:"index" json:"approvedBy,omitempty"`
	ApprovedAt     *int64  `json:"approvedAt,omitempty"`
	PermissionMode uint16  `gorm:"not null;default:416" json:"permissionMode"` // 0640: Owner(RW), Group(R), Others(none)
	InputBy        uint    `gorm:"not null;index" json:"inputBy"`
	InputAt        int64   `gorm:"not null" json:"inputAt"`
	UpdatedBy      *uint   `json:"updatedBy,omitempty"`
	UpdatedAt      *int64  `gorm:"autoUpdateTime:false" json:"updatedAt,omitempty"`
}

func (ExtraPoint) TableName() string {
	return "extra_points"
}
