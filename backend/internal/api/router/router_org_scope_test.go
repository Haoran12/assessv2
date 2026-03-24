package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"assessv2/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestOrgAdminCannotDeleteEntitiesWithinOwnedOrganization(t *testing.T) {
	engine, db := setupTestServer(t)

	groupOrg := model.Organization{
		OrgName: "Scope Group",
		OrgType: "group",
		Status:  "active",
	}
	if err := db.Create(&groupOrg).Error; err != nil {
		t.Fatalf("failed to create group organization: %v", err)
	}
	inScopeOrg := model.Organization{
		OrgName:  "Scope Company A",
		OrgType:  "company",
		ParentID: &groupOrg.ID,
		Status:   "active",
	}
	if err := db.Create(&inScopeOrg).Error; err != nil {
		t.Fatalf("failed to create in-scope organization: %v", err)
	}
	outOfScopeOrg := model.Organization{
		OrgName:  "Scope Company B",
		OrgType:  "company",
		ParentID: &groupOrg.ID,
		Status:   "active",
	}
	if err := db.Create(&outOfScopeOrg).Error; err != nil {
		t.Fatalf("failed to create out-of-scope organization: %v", err)
	}

	adminUsername := "org_admin_scope"
	createAssessmentAdminWithOrg(t, db, adminUsername, inScopeOrg.ID)
	adminToken, _ := loginAndReadData(t, engine, adminUsername, testDefaultPassword)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	positionLevelID := mustPositionLevelIDByCode(t, db, "department_main")

	createDeptPayload, _ := json.Marshal(map[string]any{
		"deptName":       "Dept A",
		"organizationId": inScopeOrg.ID,
		"status":         "active",
	})
	createDeptReq := httptest.NewRequest(http.MethodPost, "/api/org/departments", bytes.NewReader(createDeptPayload))
	createDeptReq.Header.Set("Authorization", "Bearer "+adminToken)
	createDeptReq.Header.Set("Content-Type", "application/json")
	createDeptResp := httptest.NewRecorder()
	engine.ServeHTTP(createDeptResp, createDeptReq)
	if createDeptResp.Code != http.StatusOK {
		t.Fatalf("expected create department status=200, got=%d body=%s", createDeptResp.Code, createDeptResp.Body.String())
	}
	departmentID := mustIDFromEnvelopeData(t, createDeptResp.Body.Bytes())

	createDeptOutPayload, _ := json.Marshal(map[string]any{
		"deptName":       "Dept B",
		"organizationId": outOfScopeOrg.ID,
		"status":         "active",
	})
	createDeptOutReq := httptest.NewRequest(http.MethodPost, "/api/org/departments", bytes.NewReader(createDeptOutPayload))
	createDeptOutReq.Header.Set("Authorization", "Bearer "+adminToken)
	createDeptOutReq.Header.Set("Content-Type", "application/json")
	createDeptOutResp := httptest.NewRecorder()
	engine.ServeHTTP(createDeptOutResp, createDeptOutReq)
	if createDeptOutResp.Code != http.StatusForbidden {
		t.Fatalf("expected create out-of-scope department status=403, got=%d body=%s", createDeptOutResp.Code, createDeptOutResp.Body.String())
	}

	updateDeptPayload, _ := json.Marshal(map[string]any{
		"deptName":       "Dept A Updated",
		"organizationId": inScopeOrg.ID,
		"status":         "active",
	})
	updateDeptReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/org/departments/%d", departmentID), bytes.NewReader(updateDeptPayload))
	updateDeptReq.Header.Set("Authorization", "Bearer "+adminToken)
	updateDeptReq.Header.Set("Content-Type", "application/json")
	updateDeptResp := httptest.NewRecorder()
	engine.ServeHTTP(updateDeptResp, updateDeptReq)
	if updateDeptResp.Code != http.StatusOK {
		t.Fatalf("expected update department status=200, got=%d body=%s", updateDeptResp.Code, updateDeptResp.Body.String())
	}

	createEmpPayload, _ := json.Marshal(map[string]any{
		"empName":         "Employee A",
		"organizationId":  inScopeOrg.ID,
		"departmentId":    departmentID,
		"positionLevelId": positionLevelID,
		"positionTitle":   "Manager",
		"hireDate":        "2026-01-01",
		"status":          "active",
	})
	createEmpReq := httptest.NewRequest(http.MethodPost, "/api/org/employees", bytes.NewReader(createEmpPayload))
	createEmpReq.Header.Set("Authorization", "Bearer "+adminToken)
	createEmpReq.Header.Set("Content-Type", "application/json")
	createEmpResp := httptest.NewRecorder()
	engine.ServeHTTP(createEmpResp, createEmpReq)
	if createEmpResp.Code != http.StatusOK {
		t.Fatalf("expected create employee status=200, got=%d body=%s", createEmpResp.Code, createEmpResp.Body.String())
	}
	employeeID := mustIDFromEnvelopeData(t, createEmpResp.Body.Bytes())

	createEmpOutPayload, _ := json.Marshal(map[string]any{
		"empName":         "Employee B",
		"organizationId":  outOfScopeOrg.ID,
		"positionLevelId": positionLevelID,
		"positionTitle":   "Specialist",
		"status":          "active",
	})
	createEmpOutReq := httptest.NewRequest(http.MethodPost, "/api/org/employees", bytes.NewReader(createEmpOutPayload))
	createEmpOutReq.Header.Set("Authorization", "Bearer "+adminToken)
	createEmpOutReq.Header.Set("Content-Type", "application/json")
	createEmpOutResp := httptest.NewRecorder()
	engine.ServeHTTP(createEmpOutResp, createEmpOutReq)
	if createEmpOutResp.Code != http.StatusForbidden {
		t.Fatalf("expected create out-of-scope employee status=403, got=%d body=%s", createEmpOutResp.Code, createEmpOutResp.Body.String())
	}

	updateEmpPayload, _ := json.Marshal(map[string]any{
		"empName":         "Employee A Updated",
		"organizationId":  inScopeOrg.ID,
		"departmentId":    departmentID,
		"positionLevelId": positionLevelID,
		"positionTitle":   "Senior Manager",
		"hireDate":        "2026-01-01",
		"status":          "active",
	})
	updateEmpReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/org/employees/%d", employeeID), bytes.NewReader(updateEmpPayload))
	updateEmpReq.Header.Set("Authorization", "Bearer "+adminToken)
	updateEmpReq.Header.Set("Content-Type", "application/json")
	updateEmpResp := httptest.NewRecorder()
	engine.ServeHTTP(updateEmpResp, updateEmpReq)
	if updateEmpResp.Code != http.StatusOK {
		t.Fatalf("expected update employee status=200, got=%d body=%s", updateEmpResp.Code, updateEmpResp.Body.String())
	}

	deleteEmpReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/org/employees/%d", employeeID), nil)
	deleteEmpReq.Header.Set("Authorization", "Bearer "+adminToken)
	deleteEmpResp := httptest.NewRecorder()
	engine.ServeHTTP(deleteEmpResp, deleteEmpReq)
	if deleteEmpResp.Code != http.StatusForbidden {
		t.Fatalf("expected org admin delete employee status=403, got=%d body=%s", deleteEmpResp.Code, deleteEmpResp.Body.String())
	}

	deleteDeptReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/org/departments/%d", departmentID), nil)
	deleteDeptReq.Header.Set("Authorization", "Bearer "+adminToken)
	deleteDeptResp := httptest.NewRecorder()
	engine.ServeHTTP(deleteDeptResp, deleteDeptReq)
	if deleteDeptResp.Code != http.StatusForbidden {
		t.Fatalf("expected org admin delete department status=403, got=%d body=%s", deleteDeptResp.Code, deleteDeptResp.Body.String())
	}

	rootDeleteEmpReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/org/employees/%d", employeeID), nil)
	rootDeleteEmpReq.Header.Set("Authorization", "Bearer "+rootToken)
	rootDeleteEmpResp := httptest.NewRecorder()
	engine.ServeHTTP(rootDeleteEmpResp, rootDeleteEmpReq)
	if rootDeleteEmpResp.Code != http.StatusOK {
		t.Fatalf("expected root delete employee status=200, got=%d body=%s", rootDeleteEmpResp.Code, rootDeleteEmpResp.Body.String())
	}

	rootDeleteDeptReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/org/departments/%d", departmentID), nil)
	rootDeleteDeptReq.Header.Set("Authorization", "Bearer "+rootToken)
	rootDeleteDeptResp := httptest.NewRecorder()
	engine.ServeHTTP(rootDeleteDeptResp, rootDeleteDeptReq)
	if rootDeleteDeptResp.Code != http.StatusOK {
		t.Fatalf("expected root delete department status=200, got=%d body=%s", rootDeleteDeptResp.Code, rootDeleteDeptResp.Body.String())
	}
}

func createAssessmentAdminWithOrg(t *testing.T, db *gorm.DB, username string, organizationID uint) {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(testDefaultPassword), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	user := model.User{
		Username:     username,
		PasswordHash: string(hash),
		Status:       "active",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create assessment admin user: %v", err)
	}

	var role model.Role
	if err := db.Where("role_code = ?", "assessment_admin").First(&role).Error; err != nil {
		t.Fatalf("failed to query assessment_admin role: %v", err)
	}
	userRole := model.UserRole{
		UserID:    user.ID,
		RoleID:    role.ID,
		IsPrimary: true,
	}
	if err := db.Create(&userRole).Error; err != nil {
		t.Fatalf("failed to attach assessment_admin role: %v", err)
	}

	userOrg := model.UserOrganization{
		UserID:           user.ID,
		OrganizationType: "company",
		OrganizationID:   organizationID,
		IsPrimary:        true,
	}
	if err := db.Create(&userOrg).Error; err != nil {
		t.Fatalf("failed to attach organization scope: %v", err)
	}
}

func mustPositionLevelIDByCode(t *testing.T, db *gorm.DB, levelCode string) uint {
	t.Helper()
	var positionLevel model.PositionLevel
	if err := db.Where("level_code = ?", levelCode).First(&positionLevel).Error; err != nil {
		t.Fatalf("failed to query position level by code=%s: %v", levelCode, err)
	}
	return positionLevel.ID
}

func mustIDFromEnvelopeData(t *testing.T, raw []byte) uint {
	t.Helper()
	var envelope apiEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("failed to parse response envelope: %v", err)
	}
	if envelope.Code != 200 {
		t.Fatalf("unexpected business code=%d body=%s", envelope.Code, string(raw))
	}
	var payload struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("failed to parse response data payload: %v", err)
	}
	if payload.ID == 0 {
		t.Fatalf("expected payload id > 0, body=%s", string(raw))
	}
	return payload.ID
}
