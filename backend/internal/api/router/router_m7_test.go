package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
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

func TestM7SettingsAuditRollback(t *testing.T) {
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

	rollbackReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/system/audit-logs/%d/rollback", auditID), bytes.NewReader([]byte("{}")))
	rollbackReq.Header.Set("Authorization", "Bearer "+rootToken)
	rollbackReq.Header.Set("Content-Type", "application/json")
	rollbackResp := httptest.NewRecorder()
	engine.ServeHTTP(rollbackResp, rollbackReq)
	if rollbackResp.Code != http.StatusOK {
		t.Fatalf("expected audit rollback status=200, got=%d body=%s", rollbackResp.Code, rollbackResp.Body.String())
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
	if systemName != "AssessV2" {
		t.Fatalf("expected rolled back system.name=AssessV2, got=%s", systemName)
	}
}
