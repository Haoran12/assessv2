package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/database"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
)

func TestCreateSessionWritesBusinessDataFiles(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("ASSESS_DATA_ROOT", dataRoot)

	db := openIsolatedSQLiteTestDB(t)
	if err := database.AutoMigrateAndSeed(db, "Test#123456"); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	org := model.Organization{
		OrgName: "Storage Org",
		OrgType: "company",
		Status:  "active",
	}
	if err := db.Create(&org).Error; err != nil {
		t.Fatalf("create organization failed: %v", err)
	}
	dept := model.Department{
		OrganizationID: org.ID,
		DeptName:       "Dept A",
		Status:         "active",
	}
	if err := db.Create(&dept).Error; err != nil {
		t.Fatalf("create department failed: %v", err)
	}

	service := NewAssessmentSessionService(db, repository.NewAuditRepository(db))
	claims := &auth.Claims{Roles: []string{auth.RoleRoot}}

	detail, err := service.CreateSession(
		context.Background(),
		claims,
		1,
		CreateAssessmentSessionInput{
			Year:           2026,
			OrganizationID: org.ID,
			DisplayName:    "2026测试场次",
		},
		"127.0.0.1",
		"unit-test",
	)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	sessionDir := detail.Session.DataDir
	businessDataPath := filepath.Join(sessionDir, sessionBusinessDataFileName)
	defaultObjectsPath := filepath.Join(sessionDir, sessionDefaultObjectsFileName)

	if _, statErr := os.Stat(businessDataPath); statErr != nil {
		t.Fatalf("business data file not found: %v", statErr)
	}
	if _, statErr := os.Stat(defaultObjectsPath); statErr != nil {
		t.Fatalf("default object snapshot file not found: %v", statErr)
	}

	raw, err := os.ReadFile(businessDataPath)
	if err != nil {
		t.Fatalf("read business data file failed: %v", err)
	}
	payload := sessionBusinessDataFile{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal business data file failed: %v", err)
	}
	if payload.Session.ID != detail.Session.ID {
		t.Fatalf("unexpected session id in business data, got=%d want=%d", payload.Session.ID, detail.Session.ID)
	}
	if len(payload.Periods) == 0 || len(payload.ObjectGroups) == 0 || len(payload.Objects) == 0 {
		t.Fatalf("business data file missing session payload, periods=%d groups=%d objects=%d", len(payload.Periods), len(payload.ObjectGroups), len(payload.Objects))
	}
}

func TestResetObjectsToDefaultUsesSnapshotInsteadOfCurrentOrgTree(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("ASSESS_DATA_ROOT", dataRoot)

	db := openIsolatedSQLiteTestDB(t)
	if err := database.AutoMigrateAndSeed(db, "Test#123456"); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	org := model.Organization{
		OrgName: "Snapshot Org",
		OrgType: "company",
		Status:  "active",
	}
	if err := db.Create(&org).Error; err != nil {
		t.Fatalf("create organization failed: %v", err)
	}

	initialDept := model.Department{
		OrganizationID: org.ID,
		DeptName:       "Init Dept",
		Status:         "active",
	}
	if err := db.Create(&initialDept).Error; err != nil {
		t.Fatalf("create initial department failed: %v", err)
	}

	service := NewAssessmentSessionService(db, repository.NewAuditRepository(db))
	claims := &auth.Claims{Roles: []string{auth.RoleRoot}}
	detail, err := service.CreateSession(
		context.Background(),
		claims,
		1,
		CreateAssessmentSessionInput{
			Year:           2026,
			OrganizationID: org.ID,
			DisplayName:    "2026快照场次",
		},
		"127.0.0.1",
		"unit-test",
	)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	before, err := service.ListObjects(context.Background(), claims, detail.Session.ID)
	if err != nil {
		t.Fatalf("list objects before org change failed: %v", err)
	}
	beforeKeys := toObjectTargetKeySet(before)

	addedDept := model.Department{
		OrganizationID: org.ID,
		DeptName:       "New Dept After Session Created",
		Status:         "active",
	}
	if err := db.Create(&addedDept).Error; err != nil {
		t.Fatalf("create added department failed: %v", err)
	}

	afterReset, err := service.ResetObjectsToDefault(
		context.Background(),
		claims,
		1,
		detail.Session.ID,
		"127.0.0.1",
		"unit-test",
	)
	if err != nil {
		t.Fatalf("reset objects to default failed: %v", err)
	}
	afterKeys := toObjectTargetKeySet(afterReset)

	if len(beforeKeys) != len(afterKeys) {
		t.Fatalf("unexpected object count after reset, before=%d after=%d", len(beforeKeys), len(afterKeys))
	}
	for key := range beforeKeys {
		if _, exists := afterKeys[key]; !exists {
			t.Fatalf("object key lost after reset: %s", key)
		}
	}
	newDeptKey := "department:" + uintToString(addedDept.ID)
	if _, exists := afterKeys[newDeptKey]; exists {
		t.Fatalf("reset should not pull newly added department from current org tree: %s", newDeptKey)
	}
}

func toObjectTargetKeySet(items []model.AssessmentSessionObject) map[string]struct{} {
	result := make(map[string]struct{}, len(items))
	for _, item := range items {
		result[item.TargetType+":"+uintToString(item.TargetID)] = struct{}{}
	}
	return result
}

func uintToString(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}
