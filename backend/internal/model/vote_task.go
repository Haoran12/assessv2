package model

type VoteTask struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	YearID      uint   `gorm:"not null;index;uniqueIndex:uk_vote_task" json:"yearId"`
	PeriodCode  string `gorm:"size:20;not null;index;uniqueIndex:uk_vote_task" json:"periodCode"`
	VoteGroupID uint   `gorm:"not null;index;uniqueIndex:uk_vote_task" json:"voteGroupId"`
	ObjectID    uint   `gorm:"not null;index;uniqueIndex:uk_vote_task" json:"objectId"`
	VoterID     uint   `gorm:"not null;index;uniqueIndex:uk_vote_task" json:"voterId"`
	Status      string `gorm:"size:20;not null;default:pending;index" json:"status"`
	CompletedAt *int64 `json:"completedAt,omitempty"`
	CreatedBy   *uint  `json:"createdBy,omitempty"`
	CreatedAt   int64  `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedAt   int64  `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (VoteTask) TableName() string {
	return "vote_tasks"
}
