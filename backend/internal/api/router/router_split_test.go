package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"assessv2/backend/internal/config"
	"assessv2/backend/internal/database"
	"assessv2/backend/internal/migration"
	"gorm.io/gorm"
)

func TestM7AuditLogsListWithSplitDatabases(t *testing.T) {
	engine, _, _ := setupSplitTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	updateBody, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{
				"settingKey":   "system.name",
				"settingValue": "AssessV2 Split Test",
			},
		},
	})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/system/settings", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+rootToken)
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp := httptest.NewRecorder()
	engine.ServeHTTP(updateResp, updateReq)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected settings update status=200, got=%d body=%s", updateResp.Code, updateResp.Body.String())
	}

	auditReq := httptest.NewRequest(http.MethodGet, "/api/system/audit-logs?targetType=system_settings&page=1&pageSize=20", nil)
	auditReq.Header.Set("Authorization", "Bearer "+rootToken)
	auditResp := httptest.NewRecorder()
	engine.ServeHTTP(auditResp, auditReq)
	if auditResp.Code != http.StatusOK {
		t.Fatalf("expected audit list status=200, got=%d body=%s", auditResp.Code, auditResp.Body.String())
	}

	var auditEnvelope apiEnvelope
	if err := json.Unmarshal(auditResp.Body.Bytes(), &auditEnvelope); err != nil {
		t.Fatalf("failed to parse audit list response: %v", err)
	}
	var auditPayload struct {
		Items []struct {
			ID       uint   `json:"id"`
			Username string `json:"username"`
		} `json:"items"`
	}
	if err := json.Unmarshal(auditEnvelope.Data, &auditPayload); err != nil {
		t.Fatalf("failed to parse audit list payload: %v", err)
	}
	if len(auditPayload.Items) == 0 {
		t.Fatalf("expected at least one audit log for system settings")
	}

	if auditPayload.Items[0].Username != "root" {
		t.Fatalf("expected first audit username=root, got=%s", auditPayload.Items[0].Username)
	}
}

func setupSplitTestServer(t *testing.T) (http.Handler, *gorm.DB, *gorm.DB) {
	t.Helper()

	tempDir := t.TempDir()
	cfg := config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 18080,
		},
		Database: config.DatabaseConfig{
			Path: filepath.Join(tempDir, "business.db"),
		},
		AccountsDatabasePath:      filepath.Join(tempDir, "accounts.db"),
		MigrationsDir:             "migrations",
		BusinessMigrationsDir:     filepath.Join("migrations", "business"),
		AccountsMigrationsDir:     filepath.Join("migrations", "accounts"),
		JWTSecret:                 "test-secret",
		DefaultPassword:           testDefaultPassword,
		EnforceMustChangePassword: true,
	}

	businessDB, err := database.NewSQLite(cfg.Database)
	if err != nil {
		t.Fatalf("failed to init business sqlite: %v", err)
	}
	accountsCfg := cfg.Database
	accountsCfg.Path = cfg.AccountsDatabasePath
	accountsDB, err := database.NewSQLite(accountsCfg)
	if err != nil {
		t.Fatalf("failed to init accounts sqlite: %v", err)
	}

	businessManager, err := migration.NewManager(businessDB, cfg.BusinessMigrationsDir)
	if err != nil {
		t.Fatalf("failed to init business migration manager: %v", err)
	}
	if _, err := businessManager.Up(t.Context()); err != nil {
		t.Fatalf("failed to apply business schema migrations: %v", err)
	}

	accountsManager, err := migration.NewManager(accountsDB, cfg.AccountsMigrationsDir)
	if err != nil {
		t.Fatalf("failed to init accounts migration manager: %v", err)
	}
	if _, err := accountsManager.Up(t.Context()); err != nil {
		t.Fatalf("failed to apply accounts schema migrations: %v", err)
	}

	if err := database.SeedAssessmentData(businessDB); err != nil {
		t.Fatalf("failed to init assessment seed data: %v", err)
	}
	if err := database.SeedAccountsData(accountsDB, cfg.DefaultPassword); err != nil {
		t.Fatalf("failed to init accounts seed data: %v", err)
	}

	businessSQLDB, err := businessDB.DB()
	if err != nil {
		t.Fatalf("failed to get business sql db: %v", err)
	}
	accountsSQLDB, err := accountsDB.DB()
	if err != nil {
		t.Fatalf("failed to get accounts sql db: %v", err)
	}
	t.Cleanup(func() {
		_ = businessSQLDB.Close()
		_ = accountsSQLDB.Close()
	})

	engine := NewWithDatabases(cfg, businessDB, accountsDB)
	return engine, businessDB, accountsDB
}
