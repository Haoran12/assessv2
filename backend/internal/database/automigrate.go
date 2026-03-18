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
		&model.BackupRecord{},
		&model.User{},
		&model.Role{},
		&model.UserRole{},
		&model.UserOrganization{},
		&model.UserPermissionBinding{},
		&model.AuditLog{},
		&model.Organization{},
		&model.Department{},
		&model.PositionLevel{},
		&model.Employee{},
		&model.EmployeeHistory{},
		&model.AssessmentSession{},
		&model.AssessmentSessionPeriod{},
		&model.AssessmentObjectGroup{},
		&model.AssessmentSessionObject{},
		&model.RuleFile{},
		&model.RuleFileHide{},
		&model.AssessmentRuleBindingV2{},
	); err != nil {
		return fmt.Errorf("failed to run automigrate: %w", err)
	}
	return SeedBaselineData(db, defaultPassword)
}
