package database

import (
	"fmt"

	"assessv2/backend/internal/model"
	"gorm.io/gorm"
)

// AutoMigrateAndSeed is used by tests to bootstrap schema quickly without SQL migrations.
func AutoMigrateAndSeed(db *gorm.DB, defaultPassword string) error {
	if err := db.AutoMigrate(
		&model.SystemSetting{},
		&model.User{},
		&model.Role{},
		&model.UserRole{},
		&model.UserOrganization{},
		&model.AuditLog{},
		&model.Organization{},
		&model.Department{},
		&model.PositionLevel{},
		&model.Employee{},
		&model.EmployeeHistory{},
		&model.AssessmentYear{},
		&model.AssessmentPeriod{},
		&model.AssessmentObject{},
		&model.AssessmentCategory{},
		&model.AssessmentRule{},
		&model.ScoreModule{},
		&model.VoteGroup{},
		&model.RuleTemplate{},
		&model.DirectScore{},
		&model.VoteTask{},
		&model.VoteRecord{},
		&model.ExtraPoint{},
		&model.CalculatedScore{},
		&model.CalculatedModuleScore{},
		&model.Ranking{},
	); err != nil {
		return fmt.Errorf("failed to run automigrate: %w", err)
	}
	return SeedBaselineData(db, defaultPassword)
}
