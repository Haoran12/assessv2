package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"assessv2/backend/internal/api/response"
	"assessv2/backend/internal/model"
	"gorm.io/gorm"
)

func TestExprRuleSaveInvalidExpressionReturns40003(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)
	sessionID := seedExprAssessmentFixture(t, db, buildExprRuleContentJSON(t, "direct_input", "", ""))

	invalidRuleContent := buildExprRuleContentJSON(t, "custom_script", "1 +", "")
	body, _ := json.Marshal(map[string]any{
		"assessmentId": sessionID,
		"ruleName":     "Invalid Expr Rule",
		"contentJson":  invalidRuleContent,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rules/files", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+rootToken)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status=400, got=%d body=%s", resp.Code, resp.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if envelope.Code != response.CodeBadRequestBusinessRule {
		t.Fatalf("expected code=%d, got=%d body=%s", response.CodeBadRequestBusinessRule, envelope.Code, resp.Body.String())
	}
}

func TestExprCalculatedObjectsEvalErrorReturns40003(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)
	sessionID := seedExprAssessmentFixture(t, db, buildExprRuleContentJSON(t, "custom_script", "unknown_value + 1", ""))

	req := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/assessment/sessions/%d/calculated-objects?periodCode=Q1&objectGroupCode=dept_main", sessionID),
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+rootToken)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status=400, got=%d body=%s", resp.Code, resp.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if envelope.Code != response.CodeBadRequestBusinessRule {
		t.Fatalf("expected code=%d, got=%d body=%s", response.CodeBadRequestBusinessRule, envelope.Code, resp.Body.String())
	}
}

func seedExprAssessmentFixture(t *testing.T, db *gorm.DB, ruleContent string) uint {
	t.Helper()

	org := model.Organization{
		OrgName: "Expr API Org",
		OrgType: "company",
		Status:  "active",
	}
	if err := db.Create(&org).Error; err != nil {
		t.Fatalf("create organization failed: %v", err)
	}
	session := model.AssessmentSession{
		AssessmentName: "expr_api_assessment",
		DisplayName:    "Expr API Assessment",
		Year:           2026,
		OrganizationID: org.ID,
		DataDir:        "data/expr_api_assessment",
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create session failed: %v", err)
	}
	period := model.AssessmentSessionPeriod{
		AssessmentID:   session.ID,
		PeriodCode:     "Q1",
		PeriodName:     "Q1",
		RuleBindingKey: "Q1",
		SortOrder:      1,
	}
	if err := db.Create(&period).Error; err != nil {
		t.Fatalf("create period failed: %v", err)
	}
	object := model.AssessmentSessionObject{
		AssessmentID: session.ID,
		ObjectType:   "individual",
		GroupCode:    "dept_main",
		TargetID:     1,
		TargetType:   "employee",
		ObjectName:   "Expr User",
		SortOrder:    1,
		IsActive:     true,
	}
	if err := db.Create(&object).Error; err != nil {
		t.Fatalf("create object failed: %v", err)
	}
	moduleScore := model.AssessmentObjectModuleScore{
		AssessmentID: session.ID,
		PeriodCode:   "Q1",
		ObjectID:     object.ID,
		ModuleKey:    "base_performance",
		Score:        80,
	}
	if err := db.Create(&moduleScore).Error; err != nil {
		t.Fatalf("create module score failed: %v", err)
	}
	ruleFile := model.RuleFile{
		AssessmentID: session.ID,
		RuleName:     "Expr Rule",
		ContentJSON:  ruleContent,
		FilePath:     "rules/expr-rule.json",
		IsCopy:       false,
	}
	if err := db.Create(&ruleFile).Error; err != nil {
		t.Fatalf("create rule file failed: %v", err)
	}
	return session.ID
}

func buildExprRuleContentJSON(t *testing.T, method string, moduleScript string, gradeScript string) string {
	t.Helper()
	payload := map[string]any{
		"version": 3,
		"scopedRules": []map[string]any{
			{
				"id":                     "expr_scoped",
				"applicablePeriods":      []string{"Q1"},
				"applicableObjectGroups": []string{"dept_main"},
				"scoreModules": []map[string]any{
					{
						"id":                "base_performance",
						"moduleKey":         "base_performance",
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
							"hasLowerLimit": false,
							"lowerScore":    0,
							"lowerOperator": ">=",
						},
						"extraConditionScript": gradeScript,
						"conditionLogic":       "or",
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
