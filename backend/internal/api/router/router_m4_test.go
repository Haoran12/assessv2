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

func TestM4DirectScoreAndExtraPointFlow(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	createOrganization(t, db, "M4 Company 1", "company", "active", nil)
	createOrganization(t, db, "M4 Company 2", "company", "active", nil)
	yearID := createAssessmentYearForTest(t, engine, rootToken, 2097)
	activateYearAndPeriodForTest(t, engine, db, rootToken, yearID, "Q1")
	objectIDs := listAssessmentObjectIDsForYear(t, engine, rootToken, yearID)
	if len(objectIDs) < 2 {
		t.Fatalf("expected at least 2 assessment objects, got=%d", len(objectIDs))
	}

	ruleID := createM4Rule(t, engine, rootToken, yearID, "Q1", "team", "subsidiary_company", []map[string]any{
		{
			"moduleCode": "direct",
			"moduleKey":  "direct_q1",
			"moduleName": "Direct Q1",
			"weight":     1.0,
			"maxScore":   100,
			"isActive":   true,
		},
	})
	directModuleID := mustModuleIDByRuleAndCode(t, db, ruleID, "direct")

	createDirectBody, _ := json.Marshal(map[string]any{
		"yearId":     yearID,
		"periodCode": "Q1",
		"moduleId":   directModuleID,
		"objectId":   objectIDs[0],
		"score":      88.6,
		"remark":     "single input",
	})
	createDirectReq := httptest.NewRequest(http.MethodPost, "/api/scores/direct", bytes.NewReader(createDirectBody))
	createDirectReq.Header.Set("Authorization", "Bearer "+rootToken)
	createDirectReq.Header.Set("Content-Type", "application/json")
	createDirectResp := httptest.NewRecorder()
	engine.ServeHTTP(createDirectResp, createDirectReq)
	if createDirectResp.Code != http.StatusOK {
		t.Fatalf("expected create direct score status=200, got=%d body=%s", createDirectResp.Code, createDirectResp.Body.String())
	}

	dupDirectReq := httptest.NewRequest(http.MethodPost, "/api/scores/direct", bytes.NewReader(createDirectBody))
	dupDirectReq.Header.Set("Authorization", "Bearer "+rootToken)
	dupDirectReq.Header.Set("Content-Type", "application/json")
	dupDirectResp := httptest.NewRecorder()
	engine.ServeHTTP(dupDirectResp, dupDirectReq)
	if dupDirectResp.Code != http.StatusBadRequest {
		t.Fatalf("expected duplicate direct score status=400, got=%d body=%s", dupDirectResp.Code, dupDirectResp.Body.String())
	}

	batchBody, _ := json.Marshal(map[string]any{
		"yearId":     yearID,
		"periodCode": "Q1",
		"moduleId":   directModuleID,
		"overwrite":  false,
		"entries": []map[string]any{
			{"objectId": objectIDs[0], "score": 92.1, "remark": "duplicate row"},
			{"objectId": objectIDs[1], "score": 76.4, "remark": "new row"},
		},
	})
	batchReq := httptest.NewRequest(http.MethodPost, "/api/scores/direct/batch", bytes.NewReader(batchBody))
	batchReq.Header.Set("Authorization", "Bearer "+rootToken)
	batchReq.Header.Set("Content-Type", "application/json")
	batchResp := httptest.NewRecorder()
	engine.ServeHTTP(batchResp, batchReq)
	if batchResp.Code != http.StatusOK {
		t.Fatalf("expected batch direct score status=200, got=%d body=%s", batchResp.Code, batchResp.Body.String())
	}
	var batchEnvelope apiEnvelope
	if err := json.Unmarshal(batchResp.Body.Bytes(), &batchEnvelope); err != nil {
		t.Fatalf("failed to parse batch direct score response: %v", err)
	}
	var batchData struct {
		Created int `json:"created"`
		Skipped int `json:"skipped"`
	}
	if err := json.Unmarshal(batchEnvelope.Data, &batchData); err != nil {
		t.Fatalf("failed to parse batch direct score payload: %v", err)
	}
	if batchData.Created != 1 || batchData.Skipped != 1 {
		t.Fatalf("expected batch result created=1 skipped=1, got created=%d skipped=%d", batchData.Created, batchData.Skipped)
	}

	extraBody, _ := json.Marshal(map[string]any{
		"yearId":     yearID,
		"periodCode": "Q1",
		"objectId":   objectIDs[0],
		"points":     -5.0,
		"reason":     "discipline issue",
		"evidence":   "case#M4-001",
	})
	extraReq := httptest.NewRequest(http.MethodPost, "/api/scores/extra", bytes.NewReader(extraBody))
	extraReq.Header.Set("Authorization", "Bearer "+rootToken)
	extraReq.Header.Set("Content-Type", "application/json")
	extraResp := httptest.NewRecorder()
	engine.ServeHTTP(extraResp, extraReq)
	if extraResp.Code != http.StatusOK {
		t.Fatalf("expected create extra point status=200, got=%d body=%s", extraResp.Code, extraResp.Body.String())
	}

	var extraEnvelope apiEnvelope
	if err := json.Unmarshal(extraResp.Body.Bytes(), &extraEnvelope); err != nil {
		t.Fatalf("failed to parse extra point response: %v", err)
	}
	var extraData struct {
		ID        uint    `json:"id"`
		PointType string  `json:"pointType"`
		Points    float64 `json:"points"`
	}
	if err := json.Unmarshal(extraEnvelope.Data, &extraData); err != nil {
		t.Fatalf("failed to parse extra point payload: %v", err)
	}
	if extraData.ID == 0 || extraData.PointType != "deduct" || extraData.Points != 5 {
		t.Fatalf("unexpected extra point payload: %+v", extraData)
	}

	updateExtraBody, _ := json.Marshal(map[string]any{
		"pointType": "add",
		"points":    3.5,
		"reason":    "innovation award",
		"evidence":  "doc#2027-A",
		"approve":   true,
	})
	updateExtraReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/scores/extra/%d", extraData.ID), bytes.NewReader(updateExtraBody))
	updateExtraReq.Header.Set("Authorization", "Bearer "+rootToken)
	updateExtraReq.Header.Set("Content-Type", "application/json")
	updateExtraResp := httptest.NewRecorder()
	engine.ServeHTTP(updateExtraResp, updateExtraReq)
	if updateExtraResp.Code != http.StatusOK {
		t.Fatalf("expected update extra point status=200, got=%d body=%s", updateExtraResp.Code, updateExtraResp.Body.String())
	}

	listExtraReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/scores/extra?yearId=%d&periodCode=Q1", yearID), nil)
	listExtraReq.Header.Set("Authorization", "Bearer "+rootToken)
	listExtraResp := httptest.NewRecorder()
	engine.ServeHTTP(listExtraResp, listExtraReq)
	if listExtraResp.Code != http.StatusOK {
		t.Fatalf("expected list extra points status=200, got=%d body=%s", listExtraResp.Code, listExtraResp.Body.String())
	}
	var listExtraEnvelope apiEnvelope
	if err := json.Unmarshal(listExtraResp.Body.Bytes(), &listExtraEnvelope); err != nil {
		t.Fatalf("failed to parse list extra response: %v", err)
	}
	var listExtraData struct {
		Items []model.ExtraPoint `json:"items"`
	}
	if err := json.Unmarshal(listExtraEnvelope.Data, &listExtraData); err != nil {
		t.Fatalf("failed to parse list extra payload: %v", err)
	}
	if len(listExtraData.Items) == 0 {
		t.Fatalf("expected at least one extra point item")
	}

	var directAuditCount int64
	if err := db.Model(&model.AuditLog{}).
		Where("target_type = ? AND action_detail LIKE ?", "direct_scores", "%create_direct_score%").
		Count(&directAuditCount).Error; err != nil {
		t.Fatalf("failed to query direct score audit logs: %v", err)
	}
	if directAuditCount == 0 {
		t.Fatalf("expected direct score audit logs")
	}

	var extraAuditCount int64
	if err := db.Model(&model.AuditLog{}).
		Where("target_type = ? AND action_detail LIKE ?", "extra_points", "%create_extra_point%").
		Count(&extraAuditCount).Error; err != nil {
		t.Fatalf("failed to query extra point audit logs: %v", err)
	}
	if extraAuditCount == 0 {
		t.Fatalf("expected extra point audit logs")
	}
}

func TestM4VoteGenerateDraftSubmitResetAndStats(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)
	rootUserID := mustUserIDByUsername(t, db, "root")

	createOrganization(t, db, "M4 Vote Company", "company", "active", nil)
	yearID := createAssessmentYearForTest(t, engine, rootToken, 2098)
	activateYearAndPeriodForTest(t, engine, db, rootToken, yearID, "Q1")
	objectIDs := listAssessmentObjectIDsForYear(t, engine, rootToken, yearID)
	if len(objectIDs) == 0 {
		t.Fatalf("expected at least one assessment object")
	}

	ruleID := createM4Rule(t, engine, rootToken, yearID, "Q1", "team", "subsidiary_company", []map[string]any{
		{
			"moduleCode": "vote",
			"moduleKey":  "vote_q1",
			"moduleName": "Vote Q1",
			"weight":     1.0,
			"isActive":   true,
			"voteGroups": []map[string]any{
				{
					"groupCode":  "root_vote",
					"groupName":  "Root Vote Group",
					"weight":     1.0,
					"voterType":  "custom",
					"maxScore":   100,
					"isActive":   true,
					"voterScope": map[string]any{"user_ids": []uint{rootUserID}},
				},
			},
		},
	})
	voteModuleID := mustModuleIDByRuleAndCode(t, db, ruleID, "vote")

	generateBody, _ := json.Marshal(map[string]any{
		"yearId":     yearID,
		"periodCode": "Q1",
		"moduleId":   voteModuleID,
		"objectIds":  []uint{objectIDs[0]},
	})
	generateReq := httptest.NewRequest(http.MethodPost, "/api/votes/tasks/generate", bytes.NewReader(generateBody))
	generateReq.Header.Set("Authorization", "Bearer "+rootToken)
	generateReq.Header.Set("Content-Type", "application/json")
	generateResp := httptest.NewRecorder()
	engine.ServeHTTP(generateResp, generateReq)
	if generateResp.Code != http.StatusOK {
		t.Fatalf("expected generate vote tasks status=200, got=%d body=%s", generateResp.Code, generateResp.Body.String())
	}

	listTasksReq := httptest.NewRequest(http.MethodGet, "/api/votes/tasks?mine=true", nil)
	listTasksReq.Header.Set("Authorization", "Bearer "+rootToken)
	listTasksResp := httptest.NewRecorder()
	engine.ServeHTTP(listTasksResp, listTasksReq)
	if listTasksResp.Code != http.StatusOK {
		t.Fatalf("expected list vote tasks status=200, got=%d body=%s", listTasksResp.Code, listTasksResp.Body.String())
	}
	var listTasksEnvelope apiEnvelope
	if err := json.Unmarshal(listTasksResp.Body.Bytes(), &listTasksEnvelope); err != nil {
		t.Fatalf("failed to parse list vote tasks response: %v", err)
	}
	var listTasksData struct {
		Items []struct {
			ID     uint   `json:"id"`
			Status string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listTasksEnvelope.Data, &listTasksData); err != nil {
		t.Fatalf("failed to parse list vote tasks payload: %v", err)
	}
	if len(listTasksData.Items) == 0 {
		t.Fatalf("expected generated vote tasks")
	}
	taskID := listTasksData.Items[0].ID
	if listTasksData.Items[0].Status != "pending" {
		t.Fatalf("expected generated vote task status=pending, got=%s", listTasksData.Items[0].Status)
	}

	resetPendingReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/votes/tasks/%d/reset", taskID), nil)
	resetPendingReq.Header.Set("Authorization", "Bearer "+rootToken)
	resetPendingResp := httptest.NewRecorder()
	engine.ServeHTTP(resetPendingResp, resetPendingReq)
	if resetPendingResp.Code != http.StatusBadRequest {
		t.Fatalf("expected reset pending vote task status=400, got=%d body=%s", resetPendingResp.Code, resetPendingResp.Body.String())
	}

	draftBody, _ := json.Marshal(map[string]any{
		"gradeOption": "good",
		"remark":      "draft vote",
	})
	draftReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/votes/tasks/%d/draft", taskID), bytes.NewReader(draftBody))
	draftReq.Header.Set("Authorization", "Bearer "+rootToken)
	draftReq.Header.Set("Content-Type", "application/json")
	draftResp := httptest.NewRecorder()
	engine.ServeHTTP(draftResp, draftReq)
	if draftResp.Code != http.StatusOK {
		t.Fatalf("expected save vote draft status=200, got=%d body=%s", draftResp.Code, draftResp.Body.String())
	}

	draftStatsReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/votes/statistics?yearId=%d&periodCode=Q1&moduleId=%d", yearID, voteModuleID), nil)
	draftStatsReq.Header.Set("Authorization", "Bearer "+rootToken)
	draftStatsResp := httptest.NewRecorder()
	engine.ServeHTTP(draftStatsResp, draftStatsReq)
	if draftStatsResp.Code != http.StatusOK {
		t.Fatalf("expected draft vote statistics status=200, got=%d body=%s", draftStatsResp.Code, draftStatsResp.Body.String())
	}
	var draftStatsEnvelope apiEnvelope
	if err := json.Unmarshal(draftStatsResp.Body.Bytes(), &draftStatsEnvelope); err != nil {
		t.Fatalf("failed to parse draft vote statistics response: %v", err)
	}
	var draftStatsData struct {
		TotalTasks      int `json:"totalTasks"`
		CompletedTasks  int `json:"completedTasks"`
		GroupStatistics []struct {
			GradeCounts map[string]int `json:"gradeCounts"`
		} `json:"groupStatistics"`
	}
	if err := json.Unmarshal(draftStatsEnvelope.Data, &draftStatsData); err != nil {
		t.Fatalf("failed to parse draft vote statistics payload: %v", err)
	}
	if draftStatsData.TotalTasks == 0 || draftStatsData.CompletedTasks != 0 {
		t.Fatalf("expected draft stats total>0 and completed=0, got total=%d completed=%d", draftStatsData.TotalTasks, draftStatsData.CompletedTasks)
	}
	if len(draftStatsData.GroupStatistics) == 0 {
		t.Fatalf("expected group statistics in draft stage")
	}
	if draftStatsData.GroupStatistics[0].GradeCounts["good"] != 0 {
		t.Fatalf("expected draft vote not counted in grade stats, got=%+v", draftStatsData.GroupStatistics[0].GradeCounts)
	}

	submitBody, _ := json.Marshal(map[string]any{
		"gradeOption": "excellent",
		"remark":      "final vote",
	})
	submitReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/votes/tasks/%d/submit", taskID), bytes.NewReader(submitBody))
	submitReq.Header.Set("Authorization", "Bearer "+rootToken)
	submitReq.Header.Set("Content-Type", "application/json")
	submitResp := httptest.NewRecorder()
	engine.ServeHTTP(submitResp, submitReq)
	if submitResp.Code != http.StatusOK {
		t.Fatalf("expected submit vote status=200, got=%d body=%s", submitResp.Code, submitResp.Body.String())
	}

	reSubmitReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/votes/tasks/%d/submit", taskID), bytes.NewReader(submitBody))
	reSubmitReq.Header.Set("Authorization", "Bearer "+rootToken)
	reSubmitReq.Header.Set("Content-Type", "application/json")
	reSubmitResp := httptest.NewRecorder()
	engine.ServeHTTP(reSubmitResp, reSubmitReq)
	if reSubmitResp.Code != http.StatusBadRequest {
		t.Fatalf("expected re-submit vote status=400, got=%d body=%s", reSubmitResp.Code, reSubmitResp.Body.String())
	}

	statsReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/votes/statistics?yearId=%d&periodCode=Q1&moduleId=%d", yearID, voteModuleID), nil)
	statsReq.Header.Set("Authorization", "Bearer "+rootToken)
	statsResp := httptest.NewRecorder()
	engine.ServeHTTP(statsResp, statsReq)
	if statsResp.Code != http.StatusOK {
		t.Fatalf("expected vote statistics status=200, got=%d body=%s", statsResp.Code, statsResp.Body.String())
	}
	var statsEnvelope apiEnvelope
	if err := json.Unmarshal(statsResp.Body.Bytes(), &statsEnvelope); err != nil {
		t.Fatalf("failed to parse vote statistics response: %v", err)
	}
	var statsData struct {
		TotalTasks      int `json:"totalTasks"`
		CompletedTasks  int `json:"completedTasks"`
		GroupStatistics []struct {
			GradeCounts map[string]int `json:"gradeCounts"`
		} `json:"groupStatistics"`
	}
	if err := json.Unmarshal(statsEnvelope.Data, &statsData); err != nil {
		t.Fatalf("failed to parse vote statistics payload: %v", err)
	}
	if statsData.TotalTasks == 0 || statsData.CompletedTasks == 0 {
		t.Fatalf("expected completed vote statistics, got total=%d completed=%d", statsData.TotalTasks, statsData.CompletedTasks)
	}
	if len(statsData.GroupStatistics) == 0 || statsData.GroupStatistics[0].GradeCounts["excellent"] == 0 {
		t.Fatalf("expected excellent grade count > 0 in group statistics, got=%+v", statsData.GroupStatistics)
	}

	resetReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/votes/tasks/%d/reset", taskID), nil)
	resetReq.Header.Set("Authorization", "Bearer "+rootToken)
	resetResp := httptest.NewRecorder()
	engine.ServeHTTP(resetResp, resetReq)
	if resetResp.Code != http.StatusOK {
		t.Fatalf("expected reset vote task status=200, got=%d body=%s", resetResp.Code, resetResp.Body.String())
	}

	var voteAuditCount int64
	if err := db.Model(&model.AuditLog{}).
		Where("target_type = ? AND action_detail LIKE ?", "vote_tasks", "%submit_vote%").
		Count(&voteAuditCount).Error; err != nil {
		t.Fatalf("failed to query vote audit logs: %v", err)
	}
	if voteAuditCount == 0 {
		t.Fatalf("expected vote submit audit logs")
	}
}

func createM4Rule(
	t *testing.T,
	engine http.Handler,
	token string,
	yearID uint,
	periodCode string,
	objectType string,
	objectCategory string,
	modules []map[string]any,
) uint {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"yearId":         yearID,
		"periodCode":     periodCode,
		"objectType":     objectType,
		"objectCategory": objectCategory,
		"ruleName":       fmt.Sprintf("M4 Rule %d %s", yearID, periodCode),
		"isActive":       true,
		"modules":        modules,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rules", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected create rule status=200, got=%d body=%s", resp.Code, resp.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse create rule response: %v", err)
	}
	var payload struct {
		Rule struct {
			ID uint `json:"id"`
		} `json:"rule"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("failed to parse create rule payload: %v", err)
	}
	if payload.Rule.ID == 0 {
		t.Fatalf("expected create rule id > 0")
	}
	return payload.Rule.ID
}

func listAssessmentObjectIDsForYear(t *testing.T, engine http.Handler, token string, yearID uint) []uint {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/assessment/years/%d/objects", yearID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected list assessment objects status=200, got=%d body=%s", resp.Code, resp.Body.String())
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse list objects response: %v", err)
	}
	var payload struct {
		Items []struct {
			ID uint `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("failed to parse list objects payload: %v", err)
	}
	ids := make([]uint, 0, len(payload.Items))
	for _, item := range payload.Items {
		ids = append(ids, item.ID)
	}
	return ids
}

func mustModuleIDByRuleAndCode(t *testing.T, db *gorm.DB, ruleID uint, moduleCode string) uint {
	t.Helper()
	var module model.ScoreModule
	if err := db.Where("rule_id = ? AND module_code = ?", ruleID, moduleCode).First(&module).Error; err != nil {
		t.Fatalf("failed to query module by rule=%d code=%s: %v", ruleID, moduleCode, err)
	}
	return module.ID
}
