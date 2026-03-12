package model

type VoteRecord struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID      uint   `gorm:"not null;uniqueIndex" json:"taskId"`
	GradeOption string `gorm:"size:20;not null;index" json:"gradeOption"`
	Remark      string `gorm:"type:text" json:"remark"`
	VotedAt     int64  `gorm:"not null" json:"votedAt"`
	CreatedAt   int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedAt   int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (VoteRecord) TableName() string {
	return "vote_records"
}
