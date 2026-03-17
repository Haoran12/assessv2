package service

import (
	"context"
	"testing"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

func TestCreateYearWithLegacyYearNameColumn(t *testing.T) {
	t.Setenv("ASSESS_DATA_ROOT", "")

	db := openIsolatedSQLiteTestDB(t)
	if err := createLegacyAssessmentSchema(db); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}

	svc := NewAssessmentService(db, repository.NewAuditRepository(db))
	input := CreateAssessmentYearInput{Year: 2028, Description: "legacy schema create"}
	claims := &auth.Claims{UserID: 1, Roles: []string{"root"}}

	result, err := svc.CreateYear(context.Background(), claims, 1, input, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("create year failed on legacy schema: %v", err)
	}
	if result == nil || result.Year.ID == 0 {
		t.Fatalf("expected created year with valid id")
	}
	if result.Year.Year != input.Year {
		t.Fatalf("expected year=%d, got=%d", input.Year, result.Year.Year)
	}
	if len(result.Periods) == 0 {
		t.Fatalf("expected generated periods for created year")
	}

	var yearName string
	if err := db.Raw("SELECT year_name FROM assessment_years WHERE id = ?", result.Year.ID).Scan(&yearName).Error; err != nil {
		t.Fatalf("query legacy year_name failed: %v", err)
	}
	if yearName == "" {
		t.Fatalf("expected legacy year_name to be populated")
	}
}

func createLegacyAssessmentSchema(db *gorm.DB) error {
	statements := []string{
		`CREATE TABLE assessment_years (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			year INTEGER NOT NULL UNIQUE,
			year_name VARCHAR(100) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'preparing',
			start_date DATE,
			end_date DATE,
			description TEXT,
			permission_mode SMALLINT NOT NULL DEFAULT 420,
			created_by INTEGER,
			created_at INTEGER NOT NULL,
			updated_by INTEGER,
			updated_at INTEGER NOT NULL
		);`,
		`CREATE TABLE assessment_periods (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			year_id INTEGER NOT NULL,
			period_code VARCHAR(20) NOT NULL,
			period_name VARCHAR(100) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'preparing',
			start_date DATE,
			end_date DATE,
			created_by INTEGER,
			created_at INTEGER NOT NULL,
			updated_by INTEGER,
			updated_at INTEGER NOT NULL,
			UNIQUE (year_id, period_code)
		);`,
		`CREATE TABLE assessment_objects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			year_id INTEGER NOT NULL,
			object_type VARCHAR(20) NOT NULL,
			object_category VARCHAR(50) NOT NULL,
			target_id INTEGER NOT NULL,
			target_type VARCHAR(20) NOT NULL,
			object_name VARCHAR(200) NOT NULL,
			parent_object_id INTEGER,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			created_by INTEGER,
			created_at INTEGER NOT NULL,
			updated_by INTEGER,
			updated_at INTEGER NOT NULL,
			UNIQUE (year_id, target_type, target_id)
		);`,
		`CREATE TABLE organizations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			org_name VARCHAR(200) NOT NULL,
			org_type VARCHAR(20) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'active',
			deleted_at INTEGER
		);`,
		`CREATE TABLE departments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			dept_name VARCHAR(200) NOT NULL,
			organization_id INTEGER NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'active',
			deleted_at INTEGER
		);`,
		`CREATE TABLE position_levels (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			level_code VARCHAR(50) NOT NULL
		);`,
		`CREATE TABLE employees (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			emp_name VARCHAR(100) NOT NULL,
			organization_id INTEGER NOT NULL,
			department_id INTEGER,
			position_level_id INTEGER NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'active',
			deleted_at INTEGER
		);`,
		`CREATE TABLE system_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			setting_key VARCHAR(100) NOT NULL UNIQUE,
			setting_value TEXT,
			setting_type VARCHAR(20) NOT NULL,
			description TEXT,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			updated_by INTEGER,
			updated_at INTEGER NOT NULL
		);`,
		`CREATE TABLE audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			action_type VARCHAR(50) NOT NULL,
			target_type VARCHAR(50),
			target_id INTEGER,
			action_detail TEXT,
			ip_address VARCHAR(45),
			user_agent TEXT,
			created_at INTEGER NOT NULL
		);`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
