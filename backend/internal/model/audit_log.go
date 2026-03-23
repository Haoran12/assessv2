package model

type AuditLog struct {
	ID           uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       *uint  `gorm:"index" json:"userId,omitempty"`
	ActionType   string `gorm:"size:50;not null;index" json:"actionType"`
	TargetType   string `gorm:"size:50;index" json:"targetType"`
	TargetID     *uint  `gorm:"index" json:"targetId,omitempty"`
	EventCode    string `gorm:"size:100;index" json:"eventCode"`
	Summary      string `gorm:"size:300" json:"summary"`
	ChangeCount  int    `gorm:"not null;default:0" json:"changeCount"`
	HasDiff      bool   `gorm:"not null;default:false;index" json:"hasDiff"`
	ActionDetail string `gorm:"type:text" json:"actionDetail"`
	IPAddress    string `gorm:"size:50" json:"ipAddress"`
	UserAgent    string `gorm:"type:text" json:"userAgent"`
	CreatedAt    int64  `gorm:"not null;autoCreateTime;index" json:"createdAt"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}
