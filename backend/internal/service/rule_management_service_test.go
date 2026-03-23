package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/database"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

func TestRuleManagementCreateRuleFileValidateExpressions(t *testing.T) {
	fixture := setupRuleManagementFixture(t)
	contentJSON := buildRuleManagementRuleContentJSON(t, "custom_script", "1 +", "")

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
	if !errors.Is(err, ErrInvalidExpression) {
		t.Fatalf("expected ErrInvalidExpression, got=%v", err)
	}
}

func TestRuleManagementUpdateRuleFileValidateExpressions(t *testing.T) {
	fixture := setupRuleManagementFixture(t)
	validContent := buildRuleManagementRuleContentJSON(t, "custom_script", "base + 1", "totalScore >= 90")

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

	invalidContent := buildRuleManagementRuleContentJSON(t, "custom_script", "base + 1", "1 + 1")
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

type ruleManagementFixture struct {
	db        *gorm.DB
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

	service := NewRuleManagementService(db, repository.NewAuditRepository(db))
	return ruleManagementFixture{
		db:        db,
		service:   service,
		claims:    &auth.Claims{Roles: []string{auth.RoleRoot}},
		sessionID: session.ID,
	}
}

func buildRuleManagementRuleContentJSON(t *testing.T, method string, moduleScript string, gradeScript string) string {
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
						"extraConditionScript": gradeScript,
						"conditionLogic":       "and",
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
