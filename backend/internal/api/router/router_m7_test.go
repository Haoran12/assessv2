package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"assessv2/backend/internal/model"
	"gorm.io/gorm"
)

func TestM7BackupLifecycle(t *testing.T) {
	engine, _ := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	createBody, _ := json.Marshal(map[string]any{
		"description": "M7 backup lifecycle test",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/backup/records", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+rootToken)
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	engine.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("expected create backup status=200, got=%d body=%s", createResp.Code, createResp.Body.String())
	}

	var createEnvelope apiEnvelope
	if err := json.Unmarshal(createResp.Body.Bytes(), &createEnvelope); err != nil {
		t.Fatalf("failed to parse create backup response: %v", err)
	}
	var created struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(createEnvelope.Data, &created); err != nil {
		t.Fatalf("failed to parse create backup data: %v", err)
	}
	if created.ID == 0 {
		t.Fatalf("expected created backup id > 0")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/backup/records?page=1&pageSize=20", nil)
	listReq.Header.Set("Authorization", "Bearer "+rootToken)
	listResp := httptest.NewRecorder()
	engine.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected list backup status=200, got=%d body=%s", listResp.Code, listResp.Body.String())
	}

	downloadReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/backup/records/%d/download", created.ID), nil)
	downloadReq.Header.Set("Authorization", "Bearer "+rootToken)
	downloadResp := httptest.NewRecorder()
	engine.ServeHTTP(downloadResp, downloadReq)
	if downloadResp.Code != http.StatusOK {
		t.Fatalf("expected download backup status=200, got=%d body=%s", downloadResp.Code, downloadResp.Body.String())
	}
	if len(downloadResp.Body.Bytes()) == 0 {
		t.Fatalf("expected non-empty backup download content")
	}

	badRestoreBody, _ := json.Marshal(map[string]any{
		"confirmText": "wrong-confirmation",
	})
	badRestoreReq := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/backup/records/%d/restore", created.ID),
		bytes.NewReader(badRestoreBody),
	)
	badRestoreReq.Header.Set("Authorization", "Bearer "+rootToken)
	badRestoreReq.Header.Set("Content-Type", "application/json")
	badRestoreResp := httptest.NewRecorder()
	engine.ServeHTTP(badRestoreResp, badRestoreReq)
	if badRestoreResp.Code != http.StatusBadRequest {
		t.Fatalf("expected bad restore status=400, got=%d body=%s", badRestoreResp.Code, badRestoreResp.Body.String())
	}

	restoreBody, _ := json.Marshal(map[string]any{
		"confirmText": "CONFIRM_RESTORE",
	})
	restoreReq := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/backup/records/%d/restore", created.ID),
		bytes.NewReader(restoreBody),
	)
	restoreReq.Header.Set("Authorization", "Bearer "+rootToken)
	restoreReq.Header.Set("Content-Type", "application/json")
	restoreResp := httptest.NewRecorder()
	engine.ServeHTTP(restoreResp, restoreReq)
	if restoreResp.Code != http.StatusOK {
		t.Fatalf("expected restore status=200, got=%d body=%s", restoreResp.Code, restoreResp.Body.String())
	}
}

func TestM7SettingsAuditDetail(t *testing.T) {
	engine, _ := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	settingsReq := httptest.NewRequest(http.MethodGet, "/api/system/settings", nil)
	settingsReq.Header.Set("Authorization", "Bearer "+rootToken)
	settingsResp := httptest.NewRecorder()
	engine.ServeHTTP(settingsResp, settingsReq)
	if settingsResp.Code != http.StatusOK {
		t.Fatalf("expected settings list status=200, got=%d body=%s", settingsResp.Code, settingsResp.Body.String())
	}

	updateBody, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{
				"settingKey":   "system.name",
				"settingValue": "AssessV2 M7 Test",
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
			ID uint `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(auditEnvelope.Data, &auditPayload); err != nil {
		t.Fatalf("failed to parse audit list payload: %v", err)
	}
	if len(auditPayload.Items) == 0 {
		t.Fatalf("expected at least one audit log for system settings")
	}
	auditID := auditPayload.Items[0].ID

	detailReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/system/audit-logs/%d", auditID), nil)
	detailReq.Header.Set("Authorization", "Bearer "+rootToken)
	detailResp := httptest.NewRecorder()
	engine.ServeHTTP(detailResp, detailReq)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("expected audit detail status=200, got=%d body=%s", detailResp.Code, detailResp.Body.String())
	}

	verifyReq := httptest.NewRequest(http.MethodGet, "/api/system/settings", nil)
	verifyReq.Header.Set("Authorization", "Bearer "+rootToken)
	verifyResp := httptest.NewRecorder()
	engine.ServeHTTP(verifyResp, verifyReq)
	if verifyResp.Code != http.StatusOK {
		t.Fatalf("expected verify settings status=200, got=%d body=%s", verifyResp.Code, verifyResp.Body.String())
	}

	var verifyEnvelope apiEnvelope
	if err := json.Unmarshal(verifyResp.Body.Bytes(), &verifyEnvelope); err != nil {
		t.Fatalf("failed to parse verify settings response: %v", err)
	}
	var verifyPayload struct {
		Items []struct {
			SettingKey   string `json:"settingKey"`
			SettingValue string `json:"settingValue"`
		} `json:"items"`
	}
	if err := json.Unmarshal(verifyEnvelope.Data, &verifyPayload); err != nil {
		t.Fatalf("failed to parse verify settings payload: %v", err)
	}

	systemName := ""
	for _, item := range verifyPayload.Items {
		if item.SettingKey == "system.name" {
			systemName = item.SettingValue
			break
		}
	}
	if systemName != "AssessV2 M7 Test" {
		t.Fatalf("expected updated system.name=AssessV2 M7 Test, got=%s", systemName)
	}
}

func TestM7OrgPackageLifecycle(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)
	rootOrgID, employeeAID, employeeBID := seedOrgBackupFixture(t, db)

	createBody, _ := json.Marshal(map[string]any{
		"rootOrganizationId":     rootOrgID,
		"description":            "M7 org package lifecycle test",
		"includeEmployeeHistory": true,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/backup/org-packages", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+rootToken)
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	engine.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("expected create org package status=200, got=%d body=%s", createResp.Code, createResp.Body.String())
	}

	var createEnvelope apiEnvelope
	if err := json.Unmarshal(createResp.Body.Bytes(), &createEnvelope); err != nil {
		t.Fatalf("failed to parse create org package response: %v", err)
	}
	var created struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(createEnvelope.Data, &created); err != nil {
		t.Fatalf("failed to parse create org package payload: %v", err)
	}
	if created.ID == 0 {
		t.Fatalf("expected created org package id > 0")
	}

	listReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/backup/org-packages?page=1&pageSize=20&rootOrganizationId=%d", rootOrgID), nil)
	listReq.Header.Set("Authorization", "Bearer "+rootToken)
	listResp := httptest.NewRecorder()
	engine.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected list org package status=200, got=%d body=%s", listResp.Code, listResp.Body.String())
	}

	downloadReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/backup/org-packages/%d/download", created.ID), nil)
	downloadReq.Header.Set("Authorization", "Bearer "+rootToken)
	downloadResp := httptest.NewRecorder()
	engine.ServeHTTP(downloadResp, downloadReq)
	if downloadResp.Code != http.StatusOK {
		t.Fatalf("expected download org package status=200, got=%d body=%s", downloadResp.Code, downloadResp.Body.String())
	}
	if len(downloadResp.Body.Bytes()) == 0 {
		t.Fatalf("expected non-empty org package download content")
	}

	var packageRecord model.BackupRecord
	if err := db.Where("id = ?", created.ID).First(&packageRecord).Error; err != nil {
		t.Fatalf("failed to query created org package record: %v", err)
	}
	if packageRecord.ContentType != "org_logical" || packageRecord.ScopeOrgID == nil || *packageRecord.ScopeOrgID != rootOrgID {
		t.Fatalf("unexpected org package record metadata: %+v", packageRecord)
	}
	var manifest map[string]any
	if err := json.Unmarshal([]byte(packageRecord.ManifestJSON), &manifest); err != nil {
		t.Fatalf("failed to parse org package manifest json: %v", err)
	}
	if sanitized, ok := manifest["sanitizedHistoryRefsCount"].(float64); !ok || sanitized < 1 {
		t.Fatalf("expected sanitizedHistoryRefsCount >= 1, got=%v", manifest["sanitizedHistoryRefsCount"])
	}

	if err := db.Model(&model.Employee{}).Where("id = ?", employeeAID).Update("emp_name", "Mutated Employee A").Error; err != nil {
		t.Fatalf("failed to mutate scoped employee: %v", err)
	}

	var positionLevel model.PositionLevel
	if err := db.Order("id ASC").First(&positionLevel).Error; err != nil {
		t.Fatalf("failed to load position level: %v", err)
	}
	extraEmployee := model.Employee{
		EmpName:         "Extra Scoped Employee",
		OrganizationID:  rootOrgID,
		PositionLevelID: positionLevel.ID,
		Status:          "active",
	}
	if err := db.Create(&extraEmployee).Error; err != nil {
		t.Fatalf("failed to insert extra scoped employee: %v", err)
	}

	badRestoreBody, _ := json.Marshal(map[string]any{
		"confirmText":              "WRONG",
		"mode":                     "replace_scope",
		"targetRootOrganizationId": rootOrgID,
	})
	badRestoreReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/backup/org-packages/%d/restore", created.ID), bytes.NewReader(badRestoreBody))
	badRestoreReq.Header.Set("Authorization", "Bearer "+rootToken)
	badRestoreReq.Header.Set("Content-Type", "application/json")
	badRestoreResp := httptest.NewRecorder()
	engine.ServeHTTP(badRestoreResp, badRestoreReq)
	if badRestoreResp.Code != http.StatusBadRequest {
		t.Fatalf("expected bad org restore status=400, got=%d body=%s", badRestoreResp.Code, badRestoreResp.Body.String())
	}

	restoreBody, _ := json.Marshal(map[string]any{
		"confirmText":              "CONFIRM_ORG_RESTORE",
		"mode":                     "replace_scope",
		"targetRootOrganizationId": rootOrgID,
	})
	restoreReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/backup/org-packages/%d/restore", created.ID), bytes.NewReader(restoreBody))
	restoreReq.Header.Set("Authorization", "Bearer "+rootToken)
	restoreReq.Header.Set("Content-Type", "application/json")
	restoreResp := httptest.NewRecorder()
	engine.ServeHTTP(restoreResp, restoreReq)
	if restoreResp.Code != http.StatusOK {
		t.Fatalf("expected org restore status=200, got=%d body=%s", restoreResp.Code, restoreResp.Body.String())
	}

	var employeeA model.Employee
	if err := db.Where("id = ?", employeeAID).First(&employeeA).Error; err != nil {
		t.Fatalf("failed to load employeeA after restore: %v", err)
	}
	if employeeA.EmpName != "Employee A" {
		t.Fatalf("expected employeeA name restored to Employee A, got=%s", employeeA.EmpName)
	}

	var extraCount int64
	if err := db.Model(&model.Employee{}).Where("id = ?", extraEmployee.ID).Count(&extraCount).Error; err != nil {
		t.Fatalf("failed to check extra employee after restore: %v", err)
	}
	if extraCount != 0 {
		t.Fatalf("expected extra scoped employee to be removed after restore")
	}

	var employeeB model.Employee
	if err := db.Where("id = ?", employeeBID).First(&employeeB).Error; err != nil {
		t.Fatalf("failed to load employeeB after restore: %v", err)
	}
	if employeeB.EmpName != "Employee B" {
		t.Fatalf("expected out-of-scope employeeB unchanged, got=%s", employeeB.EmpName)
	}

	var beforeRestoreCount int64
	if err := db.Model(&model.BackupRecord{}).
		Where("backup_type = ? AND content_type = ?", "before_restore", "full_snapshot").
		Count(&beforeRestoreCount).Error; err != nil {
		t.Fatalf("failed to query before_restore backup records: %v", err)
	}
	if beforeRestoreCount == 0 {
		t.Fatalf("expected before_restore full snapshot backup to be created")
	}
}

func seedOrgBackupFixture(t *testing.T, db *gorm.DB) (uint, uint, uint) {
	t.Helper()

	groupA := model.Organization{OrgName: "Group A", OrgType: "group", Status: "active"}
	groupB := model.Organization{OrgName: "Group B", OrgType: "group", Status: "active"}
	if err := db.Create(&groupA).Error; err != nil {
		t.Fatalf("failed to create Group A: %v", err)
	}
	if err := db.Create(&groupB).Error; err != nil {
		t.Fatalf("failed to create Group B: %v", err)
	}

	childA := model.Organization{OrgName: "Group A Child", OrgType: "company", ParentID: &groupA.ID, Status: "active"}
	if err := db.Create(&childA).Error; err != nil {
		t.Fatalf("failed to create child org: %v", err)
	}

	deptA := model.Department{DeptName: "Dept A", OrganizationID: groupA.ID, Status: "active"}
	deptB := model.Department{DeptName: "Dept B", OrganizationID: groupB.ID, Status: "active"}
	if err := db.Create(&deptA).Error; err != nil {
		t.Fatalf("failed to create deptA: %v", err)
	}
	if err := db.Create(&deptB).Error; err != nil {
		t.Fatalf("failed to create deptB: %v", err)
	}

	var positionLevel model.PositionLevel
	if err := db.Order("id ASC").First(&positionLevel).Error; err != nil {
		t.Fatalf("failed to load position level: %v", err)
	}

	employeeA := model.Employee{
		EmpName:         "Employee A",
		OrganizationID:  groupA.ID,
		DepartmentID:    &deptA.ID,
		PositionLevelID: positionLevel.ID,
		Status:          "active",
	}
	employeeB := model.Employee{
		EmpName:         "Employee B",
		OrganizationID:  groupB.ID,
		DepartmentID:    &deptB.ID,
		PositionLevelID: positionLevel.ID,
		Status:          "active",
	}
	if err := db.Create(&employeeA).Error; err != nil {
		t.Fatalf("failed to create employeeA: %v", err)
	}
	if err := db.Create(&employeeB).Error; err != nil {
		t.Fatalf("failed to create employeeB: %v", err)
	}

	history := model.EmployeeHistory{
		EmployeeID:        employeeA.ID,
		ChangeType:        "transfer",
		OldOrganizationID: &groupB.ID,
		NewOrganizationID: &groupA.ID,
		OldDepartmentID:   &deptB.ID,
		NewDepartmentID:   &deptA.ID,
		EffectiveDate:     time.Date(2026, time.January, 10, 0, 0, 0, 0, time.UTC),
	}
	if err := db.Create(&history).Error; err != nil {
		t.Fatalf("failed to create employee history: %v", err)
	}

	sessionA := model.AssessmentSession{
		AssessmentName: "assessment_a_2026",
		DisplayName:    "Assessment A",
		Year:           2026,
		OrganizationID: groupA.ID,
		DataDir:        "data/a",
	}
	sessionB := model.AssessmentSession{
		AssessmentName: "assessment_b_2026",
		DisplayName:    "Assessment B",
		Year:           2026,
		OrganizationID: groupB.ID,
		DataDir:        "data/b",
	}
	if err := db.Create(&sessionA).Error; err != nil {
		t.Fatalf("failed to create assessment session A: %v", err)
	}
	if err := db.Create(&sessionB).Error; err != nil {
		t.Fatalf("failed to create assessment session B: %v", err)
	}

	periodA := model.AssessmentSessionPeriod{
		AssessmentID: sessionA.ID,
		PeriodCode:   "Q1",
		PeriodName:   "Quarter 1",
	}
	if err := db.Create(&periodA).Error; err != nil {
		t.Fatalf("failed to create assessment period: %v", err)
	}
	groupRowA := model.AssessmentObjectGroup{
		AssessmentID: sessionA.ID,
		ObjectType:   "team",
		GroupCode:    "team_group",
		GroupName:    "Team Group",
	}
	if err := db.Create(&groupRowA).Error; err != nil {
		t.Fatalf("failed to create assessment object group: %v", err)
	}
	objectA := model.AssessmentSessionObject{
		AssessmentID: sessionA.ID,
		ObjectType:   "team",
		GroupCode:    "team_group",
		TargetID:     groupA.ID,
		TargetType:   "organization",
		ObjectName:   "Group A Team",
	}
	if err := db.Create(&objectA).Error; err != nil {
		t.Fatalf("failed to create assessment session object: %v", err)
	}
	moduleScoreA := model.AssessmentObjectModuleScore{
		AssessmentID: sessionA.ID,
		PeriodCode:   "Q1",
		ObjectID:     objectA.ID,
		ModuleKey:    "m1",
		Score:        95,
	}
	if err := db.Create(&moduleScoreA).Error; err != nil {
		t.Fatalf("failed to create assessment module score: %v", err)
	}
	ruleFileA := model.RuleFile{
		AssessmentID: sessionA.ID,
		RuleName:     "Rule A",
		ContentJSON:  `{"type":"demo"}`,
		FilePath:     "rules/rule-a.json",
	}
	if err := db.Create(&ruleFileA).Error; err != nil {
		t.Fatalf("failed to create rule file: %v", err)
	}

	return groupA.ID, employeeA.ID, employeeB.ID
}
