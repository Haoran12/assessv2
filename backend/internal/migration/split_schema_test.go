package migration

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"assessv2/backend/internal/config"
	"assessv2/backend/internal/database"
	"gorm.io/gorm"
)

func TestSplitMigrationsApplyFromEmptyDatabases(t *testing.T) {
	businessDB := openMigrationTestDB(t, "business.db")
	accountsDB := openMigrationTestDB(t, "accounts.db")

	businessManager, err := NewManager(businessDB, migrationsDirFromRepo(t, "business"))
	if err != nil {
		t.Fatalf("init business migration manager failed: %v", err)
	}
	applied, err := businessManager.Up(context.Background())
	if err != nil {
		t.Fatalf("apply business migrations failed: %v", err)
	}
	if applied != 9 {
		t.Fatalf("expected business applied migrations=9, got=%d", applied)
	}

	accountsManager, err := NewManager(accountsDB, migrationsDirFromRepo(t, "accounts"))
	if err != nil {
		t.Fatalf("init accounts migration manager failed: %v", err)
	}
	applied, err = accountsManager.Up(context.Background())
	if err != nil {
		t.Fatalf("apply accounts migrations failed: %v", err)
	}
	if applied != 3 {
		t.Fatalf("expected accounts applied migrations=3, got=%d", applied)
	}

	assertTableExists(t, businessDB, "assessment_sessions", true)
	assertTableExists(t, businessDB, "assessment_session_periods", true)
	assertTableExists(t, businessDB, "assessment_object_groups", true)
	assertTableExists(t, businessDB, "assessment_session_objects", true)
	assertTableExists(t, businessDB, "assessment_object_module_scores", true)
	assertTableExists(t, businessDB, "rule_files", true)
	assertTableExists(t, businessDB, "rule_file_hides", false)
	assertTableExists(t, businessDB, "assessment_rule_bindings", false)
	assertTableExists(t, businessDB, "system_settings", true)
	assertTableExists(t, businessDB, "users", false)
	assertTableExists(t, businessDB, "roles", false)
	assertTableExists(t, businessDB, "user_roles", false)
	assertTableExists(t, businessDB, "user_organizations", false)
	assertTableExists(t, businessDB, "user_permission_bindings", false)

	assertTableExists(t, accountsDB, "users", true)
	assertTableExists(t, accountsDB, "roles", true)
	assertTableExists(t, accountsDB, "assessment_sessions", false)
	assertTableExists(t, accountsDB, "assessment_session_objects", false)
}

func TestBusinessSchemaHasNoUserForeignKeys(t *testing.T) {
	businessDB := openMigrationTestDB(t, "business_no_user_fk.db")
	manager, err := NewManager(businessDB, migrationsDirFromRepo(t, "business"))
	if err != nil {
		t.Fatalf("init business migration manager failed: %v", err)
	}
	if _, err := manager.Up(context.Background()); err != nil {
		t.Fatalf("apply business migrations failed: %v", err)
	}

	var count int64
	if err := businessDB.Raw(`
SELECT COUNT(1)
FROM sqlite_master
WHERE type = 'table'
  AND sql IS NOT NULL
  AND lower(sql) LIKE '%references users(id)%'
`).Scan(&count).Error; err != nil {
		t.Fatalf("query users foreign key reference failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no business table to reference users(id), found=%d", count)
	}
}

func assertTableExists(t *testing.T, db *gorm.DB, table string, expected bool) {
	t.Helper()

	var count int64
	if err := db.Raw("SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count).Error; err != nil {
		t.Fatalf("query sqlite_master table=%s failed: %v", table, err)
	}
	if exists := count > 0; exists != expected {
		t.Fatalf("table %s exists=%v, expected=%v", table, exists, expected)
	}
}

func migrationsDirFromRepo(t *testing.T, domain string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("resolve caller path failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations", domain)
}

func openMigrationTestDB(t *testing.T, fileName string) *gorm.DB {
	t.Helper()

	cfg := config.DatabaseConfig{
		Path: filepath.Join(t.TempDir(), fileName),
	}
	db, err := database.NewSQLite(cfg)
	if err != nil {
		t.Fatalf("open sqlite test db failed: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open sqlite sql.DB handle failed: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	return db
}
