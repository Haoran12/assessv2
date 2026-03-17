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

func TestM2CreateYearAutoPeriodsAndExcludeInactiveTargets(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	staffLevelID := mustPositionLevelIDByCode(t, db, "general_management_personnel")

	activeCompany := createOrganization(t, db, "Company A", "company", "active", nil)
	inactiveCompany := createOrganization(t, db, "Company X", "company", "inactive", nil)

	activeDept := createDepartment(t, db, "Dept A1", activeCompany.ID, "active")
	inactiveDept := createDepartment(t, db, "Dept X1", inactiveCompany.ID, "active")

	activeEmployee := createEmployee(t, db, "Alice", activeCompany.ID, &activeDept.ID, staffLevelID, "active")
	inactiveEmployee := createEmployee(t, db, "Xavier", inactiveCompany.ID, &inactiveDept.ID, staffLevelID, "active")

	createYearBody, _ := json.Marshal(map[string]any{
		"year": 2091,
	})
	createYearReq := httptest.NewRequest(http.MethodPost, "/api/assessment/years", bytes.NewReader(createYearBody))
	createYearReq.Header.Set("Authorization", "Bearer "+rootToken)
	createYearReq.Header.Set("Content-Type", "application/json")
	createYearResp := httptest.NewRecorder()
	engine.ServeHTTP(createYearResp, createYearReq)
	if createYearResp.Code != http.StatusOK {
		t.Fatalf("expected create year status=200, got=%d body=%s", createYearResp.Code, createYearResp.Body.String())
	}

	var createYearEnvelope apiEnvelope
	if err := json.Unmarshal(createYearResp.Body.Bytes(), &createYearEnvelope); err != nil {
		t.Fatalf("failed to parse create year response: %v", err)
	}
	var createYearData struct {
		Year struct {
			ID uint `json:"id"`
		} `json:"year"`
		Periods []struct {
			ID         uint   `json:"id"`
			PeriodCode string `json:"periodCode"`
		} `json:"periods"`
		ObjectsCount int `json:"objectsCount"`
	}
	if err := json.Unmarshal(createYearEnvelope.Data, &createYearData); err != nil {
		t.Fatalf("failed to parse create year payload: %v", err)
	}
	if createYearData.Year.ID == 0 {
		t.Fatalf("expected created year id > 0")
	}
	if len(createYearData.Periods) != 5 {
		t.Fatalf("expected 5 periods in create response, got=%d", len(createYearData.Periods))
	}
	if createYearData.ObjectsCount < 3 {
		t.Fatalf("expected generated objects >=3 for active company/dept/employee, got=%d", createYearData.ObjectsCount)
	}

	periodsReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/assessment/years/%d/periods", createYearData.Year.ID), nil)
	periodsReq.Header.Set("Authorization", "Bearer "+rootToken)
	periodsResp := httptest.NewRecorder()
	engine.ServeHTTP(periodsResp, periodsReq)
	if periodsResp.Code != http.StatusOK {
		t.Fatalf("expected list periods status=200, got=%d body=%s", periodsResp.Code, periodsResp.Body.String())
	}
	var periodsEnvelope apiEnvelope
	if err := json.Unmarshal(periodsResp.Body.Bytes(), &periodsEnvelope); err != nil {
		t.Fatalf("failed to parse periods response: %v", err)
	}
	var periodsData struct {
		Items []struct {
			PeriodCode string `json:"periodCode"`
		} `json:"items"`
	}
	if err := json.Unmarshal(periodsEnvelope.Data, &periodsData); err != nil {
		t.Fatalf("failed to parse periods payload: %v", err)
	}
	if len(periodsData.Items) != 5 {
		t.Fatalf("expected 5 periods in list response, got=%d", len(periodsData.Items))
	}

	objectsReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/assessment/years/%d/objects", createYearData.Year.ID), nil)
	objectsReq.Header.Set("Authorization", "Bearer "+rootToken)
	objectsResp := httptest.NewRecorder()
	engine.ServeHTTP(objectsResp, objectsReq)
	if objectsResp.Code != http.StatusOK {
		t.Fatalf("expected list objects status=200, got=%d body=%s", objectsResp.Code, objectsResp.Body.String())
	}
	var objectsEnvelope apiEnvelope
	if err := json.Unmarshal(objectsResp.Body.Bytes(), &objectsEnvelope); err != nil {
		t.Fatalf("failed to parse objects response: %v", err)
	}
	var objectsData struct {
		Items []struct {
			TargetType string `json:"targetType"`
			TargetID   uint   `json:"targetId"`
		} `json:"items"`
	}
	if err := json.Unmarshal(objectsEnvelope.Data, &objectsData); err != nil {
		t.Fatalf("failed to parse objects payload: %v", err)
	}

	hasActiveCompany := false
	hasActiveDept := false
	hasActiveEmployee := false
	for _, item := range objectsData.Items {
		switch {
		case item.TargetType == "organization" && item.TargetID == activeCompany.ID:
			hasActiveCompany = true
		case item.TargetType == "department" && item.TargetID == activeDept.ID:
			hasActiveDept = true
		case item.TargetType == "employee" && item.TargetID == activeEmployee.ID:
			hasActiveEmployee = true
		case item.TargetType == "organization" && item.TargetID == inactiveCompany.ID:
			t.Fatalf("inactive company should not be generated as assessment object")
		case item.TargetType == "department" && item.TargetID == inactiveDept.ID:
			t.Fatalf("department under inactive company should not be generated as assessment object")
		case item.TargetType == "employee" && item.TargetID == inactiveEmployee.ID:
			t.Fatalf("employee under inactive company should not be generated as assessment object")
		}
	}

	if !hasActiveCompany || !hasActiveDept || !hasActiveEmployee {
		t.Fatalf("expected active organization targets to be generated, company=%v dept=%v employee=%v", hasActiveCompany, hasActiveDept, hasActiveEmployee)
	}
}

func TestM2PeriodCompletedCanReopen(t *testing.T) {
	engine, _ := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	createYearBody, _ := json.Marshal(map[string]any{
		"year": 2092,
	})
	createYearReq := httptest.NewRequest(http.MethodPost, "/api/assessment/years", bytes.NewReader(createYearBody))
	createYearReq.Header.Set("Authorization", "Bearer "+rootToken)
	createYearReq.Header.Set("Content-Type", "application/json")
	createYearResp := httptest.NewRecorder()
	engine.ServeHTTP(createYearResp, createYearReq)
	if createYearResp.Code != http.StatusOK {
		t.Fatalf("expected create year status=200, got=%d body=%s", createYearResp.Code, createYearResp.Body.String())
	}

	var createYearEnvelope apiEnvelope
	if err := json.Unmarshal(createYearResp.Body.Bytes(), &createYearEnvelope); err != nil {
		t.Fatalf("failed to parse create year response: %v", err)
	}
	var createYearData struct {
		Periods []struct {
			ID uint `json:"id"`
		} `json:"periods"`
	}
	if err := json.Unmarshal(createYearEnvelope.Data, &createYearData); err != nil {
		t.Fatalf("failed to parse create year payload: %v", err)
	}
	if len(createYearData.Periods) == 0 {
		t.Fatalf("expected periods in create year response")
	}
	periodID := createYearData.Periods[0].ID

	lockReqBody, _ := json.Marshal(map[string]string{"status": "completed"})
	lockReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/assessment/periods/%d/status", periodID), bytes.NewReader(lockReqBody))
	lockReq.Header.Set("Authorization", "Bearer "+rootToken)
	lockReq.Header.Set("Content-Type", "application/json")
	lockResp := httptest.NewRecorder()
	engine.ServeHTTP(lockResp, lockReq)
	if lockResp.Code != http.StatusOK {
		t.Fatalf("expected complete period status=200, got=%d body=%s", lockResp.Code, lockResp.Body.String())
	}

	reopenReqBody, _ := json.Marshal(map[string]string{"status": "active"})
	reopenReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/assessment/periods/%d/status", periodID), bytes.NewReader(reopenReqBody))
	reopenReq.Header.Set("Authorization", "Bearer "+rootToken)
	reopenReq.Header.Set("Content-Type", "application/json")
	reopenResp := httptest.NewRecorder()
	engine.ServeHTTP(reopenResp, reopenReq)
	if reopenResp.Code != http.StatusOK {
		t.Fatalf("expected reopen completed period status=200, got=%d body=%s", reopenResp.Code, reopenResp.Body.String())
	}
}

func TestM2EmployeeTransferWritesHistory(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	staffLevelID := mustPositionLevelIDByCode(t, db, "general_management_personnel")
	company := createOrganization(t, db, "Company B", "company", "active", nil)
	fromDept := createDepartment(t, db, "Dept B1", company.ID, "active")
	toDept := createDepartment(t, db, "Dept B2", company.ID, "active")
	employee := createEmployee(t, db, "Bob", company.ID, &fromDept.ID, staffLevelID, "active")

	transferBody, _ := json.Marshal(map[string]any{
		"changeType":      "transfer",
		"newDepartmentId": toDept.ID,
		"changeReason":    "test transfer",
		"effectiveDate":   time.Now().Format("2006-01-02"),
	})
	transferReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/org/employees/%d/transfer", employee.ID), bytes.NewReader(transferBody))
	transferReq.Header.Set("Authorization", "Bearer "+rootToken)
	transferReq.Header.Set("Content-Type", "application/json")
	transferResp := httptest.NewRecorder()
	engine.ServeHTTP(transferResp, transferReq)
	if transferResp.Code != http.StatusOK {
		t.Fatalf("expected transfer employee status=200, got=%d body=%s", transferResp.Code, transferResp.Body.String())
	}

	historyReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/org/employees/%d/history", employee.ID), nil)
	historyReq.Header.Set("Authorization", "Bearer "+rootToken)
	historyResp := httptest.NewRecorder()
	engine.ServeHTTP(historyResp, historyReq)
	if historyResp.Code != http.StatusOK {
		t.Fatalf("expected employee history status=200, got=%d body=%s", historyResp.Code, historyResp.Body.String())
	}
	var historyEnvelope apiEnvelope
	if err := json.Unmarshal(historyResp.Body.Bytes(), &historyEnvelope); err != nil {
		t.Fatalf("failed to parse history response: %v", err)
	}
	var historyData struct {
		Items []struct {
			ChangeType      string `json:"changeType"`
			NewDepartmentID *uint  `json:"newDepartmentId"`
		} `json:"items"`
	}
	if err := json.Unmarshal(historyEnvelope.Data, &historyData); err != nil {
		t.Fatalf("failed to parse history payload: %v", err)
	}
	if len(historyData.Items) == 0 {
		t.Fatalf("expected at least one employee history record")
	}
	if historyData.Items[0].ChangeType != "transfer" {
		t.Fatalf("expected latest history changeType=transfer, got=%s", historyData.Items[0].ChangeType)
	}
	if historyData.Items[0].NewDepartmentID == nil || *historyData.Items[0].NewDepartmentID != toDept.ID {
		t.Fatalf("expected latest history newDepartmentId=%d, got=%v", toDept.ID, historyData.Items[0].NewDepartmentID)
	}
}

func TestM2RootCanDeleteOrganizationDepartmentEmployee(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	staffLevelID := mustPositionLevelIDByCode(t, db, "general_management_personnel")
	company := createOrganization(t, db, "Delete Co", "company", "active", nil)
	dept := createDepartment(t, db, "Delete Dept", company.ID, "active")
	employee := createEmployee(t, db, "Delete Bob", company.ID, &dept.ID, staffLevelID, "active")

	deleteEmployeeReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/org/employees/%d", employee.ID), nil)
	deleteEmployeeReq.Header.Set("Authorization", "Bearer "+rootToken)
	deleteEmployeeResp := httptest.NewRecorder()
	engine.ServeHTTP(deleteEmployeeResp, deleteEmployeeReq)
	if deleteEmployeeResp.Code != http.StatusOK {
		t.Fatalf("expected delete employee status=200, got=%d body=%s", deleteEmployeeResp.Code, deleteEmployeeResp.Body.String())
	}

	var activeEmployeeCount int64
	if err := db.Model(&model.Employee{}).Where("id = ? AND deleted_at IS NULL", employee.ID).Count(&activeEmployeeCount).Error; err != nil {
		t.Fatalf("failed to verify employee soft deletion: %v", err)
	}
	if activeEmployeeCount != 0 {
		t.Fatalf("expected employee deleted_at to be set, active count=%d", activeEmployeeCount)
	}

	deleteDepartmentReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/org/departments/%d", dept.ID), nil)
	deleteDepartmentReq.Header.Set("Authorization", "Bearer "+rootToken)
	deleteDepartmentResp := httptest.NewRecorder()
	engine.ServeHTTP(deleteDepartmentResp, deleteDepartmentReq)
	if deleteDepartmentResp.Code != http.StatusOK {
		t.Fatalf("expected delete department status=200, got=%d body=%s", deleteDepartmentResp.Code, deleteDepartmentResp.Body.String())
	}

	var activeDepartmentCount int64
	if err := db.Model(&model.Department{}).Where("id = ? AND deleted_at IS NULL", dept.ID).Count(&activeDepartmentCount).Error; err != nil {
		t.Fatalf("failed to verify department soft deletion: %v", err)
	}
	if activeDepartmentCount != 0 {
		t.Fatalf("expected department deleted_at to be set, active count=%d", activeDepartmentCount)
	}

	deleteOrganizationReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/org/organizations/%d", company.ID), nil)
	deleteOrganizationReq.Header.Set("Authorization", "Bearer "+rootToken)
	deleteOrganizationResp := httptest.NewRecorder()
	engine.ServeHTTP(deleteOrganizationResp, deleteOrganizationReq)
	if deleteOrganizationResp.Code != http.StatusOK {
		t.Fatalf("expected delete organization status=200, got=%d body=%s", deleteOrganizationResp.Code, deleteOrganizationResp.Body.String())
	}

	var activeOrganizationCount int64
	if err := db.Model(&model.Organization{}).Where("id = ? AND deleted_at IS NULL", company.ID).Count(&activeOrganizationCount).Error; err != nil {
		t.Fatalf("failed to verify organization soft deletion: %v", err)
	}
	if activeOrganizationCount != 0 {
		t.Fatalf("expected organization deleted_at to be set, active count=%d", activeOrganizationCount)
	}
}

func TestM2DeleteOrganizationDepartmentEmployeeRequiresRoot(t *testing.T) {
	engine, db := setupTestServer(t)
	createViewerUser(t, db, "viewer_org_delete", "Viewer OrgDelete")
	viewerToken, _ := loginAndReadData(t, engine, "viewer_org_delete", testDefaultPassword)

	staffLevelID := mustPositionLevelIDByCode(t, db, "general_management_personnel")
	company := createOrganization(t, db, "Protected Co", "company", "active", nil)
	dept := createDepartment(t, db, "Protected Dept", company.ID, "active")
	employee := createEmployee(t, db, "Protected Bob", company.ID, &dept.ID, staffLevelID, "active")

	deleteEmployeeReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/org/employees/%d", employee.ID), nil)
	deleteEmployeeReq.Header.Set("Authorization", "Bearer "+viewerToken)
	deleteEmployeeResp := httptest.NewRecorder()
	engine.ServeHTTP(deleteEmployeeResp, deleteEmployeeReq)
	if deleteEmployeeResp.Code != http.StatusForbidden {
		t.Fatalf("expected viewer delete employee status=403, got=%d body=%s", deleteEmployeeResp.Code, deleteEmployeeResp.Body.String())
	}

	deleteDepartmentReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/org/departments/%d", dept.ID), nil)
	deleteDepartmentReq.Header.Set("Authorization", "Bearer "+viewerToken)
	deleteDepartmentResp := httptest.NewRecorder()
	engine.ServeHTTP(deleteDepartmentResp, deleteDepartmentReq)
	if deleteDepartmentResp.Code != http.StatusForbidden {
		t.Fatalf("expected viewer delete department status=403, got=%d body=%s", deleteDepartmentResp.Code, deleteDepartmentResp.Body.String())
	}

	deleteOrganizationReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/org/organizations/%d", company.ID), nil)
	deleteOrganizationReq.Header.Set("Authorization", "Bearer "+viewerToken)
	deleteOrganizationResp := httptest.NewRecorder()
	engine.ServeHTTP(deleteOrganizationResp, deleteOrganizationReq)
	if deleteOrganizationResp.Code != http.StatusForbidden {
		t.Fatalf("expected viewer delete organization status=403, got=%d body=%s", deleteOrganizationResp.Code, deleteOrganizationResp.Body.String())
	}
}

func TestM2RootCanCRUDPositionLevels(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	createBody, _ := json.Marshal(map[string]any{
		"levelCode":       "custom_test_level",
		"levelName":       "Custom Test Level",
		"description":     "for api test",
		"isForAssessment": true,
		"sortOrder":       88,
		"status":          "active",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/org/position-levels", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+rootToken)
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	engine.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("expected create position level status=200, got=%d body=%s", createResp.Code, createResp.Body.String())
	}

	var createEnvelope apiEnvelope
	if err := json.Unmarshal(createResp.Body.Bytes(), &createEnvelope); err != nil {
		t.Fatalf("failed to parse create position level response: %v", err)
	}
	var created struct {
		ID        uint   `json:"id"`
		LevelCode string `json:"levelCode"`
		LevelName string `json:"levelName"`
	}
	if err := json.Unmarshal(createEnvelope.Data, &created); err != nil {
		t.Fatalf("failed to parse create position level payload: %v", err)
	}
	if created.ID == 0 {
		t.Fatalf("expected created position level id > 0")
	}
	if created.LevelCode != "custom_test_level" {
		t.Fatalf("expected created level_code=custom_test_level, got=%s", created.LevelCode)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"levelCode":       "custom_test_level_v2",
		"levelName":       "Custom Test Level V2",
		"description":     "updated",
		"isForAssessment": false,
		"sortOrder":       99,
		"status":          "inactive",
	})
	updateReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/org/position-levels/%d", created.ID), bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+rootToken)
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp := httptest.NewRecorder()
	engine.ServeHTTP(updateResp, updateReq)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected update position level status=200, got=%d body=%s", updateResp.Code, updateResp.Body.String())
	}

	var updateEnvelope apiEnvelope
	if err := json.Unmarshal(updateResp.Body.Bytes(), &updateEnvelope); err != nil {
		t.Fatalf("failed to parse update position level response: %v", err)
	}
	var updated struct {
		ID              uint   `json:"id"`
		LevelCode       string `json:"levelCode"`
		LevelName       string `json:"levelName"`
		IsForAssessment bool   `json:"isForAssessment"`
		Status          string `json:"status"`
	}
	if err := json.Unmarshal(updateEnvelope.Data, &updated); err != nil {
		t.Fatalf("failed to parse update position level payload: %v", err)
	}
	if updated.ID != created.ID {
		t.Fatalf("expected updated id=%d, got=%d", created.ID, updated.ID)
	}
	if updated.LevelCode != "custom_test_level_v2" {
		t.Fatalf("expected updated level_code=custom_test_level_v2, got=%s", updated.LevelCode)
	}
	if updated.LevelName != "Custom Test Level V2" || updated.IsForAssessment || updated.Status != "inactive" {
		t.Fatalf("unexpected update payload levelName=%s isForAssessment=%v status=%s", updated.LevelName, updated.IsForAssessment, updated.Status)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/org/position-levels/%d", created.ID), nil)
	deleteReq.Header.Set("Authorization", "Bearer "+rootToken)
	deleteResp := httptest.NewRecorder()
	engine.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("expected delete position level status=200, got=%d body=%s", deleteResp.Code, deleteResp.Body.String())
	}

	var count int64
	if err := db.Model(&model.PositionLevel{}).Where("id = ?", created.ID).Count(&count).Error; err != nil {
		t.Fatalf("failed to verify deleted position level: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected deleted position level count=0, got=%d", count)
	}
}

func TestM2ListAssessmentCategories(t *testing.T) {
	engine, _ := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	req := httptest.NewRequest(http.MethodGet, "/api/org/assessment-categories", nil)
	req.Header.Set("Authorization", "Bearer "+rootToken)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected list assessment categories status=200, got=%d body=%s", resp.Code, resp.Body.String())
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse list assessment categories response: %v", err)
	}
	var payload struct {
		Items []struct {
			CategoryCode string `json:"categoryCode"`
			ObjectType   string `json:"objectType"`
		} `json:"items"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("failed to parse list assessment categories payload: %v", err)
	}
	if len(payload.Items) != 11 {
		t.Fatalf("expected default category count=11, got=%d", len(payload.Items))
	}

	teamCount := 0
	individualCount := 0
	for _, item := range payload.Items {
		if item.ObjectType == "team" {
			teamCount++
		}
		if item.ObjectType == "individual" {
			individualCount++
		}
	}
	if teamCount != 6 || individualCount != 5 {
		t.Fatalf("expected category distribution team=6 individual=5, got team=%d individual=%d", teamCount, individualCount)
	}
}

func TestM2RootCanDeleteSystemPositionLevel(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)
	systemLevelID := mustPositionLevelIDByCode(t, db, "leadership_main")

	deleteReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/org/position-levels/%d", systemLevelID), nil)
	deleteReq.Header.Set("Authorization", "Bearer "+rootToken)
	deleteResp := httptest.NewRecorder()
	engine.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("expected delete system position level status=200, got=%d body=%s", deleteResp.Code, deleteResp.Body.String())
	}

	var count int64
	if err := db.Model(&model.PositionLevel{}).Where("id = ?", systemLevelID).Count(&count).Error; err != nil {
		t.Fatalf("failed to verify deleted system position level: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected deleted system position level count=0, got=%d", count)
	}
}

func TestM2PositionLevelMutationRequiresRoot(t *testing.T) {
	engine, db := setupTestServer(t)
	createViewerUser(t, db, "viewer_pl", "Viewer PositionLevel")

	viewerToken, _ := loginAndReadData(t, engine, "viewer_pl", testDefaultPassword)
	existingLevelID := mustPositionLevelIDByCode(t, db, "general_management_personnel")

	createBody, _ := json.Marshal(map[string]any{
		"levelCode":       "viewer_should_fail",
		"levelName":       "Viewer Should Fail",
		"isForAssessment": true,
		"sortOrder":       1,
		"status":          "active",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/org/position-levels", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+viewerToken)
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	engine.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusForbidden {
		t.Fatalf("expected viewer create position level status=403, got=%d body=%s", createResp.Code, createResp.Body.String())
	}

	updateBody, _ := json.Marshal(map[string]any{
		"levelCode":       "general_management_personnel",
		"levelName":       "Viewer Should Fail Update",
		"isForAssessment": true,
		"sortOrder":       1,
		"status":          "active",
	})
	updateReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/org/position-levels/%d", existingLevelID), bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+viewerToken)
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp := httptest.NewRecorder()
	engine.ServeHTTP(updateResp, updateReq)
	if updateResp.Code != http.StatusForbidden {
		t.Fatalf("expected viewer update position level status=403, got=%d body=%s", updateResp.Code, updateResp.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/org/position-levels/%d", existingLevelID), nil)
	deleteReq.Header.Set("Authorization", "Bearer "+viewerToken)
	deleteResp := httptest.NewRecorder()
	engine.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusForbidden {
		t.Fatalf("expected viewer delete position level status=403, got=%d body=%s", deleteResp.Code, deleteResp.Body.String())
	}
}

func mustPositionLevelIDByCode(t *testing.T, db *gorm.DB, levelCode string) uint {
	t.Helper()
	var level model.PositionLevel
	if err := db.Where("level_code = ?", levelCode).First(&level).Error; err != nil {
		t.Fatalf("failed to query position level by code=%s: %v", levelCode, err)
	}
	return level.ID
}

func createOrganization(
	t *testing.T,
	db *gorm.DB,
	orgName string,
	orgType string,
	status string,
	parentID *uint,
) model.Organization {
	t.Helper()
	item := model.Organization{
		OrgName:  orgName,
		OrgType:  orgType,
		ParentID: parentID,
		Status:   status,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("failed to create organization %s: %v", orgName, err)
	}
	return item
}

func createDepartment(
	t *testing.T,
	db *gorm.DB,
	deptName string,
	organizationID uint,
	status string,
) model.Department {
	t.Helper()
	item := model.Department{
		DeptName:       deptName,
		OrganizationID: organizationID,
		Status:         status,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("failed to create department %s: %v", deptName, err)
	}
	return item
}

func createEmployee(
	t *testing.T,
	db *gorm.DB,
	empName string,
	organizationID uint,
	departmentID *uint,
	positionLevelID uint,
	status string,
) model.Employee {
	t.Helper()
	item := model.Employee{
		EmpName:         empName,
		OrganizationID:  organizationID,
		DepartmentID:    departmentID,
		PositionLevelID: positionLevelID,
		Status:          status,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("failed to create employee %s: %v", empName, err)
	}
	return item
}
