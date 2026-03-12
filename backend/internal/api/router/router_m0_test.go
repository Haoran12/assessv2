package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestM0HealthAndLogin(t *testing.T) {
	engine, _ := setupTestServer(t)

	healthReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthResp := httptest.NewRecorder()
	engine.ServeHTTP(healthResp, healthReq)

	if healthResp.Code != http.StatusOK {
		t.Fatalf("expected /health status=200, got=%d body=%s", healthResp.Code, healthResp.Body.String())
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(healthResp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse /health response: %v", err)
	}
	if envelope.Code != 200 {
		t.Fatalf("expected business code=200, got=%d", envelope.Code)
	}

	token, _ := loginAndReadData(t, engine, "root", testDefaultPassword)
	if token == "" {
		t.Fatalf("expected non-empty token")
	}
}
