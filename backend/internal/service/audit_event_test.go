package service

import (
	"strings"
	"testing"
)

func TestNormalizeAuditDetailBuildsChangesAndSummary(t *testing.T) {
	targetID := uint(11)
	payload, eventCode, summary, changes := normalizeAuditDetail(
		"update",
		"users",
		&targetID,
		map[string]any{
			"before": map[string]any{
				"status": "active",
				"name":   "alice",
			},
			"after": map[string]any{
				"status": "inactive",
				"name":   "alice",
				"email":  "alice@example.com",
			},
		},
	)

	if eventCode != "update.users" {
		t.Fatalf("expected eventCode=update.users, got=%s", eventCode)
	}
	if strings.TrimSpace(summary) == "" {
		t.Fatalf("expected summary to be generated")
	}
	if payload["version"] != auditDetailVersionV2 {
		t.Fatalf("expected version=%s, got=%v", auditDetailVersionV2, payload["version"])
	}
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got=%d", len(changes))
	}

	byField := map[string]AuditDiffItem{}
	for _, item := range changes {
		byField[item.Field] = item
	}
	if byField["status"].ChangeType != "updated" {
		t.Fatalf("expected status changeType=updated, got=%s", byField["status"].ChangeType)
	}
	if byField["email"].ChangeType != "added" {
		t.Fatalf("expected email changeType=added, got=%s", byField["email"].ChangeType)
	}
}

func TestNormalizeAuditDetailMasksSensitiveFields(t *testing.T) {
	payload, _, _, changes := normalizeAuditDetail(
		"update",
		"users",
		nil,
		map[string]any{
			"before": map[string]any{
				"password": "old-pass",
				"profile": map[string]any{
					"secret_key": "old-secret",
				},
			},
			"after": map[string]any{
				"password": "new-pass",
				"profile": map[string]any{
					"secret_key": "new-secret",
				},
			},
		},
	)

	before := pickMap(payload, "before")
	after := pickMap(payload, "after")
	if before["password"] != "***" || after["password"] != "***" {
		t.Fatalf("expected password fields to be masked, before=%v after=%v", before["password"], after["password"])
	}

	profileBefore := pickMap(before, "profile")
	profileAfter := pickMap(after, "profile")
	if profileBefore["secret_key"] != "***" || profileAfter["secret_key"] != "***" {
		t.Fatalf("expected nested secret fields to be masked")
	}

	for _, item := range changes {
		if strings.Contains(strings.ToLower(item.Field), "password") {
			if item.Before != "***" || item.After != "***" {
				t.Fatalf("expected masked diff values for password fields")
			}
		}
	}
}

func TestBuildAuditRecordSetsDiffMetadata(t *testing.T) {
	operatorID := uint(7)
	targetID := uint(9)
	record := buildAuditRecord(
		&operatorID,
		"update",
		"users",
		&targetID,
		map[string]any{
			"before": map[string]any{"status": "active"},
			"after":  map[string]any{"status": "inactive"},
		},
		"127.0.0.1",
		"unit-test",
	)

	if record.ChangeCount != 1 {
		t.Fatalf("expected ChangeCount=1, got=%d", record.ChangeCount)
	}
	if !record.HasDiff {
		t.Fatalf("expected HasDiff=true")
	}
	if strings.TrimSpace(record.EventCode) == "" {
		t.Fatalf("expected EventCode to be set")
	}
	if strings.TrimSpace(record.Summary) == "" {
		t.Fatalf("expected Summary to be set")
	}
	if !strings.Contains(record.ActionDetail, "\"changes\"") {
		t.Fatalf("expected action detail to contain changes payload")
	}
}
