package service

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/database"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

func TestCreateSessionPersistsDefaultSnapshotInSessionDB(t *testing.T) {
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
	businessDataPath := filepath.Join(sessionDir, "business_data.json")
	defaultObjectsPath := filepath.Join(sessionDir, "default_objects.json")

	if _, statErr := os.Stat(businessDataPath); !os.IsNotExist(statErr) {
		t.Fatalf("business_data.json should not be generated at runtime")
	}
	if _, statErr := os.Stat(defaultObjectsPath); !os.IsNotExist(statErr) {
		t.Fatalf("default_objects.json should not be generated at runtime")
	}

	summary := &AssessmentSessionSummary{AssessmentSession: detail.Session.AssessmentSession}
	err = withSessionBusinessDB(context.Background(), summary, func(sessionDB *gorm.DB) error {
		var snapshotCount int64
		if err := sessionDB.
			Model(&model.SessionDefaultObjectSnapshot{}).
			Where("assessment_id = ?", detail.Session.ID).
			Count(&snapshotCount).Error; err != nil {
			return err
		}
		if snapshotCount == 0 {
			t.Fatalf("expected session default snapshot rows in session db")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("query session db snapshot failed: %v", err)
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
