package model

type BackupRecord struct {
	ID             uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	BackupName     string `gorm:"size:200;not null" json:"backupName"`
	BackupPath     string `gorm:"size:500;not null" json:"backupPath"`
	BackupType     string `gorm:"size:20;not null;index" json:"backupType"`
	ContentType    string `gorm:"size:20;not null;default:full_snapshot;index" json:"contentType"`
	ScopeType      string `gorm:"size:20;not null;default:global;index" json:"scopeType"`
	ScopeOrgID     *uint  `gorm:"index" json:"scopeOrgId,omitempty"`
	FormatVersion  string `gorm:"size:20" json:"formatVersion"`
	ChecksumSHA256 string `gorm:"size:64" json:"checksumSha256"`
	ManifestJSON   string `gorm:"type:text" json:"manifestJson"`
	FileSize       int64  `json:"fileSize"`
	Description    string `gorm:"type:text" json:"description"`
	CreatedBy      *uint  `gorm:"index" json:"createdBy,omitempty"`
	CreatedAt      int64  `gorm:"not null;autoCreateTime;index" json:"createdAt"`
}

func (BackupRecord) TableName() string {
	return "backups"
}
