package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"assessv2/backend/internal/model"
	"gorm.io/gorm"
)

func TestM5AutoRecalculateAfterDirectScoreMutation(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	company := createOrganization(t, db, "M5 Auto Company", "company", "active", nil)
	yearID := createAssessmentYearForTest(t, engine, rootToken, 2099)
	activateYearAndPeriodForTest(t, engine, db, rootToken, yearID, "Q1")
	teamObjectID := mustAssessmentObjectIDByTarget(t, db, yearID, "organization", company.ID)

	ruleID := createM4Rule(t, engine, rootToken, yearID, "Q1", "team", "subsidiary_company", []map[string]any{
		{
			"moduleCode": "direct",
			"moduleKey":  "direct_only",
			"moduleName": "Direct Only",
			"weight":     1.0,
			"maxScore":   100,
			"sortOrder":  1,
			"isActive":   true,
		},
	})
	moduleID := mustModuleIDByRuleAndCode(t, db, ruleID, "direct")

	scoreRecord := createDirectScoreForTest(t, engine, rootToken, map[string]any{
		"yearId":     yearID,
		"periodCode": "Q1",
		"moduleId":   moduleID,
		"objectId":   teamObjectID,
		"score":      86.25,
		"remark":     "auto trigger create",
	})

	items := listCalculatedScoresForTest(t, engine, rootToken, fmt.Sprintf("/api/calc/scores?yearId=%d&periodCode=Q1&objectId=%d", yearID, teamObjectID))
	if len(items) != 1 {
		t.Fatalf("expected calculated score count=1 after create, got=%d", len(items))
	}
	if items[0].FinalScore != 86.25 {
		t.Fatalf("expected final score=86.25 after create, got=%v", items[0].FinalScore)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"score":  92.4,
		"remark": "auto trigger update",
	})
	updateReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/scores/direct/%d", scoreRecord.ID), bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+rootToken)
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp := httptest.NewRecorder()
	engine.ServeHTTP(updateResp, updateReq)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected update direct score status=200, got=%d body=%s", updateResp.Code, updateResp.Body.String())
	}

	items = listCalculatedScoresForTest(t, engine, rootToken, fmt.Sprintf("/api/calc/scores?yearId=%d&periodCode=Q1&objectId=%d", yearID, teamObjectID))
	if len(items) != 1 {
		t.Fatalf("expected calculated score count=1 after update, got=%d", len(items))
	}
	if items[0].FinalScore != 92.4 {
		t.Fatalf("expected final score=92.4 after update, got=%v", items[0].FinalScore)
	}
}

func TestM5ManualRecalculateWithDependencyAndTieBreakRanking(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	levelID := mustPositionLevelIDByCode(t, db, "general_management_personnel")
	companyA := createOrganization(t, db, "M5 Company A", "company", "active", nil)
	companyB := createOrganization(t, db, "M5 Company B", "company", "active", nil)
	deptA := createDepartment(t, db, "M5 Dept A", companyA.ID, "active")
	deptB := createDepartment(t, db, "M5 Dept B", companyB.ID, "active")
	empA := createEmployee(t, db, "M5 Alice", companyA.ID, &deptA.ID, levelID, "active")
	empB := createEmployee(t, db, "M5 Bob", companyB.ID, &deptB.ID, levelID, "active")

	yearID := createAssessmentYearForTest(t, engine, rootToken, 2100)
	activateYearAndPeriodForTest(t, engine, db, rootToken, yearID, "Q1")
	teamAObjectID := mustAssessmentObjectIDByTarget(t, db, yearID, "organization", companyA.ID)
	teamBObjectID := mustAssessmentObjectIDByTarget(t, db, yearID, "organization", companyB.ID)
	indAObjectID := mustAssessmentObjectIDByTarget(t, db, yearID, "employee", empA.ID)
	indBObjectID := mustAssessmentObjectIDByTarget(t, db, yearID, "employee", empB.ID)
	if err := db.Model(&model.AssessmentObject{}).Where("id = ?", indAObjectID).Update("parent_object_id", teamAObjectID).Error; err != nil {
		t.Fatalf("failed to set individual A parent object: %v", err)
	}
	if err := db.Model(&model.AssessmentObject{}).Where("id = ?", indBObjectID).Update("parent_object_id", teamBObjectID).Error; err != nil {
		t.Fatalf("failed to set individual B parent object: %v", err)
	}

	teamRuleID := createM4Rule(t, engine, rootToken, yearID, "Q1", "team", "subsidiary_company", []map[string]any{
		{
			"moduleCode": "direct",
			"moduleKey":  "direct_a",
			"moduleName": "Direct A",
			"weight":     0.5,
			"maxScore":   100,
			"sortOrder":  1,
			"isActive":   true,
		},
		{
			"moduleCode": "direct",
			"moduleKey":  "direct_b",
			"moduleName": "Direct B",
			"weight":     0.5,
			"maxScore":   100,
			"sortOrder":  2,
			"isActive":   true,
		},
	})
	moduleAID := mustModuleIDByRuleAndKey(t, db, teamRuleID, "direct_a")
	moduleBID := mustModuleIDByRuleAndKey(t, db, teamRuleID, "direct_b")

	_ = createDirectScoreForTest(t, engine, rootToken, map[string]any{
		"yearId":     yearID,
		"periodCode": "Q1",
		"moduleId":   moduleAID,
		"objectId":   teamAObjectID,
		"score":      90,
	})
	_ = createDirectScoreForTest(t, engine, rootToken, map[string]any{
		"yearId":     yearID,
		"periodCode": "Q1",
		"moduleId":   moduleBID,
		"objectId":   teamAObjectID,
		"score":      80,
	})
	_ = createDirectScoreForTest(t, engine, rootToken, map[string]any{
		"yearId":     yearID,
		"periodCode": "Q1",
		"moduleId":   moduleAID,
		"objectId":   teamBObjectID,
		"score":      80,
	})
	_ = createDirectScoreForTest(t, engine, rootToken, map[string]any{
		"yearId":     yearID,
		"periodCode": "Q1",
		"moduleId":   moduleBID,
		"objectId":   teamBObjectID,
		"score":      90,
	})

	_ = createM4Rule(t, engine, rootToken, yearID, "Q1", "individual", "general_management_personnel", []map[string]any{
		{
			"moduleCode": "custom",
			"moduleKey":  "custom_team",
			"moduleName": "Custom Team Dependency",
			"weight":     1.0,
			"sortOrder":  1,
			"expression": "team.score + if(team.rank <= 1, 5, 0)",
			"isActive":   true,
		},
	})

	recalculateBody, _ := json.Marshal(map[string]any{
		"yearId":     yearID,
		"periodCode": "Q1",
	})
	recalculateReq := httptest.NewRequest(http.MethodPost, "/api/calc/recalculate", bytes.NewReader(recalculateBody))
	recalculateReq.Header.Set("Authorization", "Bearer "+rootToken)
	recalculateReq.Header.Set("Content-Type", "application/json")
	recalculateResp := httptest.NewRecorder()
	engine.ServeHTTP(recalculateResp, recalculateReq)
	if recalculateResp.Code != http.StatusOK {
		t.Fatalf("expected manual recalculate status=200, got=%d body=%s", recalculateResp.Code, recalculateResp.Body.String())
	}

	teamRankings := listRankingsForTest(t, engine, rootToken,
		fmt.Sprintf("/api/calc/rankings?yearId=%d&periodCode=Q1&scope=overall&objectType=team&objectCategory=subsidiary_company", yearID))
	if len(teamRankings) < 2 {
		t.Fatalf("expected at least 2 team rankings, got=%d", len(teamRankings))
	}
	if teamRankings[0].ObjectID != teamAObjectID || teamRankings[0].RankNo != 1 {
		t.Fatalf("expected team A rank=1 by tie-break, got objectId=%d rank=%d", teamRankings[0].ObjectID, teamRankings[0].RankNo)
	}
	if teamRankings[1].ObjectID != teamBObjectID || teamRankings[1].RankNo != 2 {
		t.Fatalf("expected team B rank=2 by tie-break, got objectId=%d rank=%d", teamRankings[1].ObjectID, teamRankings[1].RankNo)
	}

	indAScore := listCalculatedScoresForTest(t, engine, rootToken,
		fmt.Sprintf("/api/calc/scores?yearId=%d&periodCode=Q1&objectId=%d", yearID, indAObjectID))
	indBScore := listCalculatedScoresForTest(t, engine, rootToken,
		fmt.Sprintf("/api/calc/scores?yearId=%d&periodCode=Q1&objectId=%d", yearID, indBObjectID))
	if len(indAScore) != 1 || len(indBScore) != 1 {
		t.Fatalf("expected two individual calculated scores, got A=%d B=%d", len(indAScore), len(indBScore))
	}
	if indAScore[0].FinalScore <= indBScore[0].FinalScore {
		t.Fatalf("expected individual A score > B score due team.rank dependency, got A=%v B=%v", indAScore[0].FinalScore, indBScore[0].FinalScore)
	}

	modules := listCalculatedModulesForTest(t, engine, rootToken, indAScore[0].ID)
	if len(modules) != 1 || modules[0].ModuleKey != "custom_team" {
		t.Fatalf("expected custom module detail for individual A, got=%+v", modules)
	}
}

func createDirectScoreForTest(t *testing.T, engine http.Handler, token string, payload map[string]any) model.DirectScore {
	t.Helper()
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/scores/direct", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected create direct score status=200, got=%d body=%s", resp.Code, resp.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse create direct score response: %v", err)
	}
	var item model.DirectScore
	if err := json.Unmarshal(envelope.Data, &item); err != nil {
		t.Fatalf("failed to parse create direct score payload: %v", err)
	}
	return item
}

func mustAssessmentObjectIDByTarget(t *testing.T, db *gorm.DB, yearID uint, targetType string, targetID uint) uint {
	t.Helper()
	var object model.AssessmentObject
	if err := db.Where("year_id = ? AND target_type = ? AND target_id = ?", yearID, targetType, targetID).First(&object).Error; err != nil {
		t.Fatalf("failed to query assessment object by target(%s,%d): %v", targetType, targetID, err)
	}
	return object.ID
}

func mustModuleIDByRuleAndKey(t *testing.T, db *gorm.DB, ruleID uint, moduleKey string) uint {
	t.Helper()
	var module model.ScoreModule
	if err := db.Where("rule_id = ? AND module_key = ?", ruleID, moduleKey).First(&module).Error; err != nil {
		t.Fatalf("failed to query module by key=%s rule=%d: %v", moduleKey, ruleID, err)
	}
	return module.ID
}

func listCalculatedScoresForTest(t *testing.T, engine http.Handler, token string, url string) []serviceCalculatedScoreItem {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected list calculated scores status=200, got=%d body=%s", resp.Code, resp.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse list calculated scores response: %v", err)
	}
	var payload struct {
		Items []serviceCalculatedScoreItem `json:"items"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("failed to parse list calculated scores payload: %v", err)
	}
	return payload.Items
}

func listCalculatedModulesForTest(t *testing.T, engine http.Handler, token string, calculatedScoreID uint) []model.CalculatedModuleScore {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/calc/scores/%d/modules", calculatedScoreID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected list calculated modules status=200, got=%d body=%s", resp.Code, resp.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse list calculated modules response: %v", err)
	}
	var payload struct {
		Items []model.CalculatedModuleScore `json:"items"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("failed to parse list calculated modules payload: %v", err)
	}
	return payload.Items
}

func listRankingsForTest(t *testing.T, engine http.Handler, token string, url string) []model.Ranking {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected list rankings status=200, got=%d body=%s", resp.Code, resp.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse list rankings response: %v", err)
	}
	var payload struct {
		Items []model.Ranking `json:"items"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("failed to parse list rankings payload: %v", err)
	}
	return payload.Items
}

type serviceCalculatedScoreItem struct {
	ID         uint    `json:"id"`
	ObjectID   uint    `json:"objectId"`
	FinalScore float64 `json:"finalScore"`
}

func activateYearAndPeriodForTest(
	t *testing.T,
	engine http.Handler,
	db *gorm.DB,
	token string,
	yearID uint,
	periodCode string,
) {
	t.Helper()

	yearBody, _ := json.Marshal(map[string]any{"status": "active"})
	yearReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/assessment/years/%d/status", yearID), bytes.NewReader(yearBody))
	yearReq.Header.Set("Authorization", "Bearer "+token)
	yearReq.Header.Set("Content-Type", "application/json")
	yearResp := httptest.NewRecorder()
	engine.ServeHTTP(yearResp, yearReq)
	if yearResp.Code != http.StatusOK {
		t.Fatalf("expected update year status=200, got=%d body=%s", yearResp.Code, yearResp.Body.String())
	}

	var period model.AssessmentPeriod
	if err := db.Where("year_id = ? AND period_code = ?", yearID, periodCode).First(&period).Error; err != nil {
		t.Fatalf("failed to query period(%s) for year=%d: %v", periodCode, yearID, err)
	}

	periodBody, _ := json.Marshal(map[string]any{"status": "active"})
	periodReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/assessment/periods/%d/status", period.ID), bytes.NewReader(periodBody))
	periodReq.Header.Set("Authorization", "Bearer "+token)
	periodReq.Header.Set("Content-Type", "application/json")
	periodResp := httptest.NewRecorder()
	engine.ServeHTTP(periodResp, periodReq)
	if periodResp.Code != http.StatusOK {
		t.Fatalf("expected update period status=200, got=%d body=%s", periodResp.Code, periodResp.Body.String())
	}
}
