package model

type Ranking struct {
	ID                uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	YearID            uint    `gorm:"not null;index;uniqueIndex:uk_ranking_scope" json:"yearId"`
	PeriodCode        string  `gorm:"size:20;not null;index;uniqueIndex:uk_ranking_scope" json:"periodCode"`
	ObjectID          uint    `gorm:"not null;index;uniqueIndex:uk_ranking_scope" json:"objectId"`
	ObjectType        string  `gorm:"size:20;not null" json:"objectType"`
	ObjectCategory    string  `gorm:"size:50;not null" json:"objectCategory"`
	RankingScope      string  `gorm:"size:30;not null;index;uniqueIndex:uk_ranking_scope" json:"rankingScope"`
	ScopeKey          string  `gorm:"size:100;not null;index;uniqueIndex:uk_ranking_scope" json:"scopeKey"`
	RankNo            int     `gorm:"not null;index" json:"rankNo"`
	Score             float64 `gorm:"type:decimal(10,6);not null" json:"score"`
	TieBreakKey       string  `gorm:"type:text" json:"tieBreakKey"`
	CalculatedScoreID uint    `gorm:"not null;index" json:"calculatedScoreId"`
	CreatedAt         int64   `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedAt         int64   `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (Ranking) TableName() string {
	return "rankings"
}
