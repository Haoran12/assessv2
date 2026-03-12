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

	staffLevelID := mustPositionLevelIDByCode(t, db, "staff")

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

func TestM2PeriodLockedCannotReopen(t *testing.T) {
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

	lockReqBody, _ := json.Marshal(map[string]string{"status": "locked"})
	lockReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/assessment/periods/%d/status", periodID), bytes.NewReader(lockReqBody))
	lockReq.Header.Set("Authorization", "Bearer "+rootToken)
	lockReq.Header.Set("Content-Type", "application/json")
	lockResp := httptest.NewRecorder()
	engine.ServeHTTP(lockResp, lockReq)
	if lockResp.Code != http.StatusOK {
		t.Fatalf("expected lock period status=200, got=%d body=%s", lockResp.Code, lockResp.Body.String())
	}

	reopenReqBody, _ := json.Marshal(map[string]string{"status": "active"})
	reopenReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/assessment/periods/%d/status", periodID), bytes.NewReader(reopenReqBody))
	reopenReq.Header.Set("Authorization", "Bearer "+rootToken)
	reopenReq.Header.Set("Content-Type", "application/json")
	reopenResp := httptest.NewRecorder()
	engine.ServeHTTP(reopenResp, reopenReq)
	if reopenResp.Code != http.StatusBadRequest {
		t.Fatalf("expected reopen locked period status=400, got=%d body=%s", reopenResp.Code, reopenResp.Body.String())
	}
}

func TestM2EmployeeTransferWritesHistory(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	staffLevelID := mustPositionLevelIDByCode(t, db, "staff")
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
