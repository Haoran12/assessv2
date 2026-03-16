package model

type CalculatedScore struct {
	ID            uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	YearID        uint    `gorm:"not null;index;uniqueIndex:uk_calc_score" json:"yearId"`
	PeriodCode    string  `gorm:"size:20;not null;index;uniqueIndex:uk_calc_score" json:"periodCode"`
	ObjectID      uint    `gorm:"not null;index;uniqueIndex:uk_calc_score" json:"objectId"`
	RuleID        uint    `gorm:"not null;index" json:"ruleId"`
	WeightedScore float64 `gorm:"type:decimal(10,6);not null;default:0" json:"weightedScore"`
	ExtraPoints   float64 `gorm:"type:decimal(10,6);not null;default:0" json:"extraPoints"`
	FinalScore    float64 `gorm:"type:decimal(10,6);not null;default:0;index" json:"finalScore"`
	RankBasis     string  `gorm:"type:text" json:"rankBasis"`
	DetailJSON    string  `gorm:"type:text" json:"detailJson"`
	TriggerMode   string  `gorm:"size:20;not null;default:auto" json:"triggerMode"`
	TriggeredBy   *uint   `gorm:"index" json:"triggeredBy,omitempty"`
	CalculatedAt  int64   `gorm:"not null" json:"calculatedAt"`
	CreatedAt     int64   `gorm:"not null;autoCreateTime" json:"createdAt"`
	UpdatedAt     int64   `gorm:"not null;autoUpdateTime" json:"updatedAt"`
}

func (CalculatedScore) TableName() string {
	return "calculated_scores"
}
