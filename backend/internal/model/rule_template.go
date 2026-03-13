package model

type RuleTemplate struct {
	ID             uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateName   string `gorm:"size:200;not null" json:"templateName"`
	ObjectType     string `gorm:"size:20;not null;index" json:"objectType"`
	ObjectCategory string `gorm:"size:50;not null;index" json:"objectCategory"`
	TemplateConfig string `gorm:"type:text;not null" json:"templateConfig"`
	Description    string `gorm:"type:text" json:"description"`
	IsSystem       bool   `gorm:"not null;default:false;index" json:"isSystem"`
	PermissionMode uint16 `gorm:"not null;default:420" json:"permissionMode"` // 0644: Owner(RW), Group(R), Others(R)
	CreatedBy      *uint  `json:"createdBy,omitempty"`
	CreatedAt      int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedBy      *uint  `json:"updatedBy,omitempty"`
	UpdatedAt      int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (RuleTemplate) TableName() string {
	return "rule_templates"
}
