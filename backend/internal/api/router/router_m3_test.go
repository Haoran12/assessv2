package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestM3CreateRuleAndQuarterSync(t *testing.T) {
	engine, _ := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	yearID := createAssessmentYearForTest(t, engine, rootToken, 2093)

	createRuleBody, _ := json.Marshal(map[string]any{
		"yearId":         yearID,
		"periodCode":     "Q1",
		"objectType":     "team",
		"objectCategory": "company",
		"ruleName":       "Team Quarterly Rule",
		"isActive":       true,
		"syncQuarterly":  true,
		"modules": []map[string]any{
			{
				"moduleCode": "direct",
				"moduleKey":  "direct_base",
				"moduleName": "Direct Input",
				"weight":     0.6,
				"maxScore":   100,
				"isActive":   true,
			},
			{
				"moduleCode": "vote",
				"moduleKey":  "vote_base",
				"moduleName": "Vote",
				"weight":     0.4,
				"isActive":   true,
				"voteGroups": []map[string]any{
					{
						"groupCode":  "group_leader",
						"groupName":  "Group Leader",
						"weight":     1.0,
						"voterType":  "group_leader",
						"maxScore":   100,
						"isActive":   true,
						"voterScope": map[string]any{"organization_ids": []int{1}},
					},
				},
			},
		},
	})

	createRuleReq := httptest.NewRequest(http.MethodPost, "/api/rules", bytes.NewReader(createRuleBody))
	createRuleReq.Header.Set("Authorization", "Bearer "+rootToken)
	createRuleReq.Header.Set("Content-Type", "application/json")
	createRuleResp := httptest.NewRecorder()
	engine.ServeHTTP(createRuleResp, createRuleReq)
	if createRuleResp.Code != http.StatusOK {
		t.Fatalf("expected create rule status=200, got=%d body=%s", createRuleResp.Code, createRuleResp.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/rules?yearId=%d&objectType=team&objectCategory=company", yearID), nil)
	listReq.Header.Set("Authorization", "Bearer "+rootToken)
	listResp := httptest.NewRecorder()
	engine.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected list rules status=200, got=%d body=%s", listResp.Code, listResp.Body.String())
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(listResp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse list response: %v", err)
	}
	var payload struct {
		Items []struct {
			PeriodCode  string `json:"periodCode"`
			ModuleCount int    `json:"moduleCount"`
		} `json:"items"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("failed to parse list payload: %v", err)
	}
	if len(payload.Items) != 4 {
		t.Fatalf("expected 4 quarter rules after sync, got=%d", len(payload.Items))
	}
	for _, item := range payload.Items {
		if item.ModuleCount != 2 {
			t.Fatalf("expected moduleCount=2 for period=%s, got=%d", item.PeriodCode, item.ModuleCount)
		}
	}
}

func TestM3InvalidExpressionRejected(t *testing.T) {
	engine, _ := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	yearID := createAssessmentYearForTest(t, engine, rootToken, 2094)

	createRuleBody, _ := json.Marshal(map[string]any{
		"yearId":         yearID,
		"periodCode":     "YEAR_END",
		"objectType":     "individual",
		"objectCategory": "staff",
		"ruleName":       "Invalid Expression Rule",
		"isActive":       true,
		"modules": []map[string]any{
			{
				"moduleCode": "custom",
				"moduleKey":  "custom_formula",
				"moduleName": "Custom Formula",
				"weight":     1.0,
				"expression": "team.score + os.system",
				"isActive":   true,
			},
		},
	})

	createRuleReq := httptest.NewRequest(http.MethodPost, "/api/rules", bytes.NewReader(createRuleBody))
	createRuleReq.Header.Set("Authorization", "Bearer "+rootToken)
	createRuleReq.Header.Set("Content-Type", "application/json")
	createRuleResp := httptest.NewRecorder()
	engine.ServeHTTP(createRuleResp, createRuleReq)
	if createRuleResp.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid expression status=400, got=%d body=%s", createRuleResp.Code, createRuleResp.Body.String())
	}
}

func TestM3TemplateSaveAndApplyAcrossYear(t *testing.T) {
	engine, _ := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	sourceYearID := createAssessmentYearForTest(t, engine, rootToken, 2095)
	targetYearID := createAssessmentYearForTest(t, engine, rootToken, 2096)

	createRuleBody, _ := json.Marshal(map[string]any{
		"yearId":         sourceYearID,
		"periodCode":     "YEAR_END",
		"objectType":     "individual",
		"objectCategory": "staff",
		"ruleName":       "Personal Year-End Rule",
		"isActive":       true,
		"modules": []map[string]any{
			{
				"moduleCode": "direct",
				"moduleKey":  "direct_input",
				"moduleName": "Direct Input",
				"weight":     0.5,
				"maxScore":   100,
				"isActive":   true,
			},
			{
				"moduleCode": "custom",
				"moduleKey":  "custom_formula",
				"moduleName": "Custom",
				"weight":     0.5,
				"expression": "team.score * 0.3 + if(team.rank <= 10, 5, 0)",
				"isActive":   true,
			},
		},
	})

	createRuleReq := httptest.NewRequest(http.MethodPost, "/api/rules", bytes.NewReader(createRuleBody))
	createRuleReq.Header.Set("Authorization", "Bearer "+rootToken)
	createRuleReq.Header.Set("Content-Type", "application/json")
	createRuleResp := httptest.NewRecorder()
	engine.ServeHTTP(createRuleResp, createRuleReq)
	if createRuleResp.Code != http.StatusOK {
		t.Fatalf("expected create rule status=200, got=%d body=%s", createRuleResp.Code, createRuleResp.Body.String())
	}
	var createEnvelope apiEnvelope
	if err := json.Unmarshal(createRuleResp.Body.Bytes(), &createEnvelope); err != nil {
		t.Fatalf("failed to parse create rule response: %v", err)
	}
	var createPayload struct {
		Rule struct {
			ID uint `json:"id"`
		} `json:"rule"`
	}
	if err := json.Unmarshal(createEnvelope.Data, &createPayload); err != nil {
		t.Fatalf("failed to parse create rule payload: %v", err)
	}
	if createPayload.Rule.ID == 0 {
		t.Fatalf("expected created rule id > 0")
	}

	createTemplateBody, _ := json.Marshal(map[string]any{
		"templateName": "Staff Year-End Template",
		"description":  "template from rule",
	})
	createTemplateReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/rules/%d/templates", createPayload.Rule.ID), bytes.NewReader(createTemplateBody))
	createTemplateReq.Header.Set("Authorization", "Bearer "+rootToken)
	createTemplateReq.Header.Set("Content-Type", "application/json")
	createTemplateResp := httptest.NewRecorder()
	engine.ServeHTTP(createTemplateResp, createTemplateReq)
	if createTemplateResp.Code != http.StatusOK {
		t.Fatalf("expected create template status=200, got=%d body=%s", createTemplateResp.Code, createTemplateResp.Body.String())
	}
	var templateEnvelope apiEnvelope
	if err := json.Unmarshal(createTemplateResp.Body.Bytes(), &templateEnvelope); err != nil {
		t.Fatalf("failed to parse create template response: %v", err)
	}
	var templatePayload struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(templateEnvelope.Data, &templatePayload); err != nil {
		t.Fatalf("failed to parse create template payload: %v", err)
	}
	if templatePayload.ID == 0 {
		t.Fatalf("expected created template id > 0")
	}

	applyBody, _ := json.Marshal(map[string]any{
		"yearId":         targetYearID,
		"periodCode":     "YEAR_END",
		"objectType":     "individual",
		"objectCategory": "staff",
		"ruleName":       "Applied Staff Year-End Rule",
		"isActive":       true,
		"overwrite":      false,
	})
	applyReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/rules/templates/%d/apply", templatePayload.ID), bytes.NewReader(applyBody))
	applyReq.Header.Set("Authorization", "Bearer "+rootToken)
	applyReq.Header.Set("Content-Type", "application/json")
	applyResp := httptest.NewRecorder()
	engine.ServeHTTP(applyResp, applyReq)
	if applyResp.Code != http.StatusOK {
		t.Fatalf("expected apply template status=200, got=%d body=%s", applyResp.Code, applyResp.Body.String())
	}

	var applyEnvelope apiEnvelope
	if err := json.Unmarshal(applyResp.Body.Bytes(), &applyEnvelope); err != nil {
		t.Fatalf("failed to parse apply response: %v", err)
	}
	var applyPayload struct {
		Rule struct {
			YearID     uint   `json:"yearId"`
			PeriodCode string `json:"periodCode"`
		} `json:"rule"`
	}
	if err := json.Unmarshal(applyEnvelope.Data, &applyPayload); err != nil {
		t.Fatalf("failed to parse apply payload: %v", err)
	}
	if applyPayload.Rule.YearID != targetYearID || applyPayload.Rule.PeriodCode != "YEAR_END" {
		t.Fatalf("expected applied rule year=%d period=YEAR_END, got year=%d period=%s", targetYearID, applyPayload.Rule.YearID, applyPayload.Rule.PeriodCode)
	}
}

func createAssessmentYearForTest(t *testing.T, engine http.Handler, token string, year int) uint {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"year": year,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/assessment/years", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected create year status=200, got=%d body=%s", resp.Code, resp.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse create year response: %v", err)
	}
	var payload struct {
		Year struct {
			ID uint `json:"id"`
		} `json:"year"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("failed to parse create year payload: %v", err)
	}
	if payload.Year.ID == 0 {
		t.Fatalf("expected created year id > 0")
	}
	return payload.Year.ID
}
