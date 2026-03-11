package model

type SystemSetting struct {
	ID           uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	SettingKey   string `gorm:"size:100;not null;uniqueIndex" json:"settingKey"`
	SettingValue string `gorm:"type:text" json:"settingValue"`
	SettingType  string `gorm:"size:20;not null" json:"settingType"`
	Description  string `gorm:"type:text" json:"description"`
	IsSystem     bool   `gorm:"not null;default:false" json:"isSystem"`
	UpdatedBy    *uint  `json:"updatedBy"`
	UpdatedAt    int64  `gorm:"not null" json:"updatedAt"`
}

func (SystemSetting) TableName() string {
	return "system_settings"
}
