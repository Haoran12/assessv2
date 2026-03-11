package model

import "time"

type SystemConfig struct {
	Key       string    `gorm:"primaryKey;size:64" json:"key"`
	Value     string    `gorm:"type:text;not null" json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (SystemConfig) TableName() string {
	return "system_config"
}
