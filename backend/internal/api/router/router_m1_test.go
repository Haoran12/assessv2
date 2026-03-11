package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"assessv2/backend/internal/config"
	"assessv2/backend/internal/database"
	"assessv2/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	testDefaultPassword = "#2026@hdwl"
	testChangedPassword = "NewPass#2026"
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
		JWTSecret:       "test-secret",
		DefaultPassword: testDefaultPassword,
	}

	db, err := database.NewSQLite(cfg.Database)
	if err != nil {
		t.Fatalf("failed to init sqlite: %v", err)
	}
	if err := database.AutoMigrateAndSeed(db, cfg.DefaultPassword); err != nil {
		t.Fatalf("failed to init schema: %v", err)
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
	if err := db.Where("role_code = ?", "viewer").First(&role).Error; err != nil {
		t.Fatalf("failed to query viewer role: %v", err)
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
