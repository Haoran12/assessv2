package service

import (
	"context"
	"database/sql"
	"testing"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

func TestCreateYearWithoutBusinessUserRecordStillSucceeds(t *testing.T) {
	t.Setenv("ASSESS_DATA_ROOT", "")

	db := openIsolatedSQLiteTestDB(t)
	if err := createSplitBusinessSchema(db); err != nil {
		t.Fatalf("create split-business schema: %v", err)
	}

	svc := NewAssessmentService(db, repository.NewAuditRepository(db))
	claims := &auth.Claims{UserID: 1, Roles: []string{"root"}}

	result, err := svc.CreateYear(
		context.Background(),
		claims,
		1, // operator exists in accounts DB but not in this business DB
		CreateAssessmentYearInput{Year: 2032, Description: "split-db fk compatibility"},
		"127.0.0.1",
		"test",
	)
	if err != nil {
		t.Fatalf("create year failed with split-db fk schema: %v", err)
	}
	if result == nil || result.Year.ID == 0 {
		t.Fatalf("expected created year with valid id")
	}

	var createdBy sql.NullInt64
	if err := db.Raw("SELECT created_by FROM assessment_years WHERE id = ?", result.Year.ID).Scan(&createdBy).Error; err != nil {
		t.Fatalf("query created_by failed: %v", err)
	}
	if createdBy.Valid {
		t.Fatalf("expected created_by NULL when business users table has no operator record, got=%d", createdBy.Int64)
	}
}

func createSplitBusinessSchema(db *gorm.DB) error {
	statements := []string{
		`PRAGMA foreign_keys = ON;`,
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username VARCHAR(50) NOT NULL
		);`,
		`CREATE TABLE assessment_years (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			year INTEGER NOT NULL UNIQUE,
			status VARCHAR(20) NOT NULL DEFAULT 'preparing',
			start_date DATE,
			end_date DATE,
			description TEXT,
			permission_mode SMALLINT NOT NULL DEFAULT 420,
			created_by INTEGER,
			created_at INTEGER NOT NULL,
			updated_by INTEGER,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY (created_by) REFERENCES users(id),
			FOREIGN KEY (updated_by) REFERENCES users(id)
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
			FOREIGN KEY (year_id) REFERENCES assessment_years(id) ON DELETE CASCADE,
			FOREIGN KEY (created_by) REFERENCES users(id),
			FOREIGN KEY (updated_by) REFERENCES users(id),
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
			FOREIGN KEY (year_id) REFERENCES assessment_years(id) ON DELETE CASCADE,
			FOREIGN KEY (created_by) REFERENCES users(id),
			FOREIGN KEY (updated_by) REFERENCES users(id),
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
			ip_address VARCHAR(50),
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
