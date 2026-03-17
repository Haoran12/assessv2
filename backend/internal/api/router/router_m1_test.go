package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"assessv2/backend/internal/config"
	"assessv2/backend/internal/database"
	"assessv2/backend/internal/migration"
	"assessv2/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	testDefaultPassword = "#AssessV2@Init"
	testChangedPassword = "NewPass#2027"
)

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func TestM1LoginProfileAndMustChangePassword(t *testing.T) {
	engine, db := setupTestServer(t)

	token, loginData := loginAndReadData(t, engine, "root", testDefaultPassword)
	if !mustBoolField(t, loginData, "mustChangePassword") {
		t.Fatalf("expected root mustChangePassword=true on first login")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/system/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected profile status=200, got=%d body=%s", resp.Code, resp.Body.String())
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse profile response: %v", err)
	}
	if envelope.Code != 200 {
		t.Fatalf("unexpected business code=%d body=%s", envelope.Code, resp.Body.String())
	}
	if !mustBoolField(t, envelope.Data, "mustChangePassword") {
		t.Fatalf("expected profile mustChangePassword=true")
	}

	var count int64
	if err := db.Model(&model.AuditLog{}).Where("action_type = ?", "login").Count(&count).Error; err != nil {
		t.Fatalf("failed to query audit logs: %v", err)
	}
	if count < 1 {
		t.Fatalf("expected at least one login audit log, got=%d", count)
	}
}

func TestM1RBACViewerDeniedUsersEndpoint(t *testing.T) {
	engine, db := setupTestServer(t)
	createViewerUser(t, db, "viewer1", "Viewer User")

	viewerToken, _ := loginAndReadData(t, engine, "viewer1", testDefaultPassword)

	viewerReq := httptest.NewRequest(http.MethodGet, "/api/system/users", nil)
	viewerReq.Header.Set("Authorization", "Bearer "+viewerToken)
	viewerResp := httptest.NewRecorder()
	engine.ServeHTTP(viewerResp, viewerReq)
	if viewerResp.Code != http.StatusForbidden {
		t.Fatalf("expected viewer status=403, got=%d body=%s", viewerResp.Code, viewerResp.Body.String())
	}

	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)
	rootReq := httptest.NewRequest(http.MethodGet, "/api/system/users?page=1&pageSize=10", nil)
	rootReq.Header.Set("Authorization", "Bearer "+rootToken)
	rootResp := httptest.NewRecorder()
	engine.ServeHTTP(rootResp, rootReq)
	if rootResp.Code != http.StatusOK {
		t.Fatalf("expected root status=200, got=%d body=%s", rootResp.Code, rootResp.Body.String())
	}
}

func TestM1ChangePasswordAndAudit(t *testing.T) {
	engine, db := setupTestServer(t)
	token, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	changePayload := map[string]string{
		"oldPassword": testDefaultPassword,
		"newPassword": testChangedPassword,
	}
	changeBody, _ := json.Marshal(changePayload)
	changeReq := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", bytes.NewReader(changeBody))
	changeReq.Header.Set("Authorization", "Bearer "+token)
	changeReq.Header.Set("Content-Type", "application/json")
	changeResp := httptest.NewRecorder()
	engine.ServeHTTP(changeResp, changeReq)
	if changeResp.Code != http.StatusOK {
		t.Fatalf("expected change-password status=200, got=%d body=%s", changeResp.Code, changeResp.Body.String())
	}

	legacyReqBody, _ := json.Marshal(map[string]string{
		"username": "root",
		"password": testDefaultPassword,
	})
	legacyReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(legacyReqBody))
	legacyReq.Header.Set("Content-Type", "application/json")
	legacyResp := httptest.NewRecorder()
	engine.ServeHTTP(legacyResp, legacyReq)
	if legacyResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected old password login status=401, got=%d body=%s", legacyResp.Code, legacyResp.Body.String())
	}

	_, loginData := loginAndReadData(t, engine, "root", testChangedPassword)
	if mustBoolField(t, loginData, "mustChangePassword") {
		t.Fatalf("expected root mustChangePassword=false after change password")
	}

	var count int64
	if err := db.Model(&model.AuditLog{}).
		Where("action_type = ? AND action_detail LIKE ?", "update", "%change_password%").
		Count(&count).Error; err != nil {
		t.Fatalf("failed to query change-password audit logs: %v", err)
	}
	if count < 1 {
		t.Fatalf("expected at least one change_password audit log, got=%d", count)
	}
}

func TestM1RootOnlyUserGroupEndpoints(t *testing.T) {
	engine, db := setupTestServer(t)
	createViewerUser(t, db, "viewer2", "Viewer Two")

	viewerToken, _ := loginAndReadData(t, engine, "viewer2", testDefaultPassword)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	viewerListReq := httptest.NewRequest(http.MethodGet, "/api/system/groups", nil)
	viewerListReq.Header.Set("Authorization", "Bearer "+viewerToken)
	viewerListResp := httptest.NewRecorder()
	engine.ServeHTTP(viewerListResp, viewerListReq)
	if viewerListResp.Code != http.StatusForbidden {
		t.Fatalf("expected viewer groups status=403, got=%d body=%s", viewerListResp.Code, viewerListResp.Body.String())
	}

	createBody, _ := json.Marshal(map[string]string{
		"roleCode":    "ops-team",
		"roleName":    "Operations Team",
		"description": "ops users",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/system/groups", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+rootToken)
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	engine.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("expected create group status=200, got=%d body=%s", createResp.Code, createResp.Body.String())
	}

	var createEnvelope apiEnvelope
	if err := json.Unmarshal(createResp.Body.Bytes(), &createEnvelope); err != nil {
		t.Fatalf("failed to parse create group response: %v", err)
	}
	var createdGroup struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(createEnvelope.Data, &createdGroup); err != nil {
		t.Fatalf("failed to parse created group payload: %v", err)
	}
	if createdGroup.ID == 0 {
		t.Fatalf("expected created group id > 0")
	}

	targetUserID := mustUserIDByUsername(t, db, "viewer2")
	updateGroupsBody, _ := json.Marshal(map[string]any{
		"roleIds": []uint{createdGroup.ID},
	})
	updateGroupsReq := httptest.NewRequest(
		http.MethodPut,
		fmt.Sprintf("/api/system/users/%d/groups", targetUserID),
		bytes.NewReader(updateGroupsBody),
	)
	updateGroupsReq.Header.Set("Authorization", "Bearer "+rootToken)
	updateGroupsReq.Header.Set("Content-Type", "application/json")
	updateGroupsResp := httptest.NewRecorder()
	engine.ServeHTTP(updateGroupsResp, updateGroupsReq)
	if updateGroupsResp.Code != http.StatusOK {
		t.Fatalf("expected update user groups status=200, got=%d body=%s", updateGroupsResp.Code, updateGroupsResp.Body.String())
	}

	usersReq := httptest.NewRequest(http.MethodGet, "/api/system/users?page=1&pageSize=50", nil)
	usersReq.Header.Set("Authorization", "Bearer "+rootToken)
	usersResp := httptest.NewRecorder()
	engine.ServeHTTP(usersResp, usersReq)
	if usersResp.Code != http.StatusOK {
		t.Fatalf("expected users status=200, got=%d body=%s", usersResp.Code, usersResp.Body.String())
	}

	var usersEnvelope apiEnvelope
	if err := json.Unmarshal(usersResp.Body.Bytes(), &usersEnvelope); err != nil {
		t.Fatalf("failed to parse users response: %v", err)
	}
	var usersPayload struct {
		Items []struct {
			Username  string   `json:"username"`
			RoleNames []string `json:"roleNames"`
		} `json:"items"`
	}
	if err := json.Unmarshal(usersEnvelope.Data, &usersPayload); err != nil {
		t.Fatalf("failed to parse users data payload: %v", err)
	}
	foundViewer := false
	for _, item := range usersPayload.Items {
		if item.Username != "viewer2" {
			continue
		}
		foundViewer = true
		if len(item.RoleNames) != 1 || item.RoleNames[0] != "Operations Team" {
			t.Fatalf("expected viewer2 roleNames=[Operations Team], got=%v", item.RoleNames)
		}
	}
	if !foundViewer {
		t.Fatalf("expected to find viewer2 in users list")
	}

	deleteReqInUse := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/system/groups/%d", createdGroup.ID), nil)
	deleteReqInUse.Header.Set("Authorization", "Bearer "+rootToken)
	deleteRespInUse := httptest.NewRecorder()
	engine.ServeHTTP(deleteRespInUse, deleteReqInUse)
	if deleteRespInUse.Code != http.StatusBadRequest {
		t.Fatalf("expected delete in-use group status=400, got=%d body=%s", deleteRespInUse.Code, deleteRespInUse.Body.String())
	}

	viewerRoleID := mustRoleIDByCode(t, db, "staff")
	resetGroupsBody, _ := json.Marshal(map[string]any{
		"roleIds": []uint{viewerRoleID},
	})
	resetGroupsReq := httptest.NewRequest(
		http.MethodPut,
		fmt.Sprintf("/api/system/users/%d/groups", targetUserID),
		bytes.NewReader(resetGroupsBody),
	)
	resetGroupsReq.Header.Set("Authorization", "Bearer "+rootToken)
	resetGroupsReq.Header.Set("Content-Type", "application/json")
	resetGroupsResp := httptest.NewRecorder()
	engine.ServeHTTP(resetGroupsResp, resetGroupsReq)
	if resetGroupsResp.Code != http.StatusOK {
		t.Fatalf("expected reset user groups status=200, got=%d body=%s", resetGroupsResp.Code, resetGroupsResp.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/system/groups/%d", createdGroup.ID), nil)
	deleteReq.Header.Set("Authorization", "Bearer "+rootToken)
	deleteResp := httptest.NewRecorder()
	engine.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("expected delete group status=200, got=%d body=%s", deleteResp.Code, deleteResp.Body.String())
	}
}

func TestM1RootUserCRUD(t *testing.T) {
	engine, db := setupTestServer(t)
	createViewerUser(t, db, "viewer3", "Viewer Three")

	viewerToken, _ := loginAndReadData(t, engine, "viewer3", testDefaultPassword)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	var staffRole model.Role
	if err := db.Where("role_code = ?", "staff").First(&staffRole).Error; err != nil {
		t.Fatalf("failed to load staff role: %v", err)
	}

	viewerCreateBody, _ := json.Marshal(map[string]any{
		"username": "ops_user",
		"realName": "Ops User",
		"status":   "active",
		"roleIds":  []uint{staffRole.ID},
	})
	viewerCreateReq := httptest.NewRequest(http.MethodPost, "/api/system/users", bytes.NewReader(viewerCreateBody))
	viewerCreateReq.Header.Set("Authorization", "Bearer "+viewerToken)
	viewerCreateReq.Header.Set("Content-Type", "application/json")
	viewerCreateResp := httptest.NewRecorder()
	engine.ServeHTTP(viewerCreateResp, viewerCreateReq)
	if viewerCreateResp.Code != http.StatusForbidden {
		t.Fatalf("expected viewer create user status=403, got=%d body=%s", viewerCreateResp.Code, viewerCreateResp.Body.String())
	}

	createBody, _ := json.Marshal(map[string]any{
		"username":           "ops_user",
		"realName":           "Ops User",
		"password":           "Temp#12345",
		"status":             "active",
		"mustChangePassword": true,
		"roleIds":            []uint{staffRole.ID},
		"primaryRoleId":      staffRole.ID,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/system/users", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+rootToken)
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	engine.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("expected create user status=200, got=%d body=%s", createResp.Code, createResp.Body.String())
	}

	var createEnvelope apiEnvelope
	if err := json.Unmarshal(createResp.Body.Bytes(), &createEnvelope); err != nil {
		t.Fatalf("failed to parse create user response: %v", err)
	}
	var createdUser struct {
		ID       uint   `json:"id"`
		Username string `json:"username"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(createEnvelope.Data, &createdUser); err != nil {
		t.Fatalf("failed to parse create user payload: %v", err)
	}
	if createdUser.ID == 0 || createdUser.Username != "ops_user" || createdUser.Status != "active" {
		t.Fatalf("unexpected created user: %+v", createdUser)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"username":           "ops_user",
		"realName":           "Ops Team User",
		"status":             "inactive",
		"mustChangePassword": false,
		"roleIds":            []uint{staffRole.ID},
		"primaryRoleId":      staffRole.ID,
	})
	updateReq := httptest.NewRequest(
		http.MethodPut,
		fmt.Sprintf("/api/system/users/%d", createdUser.ID),
		bytes.NewReader(updateBody),
	)
	updateReq.Header.Set("Authorization", "Bearer "+rootToken)
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp := httptest.NewRecorder()
	engine.ServeHTTP(updateResp, updateReq)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected update user status=200, got=%d body=%s", updateResp.Code, updateResp.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/system/users/%d", createdUser.ID), nil)
	deleteReq.Header.Set("Authorization", "Bearer "+rootToken)
	deleteResp := httptest.NewRecorder()
	engine.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("expected delete user status=200, got=%d body=%s", deleteResp.Code, deleteResp.Body.String())
	}

	var activeUserCount int64
	if err := db.Model(&model.User{}).
		Where("id = ? AND deleted_at IS NULL", createdUser.ID).
		Count(&activeUserCount).Error; err != nil {
		t.Fatalf("failed to query active user count: %v", err)
	}
	if activeUserCount != 0 {
		t.Fatalf("expected deleted user to be filtered by deleted_at, active count=%d", activeUserCount)
	}

	var userRoleCount int64
	if err := db.Model(&model.UserRole{}).
		Where("user_id = ?", createdUser.ID).
		Count(&userRoleCount).Error; err != nil {
		t.Fatalf("failed to query user roles count: %v", err)
	}
	if userRoleCount != 0 {
		t.Fatalf("expected user roles to be cleared after delete, count=%d", userRoleCount)
	}
}

func setupTestServer(t *testing.T) (http.Handler, *gorm.DB) {
	t.Helper()

	cfg := config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 18080,
		},
		Database: config.DatabaseConfig{
			Path: filepath.Join(t.TempDir(), "test.db"),
		},
		MigrationsDir:             "migrations",
		JWTSecret:                 "test-secret",
		DefaultPassword:           testDefaultPassword,
		EnforceMustChangePassword: true,
	}

	db, err := database.NewSQLite(cfg.Database)
	if err != nil {
		t.Fatalf("failed to init sqlite: %v", err)
	}
	manager, err := migration.NewManager(db, cfg.MigrationsDir)
	if err != nil {
		t.Fatalf("failed to init migration manager: %v", err)
	}
	if _, err := manager.Up(t.Context()); err != nil {
		t.Fatalf("failed to apply schema migrations: %v", err)
	}
	if err := database.SeedBaselineData(db, cfg.DefaultPassword); err != nil {
		t.Fatalf("failed to init seed data: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	engine := New(cfg, db)
	return engine, db
}

func createViewerUser(t *testing.T, db *gorm.DB, username, realName string) {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(testDefaultPassword), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := model.User{
		Username:     username,
		PasswordHash: string(hash),
		RealName:     realName,
		Status:       "active",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create viewer user: %v", err)
	}

	var role model.Role
	if err := db.Where("role_code = ?", "staff").First(&role).Error; err != nil {
		t.Fatalf("failed to query staff role: %v", err)
	}
	userRole := model.UserRole{
		UserID:    user.ID,
		RoleID:    role.ID,
		IsPrimary: true,
	}
	if err := db.Create(&userRole).Error; err != nil {
		t.Fatalf("failed to attach viewer role: %v", err)
	}
}

func loginAndReadData(t *testing.T, engine http.Handler, username, password string) (string, json.RawMessage) {
	t.Helper()

	payload := map[string]string{
		"username": username,
		"password": password,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected login status=200 for %s, got=%d body=%s", username, resp.Code, resp.Body.String())
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse login response: %v", err)
	}
	if envelope.Code != 200 {
		t.Fatalf("unexpected business code=%d body=%s", envelope.Code, resp.Body.String())
	}

	token := mustStringField(t, envelope.Data, "token")
	return token, envelope.Data
}

func mustUserIDByUsername(t *testing.T, db *gorm.DB, username string) uint {
	t.Helper()
	var user model.User
	if err := db.Where("username = ? AND deleted_at IS NULL", username).First(&user).Error; err != nil {
		t.Fatalf("failed to query user by username=%s: %v", username, err)
	}
	return user.ID
}

func mustRoleIDByCode(t *testing.T, db *gorm.DB, roleCode string) uint {
	t.Helper()
	var role model.Role
	if err := db.Where("role_code = ?", roleCode).First(&role).Error; err != nil {
		t.Fatalf("failed to query role by role_code=%s: %v", roleCode, err)
	}
	return role.ID
}

func mustStringField(t *testing.T, data json.RawMessage, field string) string {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatalf("failed to parse data payload: %v", err)
	}
	value, ok := object[field]
	if !ok {
		t.Fatalf("missing field %s in payload", field)
	}
	text, ok := value.(string)
	if !ok {
		t.Fatalf("field %s is not string", field)
	}
	return text
}

func mustBoolField(t *testing.T, data json.RawMessage, field string) bool {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatalf("failed to parse data payload: %v", err)
	}
	value, ok := object[field]
	if !ok {
		t.Fatalf("missing field %s in payload", field)
	}
	flag, ok := value.(bool)
	if !ok {
		t.Fatalf("field %s is not bool", field)
	}
	return flag
}
