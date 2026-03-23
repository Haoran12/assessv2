package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"assessv2/backend/internal/api/response"
	"assessv2/backend/internal/model"
	"gorm.io/gorm"
)

func TestExprRuleSaveAllowsInvalidModuleExpression(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)
	sessionID := seedExprAssessmentFixture(t, db, buildExprRuleContentJSON(t, "direct_input", "", "", false))

	invalidRuleContent := buildExprRuleContentJSON(t, "custom_script", "1 +", "", false)
	body, _ := json.Marshal(map[string]any{
		"assessmentId": sessionID,
		"ruleName":     "Expr Rule With Invalid Module Script",
		"contentJson":  invalidRuleContent,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rules/files", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+rootToken)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status=200, got=%d body=%s", resp.Code, resp.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if envelope.Code != response.CodeSuccess {
		t.Fatalf("expected code=%d, got=%d body=%s", response.CodeSuccess, envelope.Code, resp.Body.String())
	}
}

func TestExprRuleSaveEnabledGradeInvalidExpressionReturns40003(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)
	sessionID := seedExprAssessmentFixture(t, db, buildExprRuleContentJSON(t, "direct_input", "", "", false))

	invalidRuleContent := buildExprRuleContentJSON(t, "direct_input", "", "1 + 1", true)
	body, _ := json.Marshal(map[string]any{
		"assessmentId": sessionID,
		"ruleName":     "Expr Rule With Invalid Enabled Grade Script",
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
	sessionID := seedExprAssessmentFixture(t, db, buildExprRuleContentJSON(t, "direct_input", "", "1 + 1", true))

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

func TestExprContextEndpoint(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)
	sessionID := seedExprAssessmentFixture(t, db, buildExprRuleContentJSON(t, "custom_script", `score("Q1", objectId)`, "", false))

	req := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/rules/expression-context?assessmentId=%d&periodCode=Q1&objectGroupCode=dept_main", sessionID),
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+rootToken)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status=200, got=%d body=%s", resp.Code, resp.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if envelope.Code != response.CodeSuccess {
		t.Fatalf("expected code=%d, got=%d body=%s", response.CodeSuccess, envelope.Code, resp.Body.String())
	}

	var payload struct {
		Functions []struct {
			Name string `json:"name"`
		} `json:"functions"`
		ModuleVariables []struct {
			Name string `json:"name"`
		} `json:"moduleVariables"`
		Periods []string `json:"periods"`
		Objects []struct {
			ObjectID       uint   `json:"objectId"`
			GroupCode      string `json:"groupCode"`
			ParentObjectID *uint  `json:"parentObjectId"`
			IsPriority     bool   `json:"isPriority"`
		} `json:"objects"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("failed to parse expression context payload: %v", err)
	}
	if len(payload.Functions) == 0 {
		t.Fatalf("expected functions in expression context")
	}
	hasScoreFn := false
	for _, item := range payload.Functions {
		if item.Name == "score" {
			hasScoreFn = true
			break
		}
	}
	if !hasScoreFn {
		t.Fatalf("expected score function in expression context, got=%v", payload.Functions)
	}
	if len(payload.ModuleVariables) == 0 || len(payload.Periods) == 0 || len(payload.Objects) == 0 {
		t.Fatalf("expected moduleVariables/periods/objects to be non-empty")
	}
	priorityCount := 0
	hasParentLink := false
	for _, object := range payload.Objects {
		if object.IsPriority {
			priorityCount++
		}
		if object.GroupCode == "dept_main" && object.ParentObjectID != nil && *object.ParentObjectID > 0 {
			hasParentLink = true
		}
	}
	if priorityCount == 0 {
		t.Fatalf("expected at least one priority object in expression context")
	}
	if !hasParentLink {
		t.Fatalf("expected dept_main object to expose parentObjectId in expression context")
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
	teamObject := model.AssessmentSessionObject{
		AssessmentID: session.ID,
		ObjectType:   "team",
		GroupCode:    "dept_team",
		TargetID:     11,
		TargetType:   "department",
		ObjectName:   "Expr Team",
		SortOrder:    1,
		IsActive:     true,
	}
	if err := db.Create(&teamObject).Error; err != nil {
		t.Fatalf("create team object failed: %v", err)
	}
	object := model.AssessmentSessionObject{
		AssessmentID:   session.ID,
		ObjectType:     "individual",
		GroupCode:      "dept_main",
		TargetID:       1,
		TargetType:     "employee",
		ObjectName:     "Expr User",
		ParentObjectID: &teamObject.ID,
		SortOrder:      2,
		IsActive:       true,
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
		FilePath:     filepath.Join(t.TempDir(), "expr-rule.json"),
		IsCopy:       false,
	}
	if err := db.Create(&ruleFile).Error; err != nil {
		t.Fatalf("create rule file failed: %v", err)
	}
	return session.ID
}

func buildExprRuleContentJSON(
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
						"extraConditionEnabled": extraConditionEnabled,
						"extraConditionScript":  gradeScript,
						"conditionLogic":        "or",
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
