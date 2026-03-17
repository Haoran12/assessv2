package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"assessv2/backend/internal/config"
	"assessv2/backend/internal/database"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()
	if strings.EqualFold(strings.TrimSpace(os.Getenv("ASSESS_ALLOW_MIGRATION_CHECKSUM_RECONCILE")), "true") {
		log.Fatalf("schema audit failed: ASSESS_ALLOW_MIGRATION_CHECKSUM_RECONCILE must not be enabled")
	}

	businessDB, err := database.NewSQLite(cfg.Database)
	if err != nil {
		log.Fatalf("open business database failed: %v", err)
	}

	accountsConfig := cfg.Database
	accountsConfig.Path = cfg.AccountsDatabasePath
	accountsDB, err := database.NewSQLite(accountsConfig)
	if err != nil {
		log.Fatalf("open accounts database failed: %v", err)
	}

	audit := newSchemaAudit(businessDB, accountsDB)
	if err := audit.run(); err != nil {
		log.Fatalf("schema audit failed: %v", err)
	}

	fmt.Println("schema audit passed")
}

type schemaAudit struct {
	businessDB *gorm.DB
	accountsDB *gorm.DB
}

func newSchemaAudit(businessDB, accountsDB *gorm.DB) *schemaAudit {
	return &schemaAudit{
		businessDB: businessDB,
		accountsDB: accountsDB,
	}
}

func (a *schemaAudit) run() error {
	businessForbiddenTables := []string{
		"users",
		"roles",
		"user_roles",
		"user_organizations",
		"user_permission_bindings",
	}
	for _, table := range businessForbiddenTables {
		if err := assertTableExists(a.businessDB, table, false); err != nil {
			return fmt.Errorf("business table constraint failed: %w", err)
		}
	}

	businessRequiredTables := []string{
		"assessment_years",
		"assessment_objects",
		"system_settings",
		"audit_logs",
	}
	for _, table := range businessRequiredTables {
		if err := assertTableExists(a.businessDB, table, true); err != nil {
			return fmt.Errorf("business schema incomplete: %w", err)
		}
	}

	accountsRequiredTables := []string{
		"users",
		"roles",
		"user_roles",
		"user_organizations",
		"user_permission_bindings",
		"audit_logs",
	}
	for _, table := range accountsRequiredTables {
		if err := assertTableExists(a.accountsDB, table, true); err != nil {
			return fmt.Errorf("accounts schema incomplete: %w", err)
		}
	}

	if err := a.assertBusinessHasNoUsersFK(); err != nil {
		return err
	}
	if err := a.assertNoLegacySettingKeys(); err != nil {
		return err
	}
	return nil
}

func (a *schemaAudit) assertBusinessHasNoUsersFK() error {
	var count int64
	if err := a.businessDB.Raw(`
SELECT COUNT(1)
FROM sqlite_master
WHERE type = 'table'
  AND sql IS NOT NULL
  AND lower(sql) LIKE '%references users(id)%'
`).Scan(&count).Error; err != nil {
		return fmt.Errorf("query users FK references failed: %w", err)
	}
	if count != 0 {
		return fmt.Errorf("business schema still contains %d table(s) that reference users(id)", count)
	}
	return nil
}

func (a *schemaAudit) assertNoLegacySettingKeys() error {
	if err := assertTableExists(a.businessDB, "system_settings", true); err != nil {
		return err
	}
	var count int64
	if err := a.businessDB.Raw(`
SELECT COUNT(1)
FROM system_settings
WHERE setting_key LIKE 'legacy.%'
   OR setting_key IN (
       'assessment.org_scopes',
       'assessment.permission_bindings_legacy_fallback'
   )
`).Scan(&count).Error; err != nil {
		return fmt.Errorf("query legacy setting keys failed: %w", err)
	}
	if count != 0 {
		return fmt.Errorf("legacy setting keys still exist in business system_settings: %d", count)
	}
	return nil
}

func assertTableExists(db *gorm.DB, table string, expected bool) error {
	var count int64
	if err := db.Raw("SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count).Error; err != nil {
		return fmt.Errorf("query sqlite_master table=%s failed: %w", table, err)
	}
	exists := count > 0
	if exists != expected {
		return fmt.Errorf("table %s exists=%v expected=%v", table, exists, expected)
	}
	return nil
}
