package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/database"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

func TestRuleManagementCreateRuleFileAllowsInvalidModuleScript(t *testing.T) {
	fixture := setupRuleManagementFixture(t)
	contentJSON := buildRuleManagementRuleContentJSON(t, "custom_script", "1 +", "", false)

	_, err := fixture.service.CreateRuleFile(
		context.Background(),
		fixture.claims,
		1,
		RuleFileInput{
			AssessmentID: fixture.sessionID,
			RuleName:     "Rule With Invalid Script",
			ContentJSON:  contentJSON,
		},
		"127.0.0.1",
		"unit-test",
	)
	if err != nil {
		t.Fatalf("expected invalid module script to be allowed, got=%v", err)
	}
}

func TestRuleManagementUpdateRuleFileValidateEnabledGradeExpressions(t *testing.T) {
	fixture := setupRuleManagementFixture(t)
	validContent := buildRuleManagementRuleContentJSON(t, "custom_script", "base + 1", "totalScore >= 90", true)

	created, err := fixture.service.CreateRuleFile(
		context.Background(),
		fixture.claims,
		1,
		RuleFileInput{
			AssessmentID: fixture.sessionID,
			RuleName:     "Valid Rule",
			ContentJSON:  validContent,
		},
		"127.0.0.1",
		"unit-test",
	)
	if err != nil {
		t.Fatalf("create rule file failed: %v", err)
	}

	invalidContent := buildRuleManagementRuleContentJSON(t, "custom_script", "base + 1", "1 + 1", true)
	_, err = fixture.service.UpdateRuleFile(
		context.Background(),
		fixture.claims,
		1,
		created.ID,
		RuleFileInput{
			AssessmentID: fixture.sessionID,
			RuleName:     "Valid Rule",
			ContentJSON:  invalidContent,
		},
		"127.0.0.1",
		"unit-test",
	)
	if !errors.Is(err, ErrInvalidExpression) {
		t.Fatalf("expected ErrInvalidExpression, got=%v", err)
	}
}

func TestRuleManagementCreateRuleFileAllowsLookupFunctions(t *testing.T) {
	fixture := setupRuleManagementFixture(t)
	contentJSON := buildRuleManagementRuleContentJSON(
		t,
		"custom_script",
		`score("Q1", objectId) + moduleScore("Q1", objectId, "base")`,
		`hasScore("Q1", objectId) && targetScore("Q1", "department", 1) >= 80`,
		true,
	)

	_, err := fixture.service.CreateRuleFile(
		context.Background(),
		fixture.claims,
		1,
		RuleFileInput{
			AssessmentID: fixture.sessionID,
			RuleName:     "Rule With Lookup Functions",
			ContentJSON:  contentJSON,
		},
		"127.0.0.1",
		"unit-test",
	)
	if err != nil {
		t.Fatalf("create rule file with lookup functions failed: %v", err)
	}
}

func TestRuleManagementCreateRuleFileStoresInAssessmentDir(t *testing.T) {
	fixture := setupRuleManagementFixture(t)
	contentJSON := buildRuleManagementRuleContentJSON(t, "direct_input", "", "", false)

	record, err := fixture.service.CreateRuleFile(
		context.Background(),
		fixture.claims,
		1,
		RuleFileInput{
			AssessmentID: fixture.sessionID,
			RuleName:     "Rule Storage Check",
			ContentJSON:  contentJSON,
		},
		"127.0.0.1",
		"unit-test",
	)
	if err != nil {
		t.Fatalf("create rule file failed: %v", err)
	}

	if strings.Contains(filepath.Clean(record.FilePath), filepath.Join("rules", "rule_test_assessment")) {
		t.Fatalf("expected rule file path under session dir, got=%s", record.FilePath)
	}

	expectedPrefix := filepath.Clean(filepath.Join(os.Getenv("ASSESS_DATA_ROOT"), "rule_test_assessment"))
	if !strings.HasPrefix(filepath.Clean(record.FilePath), expectedPrefix) {
		t.Fatalf("expected rule file path prefix=%s, got=%s", expectedPrefix, record.FilePath)
	}
	if _, statErr := os.Stat(record.FilePath); statErr != nil {
		t.Fatalf("expected rule file exists, got stat err=%v", statErr)
	}
}

func TestRuleManagementListRuleFilesDoesNotMigrateLegacyRulePathAtRuntime(t *testing.T) {
	fixture := setupRuleManagementFixture(t)
	legacyRoot := filepath.Join(os.Getenv("ASSESS_DATA_ROOT"), "rules", "rule_test_assessment")
	if err := os.MkdirAll(legacyRoot, 0o755); err != nil {
		t.Fatalf("create legacy rule dir failed: %v", err)
	}
	legacyPath := filepath.Join(legacyRoot, "legacy_rule.json")
	contentJSON := buildRuleManagementRuleContentJSON(t, "direct_input", "", "", false)
	if err := os.WriteFile(legacyPath, []byte(contentJSON), 0o644); err != nil {
		t.Fatalf("write legacy rule file failed: %v", err)
	}

	record := model.RuleFile{
		AssessmentID: fixture.sessionID,
		RuleName:     "Legacy Rule",
		ContentJSON:  contentJSON,
		FilePath:     legacyPath,
	}
	if err := fixture.sessionDB.Create(&record).Error; err != nil {
		t.Fatalf("create legacy rule file record failed: %v", err)
	}

	items, err := fixture.service.ListRuleFiles(
		context.Background(),
		fixture.claims,
		RuleFileListFilter{AssessmentID: fixture.sessionID},
	)
	if err != nil {
		t.Fatalf("list rule files failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 rule file, got=%d", len(items))
	}

	if filepath.Clean(items[0].FilePath) != filepath.Clean(legacyPath) {
		t.Fatalf("expected runtime path unchanged, got=%s want=%s", items[0].FilePath, legacyPath)
	}

	var dbRecord model.RuleFile
	if err := fixture.sessionDB.Where("id = ?", record.ID).First(&dbRecord).Error; err != nil {
		t.Fatalf("reload rule record failed: %v", err)
	}
	if filepath.Clean(dbRecord.FilePath) != filepath.Clean(legacyPath) {
		t.Fatalf("expected db path unchanged, got=%s want=%s", dbRecord.FilePath, legacyPath)
	}
}

type ruleManagementFixture struct {
	db        *gorm.DB
	sessionDB *gorm.DB
	service   *RuleManagementService
	claims    *auth.Claims
	sessionID uint
}

func setupRuleManagementFixture(t *testing.T) ruleManagementFixture {
	t.Helper()

	t.Setenv("ASSESS_DATA_ROOT", t.TempDir())

	db := openIsolatedSQLiteTestDB(t)
	if err := database.AutoMigrateAndSeed(db, "Test#123456"); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	organization := model.Organization{
		OrgName: "Rule Test Org",
		OrgType: "company",
		Status:  "active",
	}
	if err := db.Create(&organization).Error; err != nil {
		t.Fatalf("create organization failed: %v", err)
	}

	session := model.AssessmentSession{
		AssessmentName: "rule_test_assessment",
		DisplayName:    "Rule Test Assessment",
		Year:           2026,
		OrganizationID: organization.ID,
		DataDir:        "data/rule_test_assessment",
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create assessment session failed: %v", err)
	}
	sessionSummary := &AssessmentSessionSummary{AssessmentSession: session}
	sessionDB, closeSessionDB, err := openSessionBusinessDB(sessionSummary)
	if err != nil {
		t.Fatalf("open session business db failed: %v", err)
	}
	t.Cleanup(closeSessionDB)

	service := NewRuleManagementService(db, repository.NewAuditRepository(db))
	return ruleManagementFixture{
		db:        db,
		sessionDB: sessionDB,
		service:   service,
		claims:    &auth.Claims{Roles: []string{auth.RoleRoot}},
		sessionID: session.ID,
	}
}

func buildRuleManagementRuleContentJSON(
	t *testing.T,
	method string,
	moduleScript string,
	gradeScript string,
	extraConditionEnabled bool,
) string {
	t.Helper()
	payload := map[string]any{
		"version": 3,
		"scopedRules": []map[string]any{
			{
				"id":                     "scoped_1",
				"applicablePeriods":      []string{"Q1"},
				"applicableObjectGroups": []string{"dept"},
				"scoreModules": []map[string]any{
					{
						"id":                "base",
						"moduleKey":         "base",
						"moduleName":        "Base",
						"weight":            100,
						"calculationMethod": method,
						"customScript":      moduleScript,
					},
				},
				"grades": []map[string]any{
					{
						"id":    "grade_a",
						"title": "A",
						"scoreNode": map[string]any{
							"hasUpperLimit": true,
							"upperScore":    100,
							"upperOperator": "<=",
							"hasLowerLimit": true,
							"lowerScore":    90,
							"lowerOperator": ">=",
						},
						"extraConditionScript":  gradeScript,
						"extraConditionEnabled": extraConditionEnabled,
						"conditionLogic":        "and",
					},
				},
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal rule content failed: %v", err)
	}
	return string(raw)
}
