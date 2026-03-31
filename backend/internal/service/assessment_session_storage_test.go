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

func TestCreateSessionDoesNotPersistDefaultSnapshotBeforeActive(t *testing.T) {
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
		if snapshotCount != 0 {
			t.Fatalf("expected no session default snapshot rows before active status, got=%d", snapshotCount)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("query session db snapshot failed: %v", err)
	}
}

func TestCreateSessionCanCopyConfigIntoIndependentSessionStorage(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("ASSESS_DATA_ROOT", dataRoot)

	db := openIsolatedSQLiteTestDB(t)
	if err := database.AutoMigrateAndSeed(db, "Test#123456"); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	sourceOrg := model.Organization{
		OrgName: "Source Org",
		OrgType: "company",
		Status:  "active",
	}
	if err := db.Create(&sourceOrg).Error; err != nil {
		t.Fatalf("create source organization failed: %v", err)
	}
	targetOrg := model.Organization{
		OrgName: "Target Org",
		OrgType: "company",
		Status:  "active",
	}
	if err := db.Create(&targetOrg).Error; err != nil {
		t.Fatalf("create target organization failed: %v", err)
	}

	sourceDept := model.Department{
		OrganizationID: sourceOrg.ID,
		DeptName:       "Source Dept",
		Status:         "active",
	}
	if err := db.Create(&sourceDept).Error; err != nil {
		t.Fatalf("create source department failed: %v", err)
	}
	targetDept := model.Department{
		OrganizationID: targetOrg.ID,
		DeptName:       "Target Dept",
		Status:         "active",
	}
	if err := db.Create(&targetDept).Error; err != nil {
		t.Fatalf("create target department failed: %v", err)
	}

	service := NewAssessmentSessionService(db, repository.NewAuditRepository(db))
	claims := &auth.Claims{Roles: []string{auth.RoleRoot}}
	ctx := context.Background()

	sourceDetail, err := service.CreateSession(
		ctx,
		claims,
		1,
		CreateAssessmentSessionInput{
			Year:           2026,
			OrganizationID: sourceOrg.ID,
			DisplayName:    "来源场次",
		},
		"127.0.0.1",
		"unit-test",
	)
	if err != nil {
		t.Fatalf("create source session failed: %v", err)
	}

	customPeriods := []SessionPeriodItem{
		{PeriodCode: "M1", PeriodName: "月度一", RuleBindingKey: "M1", SortOrder: 1},
		{PeriodCode: "M2", PeriodName: "月度二", RuleBindingKey: "M1", SortOrder: 2},
		{PeriodCode: "FINAL", PeriodName: "年度总评", RuleBindingKey: "FINAL", SortOrder: 3},
	}
	if _, err := service.ReplacePeriods(ctx, claims, 1, sourceDetail.Session.ID, customPeriods, "127.0.0.1", "unit-test"); err != nil {
		t.Fatalf("replace source periods failed: %v", err)
	}

	customGroups := []SessionObjectGroupItem{
		{ObjectType: ObjectTypeTeam, GroupCode: "team_core", GroupName: "核心团队", SortOrder: 1},
		{ObjectType: ObjectTypeIndividual, GroupCode: "manager", GroupName: "管理人员", SortOrder: 2},
	}
	if _, err := service.ReplaceObjectGroups(ctx, claims, 1, sourceDetail.Session.ID, customGroups, "127.0.0.1", "unit-test"); err != nil {
		t.Fatalf("replace source object groups failed: %v", err)
	}

	sourceSummary, err := service.loadSessionSummary(ctx, sourceDetail.Session.ID)
	if err != nil {
		t.Fatalf("load source session summary failed: %v", err)
	}

	sourceRuleContent := `{"version":3,"scopedRules":[{"id":"copied_rule","applicablePeriods":["M1"],"applicableObjectGroups":["team_core"],"scoreModules":[{"id":"vote_module","moduleKey":"vote_module","moduleName":"投票得分","weight":80,"calculationMethod":"direct_input","customScript":""}],"grades":[{"id":"grade_a","title":"A","scoreNode":{"hasUpperLimit":true,"upperScore":100,"upperOperator":"<=","hasLowerLimit":true,"lowerScore":90,"lowerOperator":">="},"extraConditionScript":"","conditionLogic":"and","maxRatioPercent":null,"maxRatioRoundingMode":"real"}]}]}`
	sourceRulePath, err := ensureRuleFilePath(sourceSummary.AssessmentName, buildRuleFileName("来源规则"))
	if err != nil {
		t.Fatalf("resolve source rule file path failed: %v", err)
	}
	if err := os.WriteFile(sourceRulePath, []byte(sourceRuleContent), 0o644); err != nil {
		t.Fatalf("write source rule file failed: %v", err)
	}
	if err := withSessionBusinessDB(ctx, sourceSummary, func(sessionDB *gorm.DB) error {
		return sessionDB.Create(&model.RuleFile{
			AssessmentID: sourceSummary.ID,
			RuleName:     "来源规则",
			Description:  "来源场次规则文件",
			ContentJSON:  sourceRuleContent,
			FilePath:     sourceRulePath,
			CreatedBy:    uintPtr(1),
			UpdatedBy:    uintPtr(1),
		}).Error
	}); err != nil {
		t.Fatalf("insert source rule file failed: %v", err)
	}

	targetDetail, err := service.CreateSession(
		ctx,
		claims,
		1,
		CreateAssessmentSessionInput{
			Year:              2026,
			OrganizationID:    targetOrg.ID,
			DisplayName:       "目标场次",
			CopyFromSessionID: sourceDetail.Session.ID,
		},
		"127.0.0.1",
		"unit-test",
	)
	if err != nil {
		t.Fatalf("create copied session failed: %v", err)
	}

	if len(targetDetail.Periods) != len(customPeriods) {
		t.Fatalf("expected copied periods count=%d, got=%d", len(customPeriods), len(targetDetail.Periods))
	}
	for index, item := range customPeriods {
		got := targetDetail.Periods[index]
		if got.PeriodCode != item.PeriodCode || got.PeriodName != item.PeriodName || got.RuleBindingKey != item.RuleBindingKey {
			t.Fatalf("unexpected copied period at index %d: %+v", index, got)
		}
	}

	if len(targetDetail.ObjectGroups) != len(customGroups) {
		t.Fatalf("expected copied object groups count=%d, got=%d", len(customGroups), len(targetDetail.ObjectGroups))
	}
	for index, item := range customGroups {
		got := targetDetail.ObjectGroups[index]
		if got.ObjectType != item.ObjectType || got.GroupCode != item.GroupCode || got.GroupName != item.GroupName {
			t.Fatalf("unexpected copied object group at index %d: %+v", index, got)
		}
	}

	targetSummary, err := service.loadSessionSummary(ctx, targetDetail.Session.ID)
	if err != nil {
		t.Fatalf("load target session summary failed: %v", err)
	}
	targetRuleFile, err := service.pickSessionRuleFile(ctx, targetSummary)
	if err != nil {
		t.Fatalf("load target rule file failed: %v", err)
	}
	if targetRuleFile.AssessmentID != targetDetail.Session.ID {
		t.Fatalf("target rule file assessment_id mismatch: %d", targetRuleFile.AssessmentID)
	}
	if targetRuleFile.FilePath == sourceRulePath {
		t.Fatalf("target rule file should not reuse source path: %s", targetRuleFile.FilePath)
	}
	expectedRuleContent, err := normalizeRuleContentByPeriodBindings(sourceRuleContent, targetDetail.Periods)
	if err != nil {
		t.Fatalf("normalize expected copied rule content failed: %v", err)
	}
	if targetRuleFile.ContentJSON != expectedRuleContent {
		t.Fatalf("unexpected copied rule content: %s", targetRuleFile.ContentJSON)
	}
	targetRuleBytes, err := os.ReadFile(targetRuleFile.FilePath)
	if err != nil {
		t.Fatalf("read target rule file failed: %v", err)
	}
	if string(targetRuleBytes) != expectedRuleContent {
		t.Fatalf("target rule file content mismatch: %s", string(targetRuleBytes))
	}

	targetObjects, err := service.ListObjects(ctx, claims, targetDetail.Session.ID)
	if err != nil {
		t.Fatalf("list target objects failed: %v", err)
	}
	targetObjectKeys := toObjectTargetKeySet(targetObjects)
	if _, exists := targetObjectKeys["department:"+uintToString(targetDept.ID)]; !exists {
		t.Fatalf("expected target default objects to include target department")
	}
	if _, exists := targetObjectKeys["department:"+uintToString(sourceDept.ID)]; exists {
		t.Fatalf("target default objects should not copy source department objects")
	}
}

func TestResetObjectsToDefaultUsesCurrentOrgTreeWhenPreparing(t *testing.T) {
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

	newDeptKey := "department:" + uintToString(addedDept.ID)
	if _, exists := afterKeys[newDeptKey]; !exists {
		t.Fatalf("reset in preparing should pull newly added department from current org tree: %s", newDeptKey)
	}
}

func TestResetObjectsToDefaultUsesSnapshotWhenActive(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("ASSESS_DATA_ROOT", dataRoot)

	db := openIsolatedSQLiteTestDB(t)
	if err := database.AutoMigrateAndSeed(db, "Test#123456"); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	org := model.Organization{
		OrgName: "Active Snapshot Org",
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
			DisplayName:    "2026 Active Snapshot Session",
		},
		"127.0.0.1",
		"unit-test",
	)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	if _, err := service.UpdateSessionStatus(
		context.Background(),
		claims,
		1,
		detail.Session.ID,
		UpdateAssessmentSessionStatusInput{Status: AssessmentSessionStatusActive},
		"127.0.0.1",
		"unit-test",
	); err != nil {
		t.Fatalf("set session to active failed: %v", err)
	}

	addedDept := model.Department{
		OrganizationID: org.ID,
		DeptName:       "New Dept After Active",
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

	newDeptKey := "department:" + uintToString(addedDept.ID)
	if _, exists := afterKeys[newDeptKey]; exists {
		t.Fatalf("reset in active should use snapshot and ignore newly added department: %s", newDeptKey)
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
