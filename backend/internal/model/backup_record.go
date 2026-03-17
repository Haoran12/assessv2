package model

type BackupRecord struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	BackupName  string `gorm:"size:200;not null" json:"backupName"`
	BackupPath  string `gorm:"size:500;not null" json:"backupPath"`
	BackupType  string `gorm:"size:20;not null;index" json:"backupType"`
	FileSize    int64  `json:"fileSize"`
	Description string `gorm:"type:text" json:"description"`
	CreatedBy   *uint  `gorm:"index" json:"createdBy,omitempty"`
	CreatedAt   int64  `gorm:"not null;autoCreateTime;index" json:"createdAt"`
}

func (BackupRecord) TableName() string {
	return "backups"
}
