package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/database"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

func TestListCalculatedObjects_IndividualDependsOnParentTeam(t *testing.T) {
	fixture := setupCalculationFixture(t)

	if err := fixture.db.Create(&model.AssessmentObjectModuleScore{
		AssessmentID: fixture.sessionID,
		PeriodCode:   "Q1",
		ObjectID:     fixture.teamObjectID,
		ModuleKey:    "base_performance",
		Score:        85,
	}).Error; err != nil {
		t.Fatalf("create module score failed: %v", err)
	}

	rows, err := fixture.service.ListCalculatedObjects(
		context.Background(),
		fixture.claims,
		fixture.sessionID,
		"Q1",
		"dept_main",
	)
	if err != nil {
		t.Fatalf("list calculated objects failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got=%d", len(rows))
	}
	row := rows[0]
	if row.TotalScore == nil {
		t.Fatalf("expected totalScore not nil")
	}
	if !almostEqual(*row.TotalScore, 85) {
		t.Fatalf("unexpected totalScore, got=%v want=85", *row.TotalScore)
	}
	if row.ScoreSource != dependencyTypeObjectParent {
		t.Fatalf("unexpected scoreSource, got=%s want=%s", row.ScoreSource, dependencyTypeObjectParent)
	}
	if row.Rank == nil || *row.Rank != 1 {
		t.Fatalf("unexpected rank, got=%v", row.Rank)
	}
	if row.Grade != "B" {
		t.Fatalf("unexpected grade, got=%s want=B", row.Grade)
	}
}

func TestListCalculatedObjects_YearEndDependsOnQuarter(t *testing.T) {
	fixture := setupCalculationFixture(t)

	moduleScores := []model.AssessmentObjectModuleScore{
		{AssessmentID: fixture.sessionID, PeriodCode: "Q1", ObjectID: fixture.teamObjectID, ModuleKey: "base_performance", Score: 80},
		{AssessmentID: fixture.sessionID, PeriodCode: "Q2", ObjectID: fixture.teamObjectID, ModuleKey: "base_performance", Score: 82},
		{AssessmentID: fixture.sessionID, PeriodCode: "Q3", ObjectID: fixture.teamObjectID, ModuleKey: "base_performance", Score: 84},
		{AssessmentID: fixture.sessionID, PeriodCode: "Q4", ObjectID: fixture.teamObjectID, ModuleKey: "base_performance", Score: 86},
	}
	if err := fixture.db.Create(&moduleScores).Error; err != nil {
		t.Fatalf("create module scores failed: %v", err)
	}

	rows, err := fixture.service.ListCalculatedObjects(
		context.Background(),
		fixture.claims,
		fixture.sessionID,
		"YEAR_END",
		"dept_main",
	)
	if err != nil {
		t.Fatalf("list calculated objects failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got=%d", len(rows))
	}
	row := rows[0]
	if row.TotalScore == nil {
		t.Fatalf("expected totalScore not nil")
	}
	if !almostEqual(*row.TotalScore, 83) {
		t.Fatalf("unexpected totalScore, got=%v want=83", *row.TotalScore)
	}
	if row.ScoreSource != dependencyTypePeriodRollup {
		t.Fatalf("unexpected scoreSource, got=%s want=%s", row.ScoreSource, dependencyTypePeriodRollup)
	}
	if row.Rank == nil || *row.Rank != 1 {
		t.Fatalf("unexpected rank, got=%v", row.Rank)
	}
	if row.Grade != "B" {
		t.Fatalf("unexpected grade, got=%s want=B", row.Grade)
	}
}

func TestListCalculatedObjects_CustomScriptModuleScore(t *testing.T) {
	fixture := setupCalculationFixture(t)
	replaceCalculationFixtureRuleContent(t, fixture, buildRuleContentJSONWithCustomModule(t, "base_performance + 10"))

	if err := fixture.db.Create(&model.AssessmentObjectModuleScore{
		AssessmentID: fixture.sessionID,
		PeriodCode:   "Q1",
		ObjectID:     fixture.individualObjectID,
		ModuleKey:    "base_performance",
		Score:        80,
	}).Error; err != nil {
		t.Fatalf("create module score failed: %v", err)
	}

	rows, err := fixture.service.ListCalculatedObjects(
		context.Background(),
		fixture.claims,
		fixture.sessionID,
		"Q1",
		"dept_main",
	)
	if err != nil {
		t.Fatalf("list calculated objects failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got=%d", len(rows))
	}
	row := rows[0]
	if row.TotalScore == nil {
		t.Fatalf("expected totalScore not nil")
	}
	if !almostEqual(*row.TotalScore, 85) {
		t.Fatalf("unexpected totalScore, got=%v want=85", *row.TotalScore)
	}
	if row.ModuleScores["derived_score"] == nil || !almostEqual(*row.ModuleScores["derived_score"], 90) {
		t.Fatalf("expected derived_score=90, got=%v", row.ModuleScores["derived_score"])
	}
}

func TestListCalculatedObjects_ExtraConditionScript(t *testing.T) {
	fixture := setupCalculationFixture(t)
	replaceCalculationFixtureRuleContent(
		t,
		fixture,
		buildRuleContentJSONWithExtraConditionScript(t, `moduleScores["base_performance"] >= 80`, "or", true),
	)

	if err := fixture.db.Create(&model.AssessmentObjectModuleScore{
		AssessmentID: fixture.sessionID,
		PeriodCode:   "Q1",
		ObjectID:     fixture.individualObjectID,
		ModuleKey:    "base_performance",
		Score:        85,
	}).Error; err != nil {
		t.Fatalf("create module score failed: %v", err)
	}

	rows, err := fixture.service.ListCalculatedObjects(
		context.Background(),
		fixture.claims,
		fixture.sessionID,
		"Q1",
		"dept_main",
	)
	if err != nil {
		t.Fatalf("list calculated objects failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got=%d", len(rows))
	}
	if rows[0].Grade != "A" {
		t.Fatalf("expected grade A via extra condition script, got=%s", rows[0].Grade)
	}
}

func TestListCalculatedObjects_ExtraConditionScriptDisabledByDefault(t *testing.T) {
	fixture := setupCalculationFixture(t)
	replaceCalculationFixtureRuleContent(
		t,
		fixture,
		buildRuleContentJSONWithExtraConditionScript(t, `moduleScores["base_performance"] >= 80`, "or", false),
	)
	if err := fixture.db.Create(&model.AssessmentObjectModuleScore{
		AssessmentID: fixture.sessionID,
		PeriodCode:   "Q1",
		ObjectID:     fixture.individualObjectID,
		ModuleKey:    "base_performance",
		Score:        85,
	}).Error; err != nil {
		t.Fatalf("create module score failed: %v", err)
	}

	rows, err := fixture.service.ListCalculatedObjects(
		context.Background(),
		fixture.claims,
		fixture.sessionID,
		"Q1",
		"dept_main",
	)
	if err != nil {
		t.Fatalf("list calculated objects failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got=%d", len(rows))
	}
	if rows[0].Grade == "A" {
		t.Fatalf("expected extra condition to be ignored when disabled")
	}
}

func TestListCalculatedObjects_CustomScriptRuntimeErrorDefaultsZero(t *testing.T) {
	fixture := setupCalculationFixture(t)
	replaceCalculationFixtureRuleContent(t, fixture, buildRuleContentJSONWithCustomModule(t, "unknown_value + 1"))
	if err := fixture.db.Create(&model.AssessmentObjectModuleScore{
		AssessmentID: fixture.sessionID,
		PeriodCode:   "Q1",
		ObjectID:     fixture.individualObjectID,
		ModuleKey:    "base_performance",
		Score:        80,
	}).Error; err != nil {
		t.Fatalf("create module score failed: %v", err)
	}

	rows, err := fixture.service.ListCalculatedObjects(
		context.Background(),
		fixture.claims,
		fixture.sessionID,
		"Q1",
		"dept_main",
	)
	if err != nil {
		t.Fatalf("expected fallback score instead of error, got=%v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got=%d", len(rows))
	}
	if rows[0].ModuleScores["derived_score"] == nil || !almostEqual(*rows[0].ModuleScores["derived_score"], 0) {
		t.Fatalf("expected derived_score fallback to 0, got=%v", rows[0].ModuleScores["derived_score"])
	}
}

func TestListCalculatedObjects_CustomScriptEmptyDefaultsZero(t *testing.T) {
	fixture := setupCalculationFixture(t)
	replaceCalculationFixtureRuleContent(t, fixture, buildRuleContentJSONWithCustomModule(t, ""))
	if err := fixture.db.Create(&model.AssessmentObjectModuleScore{
		AssessmentID: fixture.sessionID,
		PeriodCode:   "Q1",
		ObjectID:     fixture.individualObjectID,
		ModuleKey:    "base_performance",
		Score:        80,
	}).Error; err != nil {
		t.Fatalf("create module score failed: %v", err)
	}

	rows, err := fixture.service.ListCalculatedObjects(
		context.Background(),
		fixture.claims,
		fixture.sessionID,
		"Q1",
		"dept_main",
	)
	if err != nil {
		t.Fatalf("expected fallback score instead of error, got=%v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got=%d", len(rows))
	}
	if rows[0].ModuleScores["derived_score"] == nil || !almostEqual(*rows[0].ModuleScores["derived_score"], 0) {
		t.Fatalf("expected empty script fallback to 0, got=%v", rows[0].ModuleScores["derived_score"])
	}
}

func TestListCalculatedObjects_CustomScriptLookupFunctions(t *testing.T) {
	fixture := setupCalculationFixture(t)
	replaceCalculationFixtureRuleContent(
		t,
		fixture,
		buildRuleContentJSONWithLookupModuleScript(
			t,
			`score("Q1", `+strconv.FormatUint(uint64(fixture.teamObjectID), 10)+`) + moduleScore("Q1", `+strconv.FormatUint(uint64(fixture.teamObjectID), 10)+`, "base_performance") - 88`,
		),
	)

	moduleScores := []model.AssessmentObjectModuleScore{
		{AssessmentID: fixture.sessionID, PeriodCode: "Q1", ObjectID: fixture.teamObjectID, ModuleKey: "base_performance", Score: 88},
		{AssessmentID: fixture.sessionID, PeriodCode: "Q1", ObjectID: fixture.individualObjectID, ModuleKey: "base_performance", Score: 80},
	}
	if err := fixture.db.Create(&moduleScores).Error; err != nil {
		t.Fatalf("create module scores failed: %v", err)
	}

	rows, err := fixture.service.ListCalculatedObjects(
		context.Background(),
		fixture.claims,
		fixture.sessionID,
		"Q1",
		"dept_main",
	)
	if err != nil {
		t.Fatalf("list calculated objects failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got=%d", len(rows))
	}
	row := rows[0]
	if row.ModuleScores["derived_score"] == nil || !almostEqual(*row.ModuleScores["derived_score"], 88) {
		t.Fatalf("expected derived_score=88, got=%v", row.ModuleScores["derived_score"])
	}
	if row.TotalScore == nil || !almostEqual(*row.TotalScore, 84) {
		t.Fatalf("expected totalScore=84, got=%v", row.TotalScore)
	}
}

func TestListCalculatedObjects_ExtraConditionLookupFunctions(t *testing.T) {
	fixture := setupCalculationFixture(t)
	replaceCalculationFixtureRuleContent(
		t,
		fixture,
		buildRuleContentJSONWithLookupGradeScript(
			t,
			`hasScore("Q1", `+strconv.FormatUint(uint64(fixture.teamObjectID), 10)+`) && targetScore("Q1", "department", 1) >= 85`,
		),
	)

	moduleScores := []model.AssessmentObjectModuleScore{
		{AssessmentID: fixture.sessionID, PeriodCode: "Q1", ObjectID: fixture.teamObjectID, ModuleKey: "base_performance", Score: 88},
		{AssessmentID: fixture.sessionID, PeriodCode: "Q1", ObjectID: fixture.individualObjectID, ModuleKey: "base_performance", Score: 80},
	}
	if err := fixture.db.Create(&moduleScores).Error; err != nil {
		t.Fatalf("create module scores failed: %v", err)
	}

	rows, err := fixture.service.ListCalculatedObjects(
		context.Background(),
		fixture.claims,
		fixture.sessionID,
		"Q1",
		"dept_main",
	)
	if err != nil {
		t.Fatalf("list calculated objects failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got=%d", len(rows))
	}
	if rows[0].Grade != "A" {
		t.Fatalf("expected grade A by lookup functions, got=%s", rows[0].Grade)
	}
}

func TestUpsertModuleScores_VoteModuleCalculatedByBackend(t *testing.T) {
	fixture := setupCalculationFixture(t)
	replaceCalculationFixtureRuleContent(t, fixture, buildRuleContentJSONWithVoteModule(t))

	items, err := fixture.service.UpsertModuleScores(
		context.Background(),
		fixture.claims,
		1,
		fixture.sessionID,
		[]SessionObjectModuleScoreUpsertItem{
			{
				PeriodCode: "Q1",
				ObjectID:   fixture.individualObjectID,
				ModuleKey:  "vote_module",
				Score:      0,
				VoteInput: &SessionVoteInputPayload{
					SubjectVotes: []SessionVoteSubjectInput{
						{
							SubjectLabel: "主体A",
							GradeVotes: []SessionVoteGradeInput{
								{GradeLabel: "优秀", Count: 30},
								{GradeLabel: "良好", Count: 10},
							},
						},
						{
							SubjectLabel: "主体B",
							GradeVotes: []SessionVoteGradeInput{
								{GradeLabel: "一般", Count: 20},
								{GradeLabel: "较差", Count: 20},
							},
						},
					},
				},
			},
		},
		"",
		"",
	)
	if err != nil {
		t.Fatalf("upsert module scores failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got=%d", len(items))
	}
	expectedScore := 83.75
	if !almostEqual(items[0].Score, expectedScore) {
		t.Fatalf("unexpected score, got=%v want=%v", items[0].Score, expectedScore)
	}
	if strings.TrimSpace(items[0].DetailJSON) == "" {
		t.Fatalf("expected detailJson to be generated for vote module")
	}
}

func TestUpsertModuleScores_VoteModuleRequiresVoteInput(t *testing.T) {
	fixture := setupCalculationFixture(t)
	replaceCalculationFixtureRuleContent(t, fixture, buildRuleContentJSONWithVoteModule(t))

	_, err := fixture.service.UpsertModuleScores(
		context.Background(),
		fixture.claims,
		1,
		fixture.sessionID,
		[]SessionObjectModuleScoreUpsertItem{
			{
				PeriodCode: "Q1",
				ObjectID:   fixture.individualObjectID,
				ModuleKey:  "vote_module",
				Score:      88,
				VoteInput:  nil,
			},
		},
		"",
		"",
	)
	if !errors.Is(err, ErrInvalidParam) {
		t.Fatalf("expected ErrInvalidParam, got=%v", err)
	}
}

func TestUpsertModuleScores_ExtraAdjustRangeValidation(t *testing.T) {
	fixture := setupCalculationFixture(t)

	_, err := fixture.service.UpsertModuleScores(
		context.Background(),
		fixture.claims,
		1,
		fixture.sessionID,
		[]SessionObjectModuleScoreUpsertItem{
			{
				PeriodCode: "Q1",
				ObjectID:   fixture.individualObjectID,
				ModuleKey:  extraAdjustModuleKey,
				Score:      extraAdjustScoreMax + 0.01,
			},
		},
		"",
		"",
	)
	if !errors.Is(err, ErrInvalidExtraPointValue) {
		t.Fatalf("expected ErrInvalidExtraPointValue, got=%v", err)
	}
}

func TestListCalculatedObjects_IncludesExtraAdjustModuleScore(t *testing.T) {
	fixture := setupCalculationFixture(t)

	_, err := fixture.service.UpsertModuleScores(
		context.Background(),
		fixture.claims,
		1,
		fixture.sessionID,
		[]SessionObjectModuleScoreUpsertItem{
			{
				PeriodCode: "Q1",
				ObjectID:   fixture.individualObjectID,
				ModuleKey:  "base_performance",
				Score:      80,
			},
			{
				PeriodCode: "Q1",
				ObjectID:   fixture.individualObjectID,
				ModuleKey:  extraAdjustModuleKey,
				Score:      5,
			},
		},
		"",
		"",
	)
	if err != nil {
		t.Fatalf("upsert module scores failed: %v", err)
	}

	rows, err := fixture.service.ListCalculatedObjects(
		context.Background(),
		fixture.claims,
		fixture.sessionID,
		"Q1",
		"dept_main",
	)
	if err != nil {
		t.Fatalf("list calculated objects failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got=%d", len(rows))
	}
	row := rows[0]
	if row.TotalScore == nil || !almostEqual(*row.TotalScore, 85) {
		t.Fatalf("expected totalScore=85, got=%v", row.TotalScore)
	}
	extraScore := row.ModuleScores[extraAdjustModuleKey]
	if extraScore == nil || !almostEqual(*extraScore, 5) {
		t.Fatalf("expected %s=5, got=%v", extraAdjustModuleKey, extraScore)
	}
}

type calculationFixture struct {
	db                 *gorm.DB
	service            *AssessmentSessionService
	claims             *auth.Claims
	sessionID          uint
	teamObjectID       uint
	individualObjectID uint
}

func setupCalculationFixture(t *testing.T) calculationFixture {
	t.Helper()
	t.Setenv("ASSESS_DATA_ROOT", t.TempDir())

	db := openIsolatedSQLiteTestDB(t)
	if err := database.AutoMigrateAndSeed(db, "Test#123456"); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	auditRepo := repository.NewAuditRepository(db)
	service := NewAssessmentSessionService(db, auditRepo)
	claims := &auth.Claims{Roles: []string{auth.RoleRoot}}

	organization := model.Organization{
		OrgName: "Test Org",
		OrgType: "company",
		Status:  "active",
	}
	if err := db.Create(&organization).Error; err != nil {
		t.Fatalf("create organization failed: %v", err)
	}

	session := model.AssessmentSession{
		AssessmentName: "test_assessment",
		DisplayName:    "Test Assessment",
		Year:           2026,
		OrganizationID: organization.ID,
		DataDir:        "data/test_assessment",
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create assessment session failed: %v", err)
	}
	sessionSummary := &AssessmentSessionSummary{AssessmentSession: session}
	sessionDB, closeSessionDB, err := openSessionBusinessDB(sessionSummary)
	if err != nil {
		t.Fatalf("open session business db failed: %v", err)
	}
	t.Cleanup(closeSessionDB)

	periodCodes := []string{"Q1", "Q2", "Q3", "Q4", "YEAR_END"}
	for index, code := range periodCodes {
		period := model.AssessmentSessionPeriod{
			AssessmentID:   session.ID,
			PeriodCode:     code,
			PeriodName:     code,
			RuleBindingKey: code,
			SortOrder:      index + 1,
		}
		if err := sessionDB.Create(&period).Error; err != nil {
			t.Fatalf("create period %s failed: %v", code, err)
		}
	}

	teamObject := model.AssessmentSessionObject{
		AssessmentID: session.ID,
		ObjectType:   ObjectTypeTeam,
		GroupCode:    "dept",
		TargetID:     1,
		TargetType:   "department",
		ObjectName:   "Dept Team",
		SortOrder:    1,
		IsActive:     true,
	}
	if err := sessionDB.Create(&teamObject).Error; err != nil {
		t.Fatalf("create team object failed: %v", err)
	}

	individualObject := model.AssessmentSessionObject{
		AssessmentID:   session.ID,
		ObjectType:     ObjectTypeIndividual,
		GroupCode:      "dept_main",
		TargetID:       2,
		TargetType:     "employee",
		ObjectName:     "Leader A",
		ParentObjectID: &teamObject.ID,
		SortOrder:      2,
		IsActive:       true,
	}
	if err := sessionDB.Create(&individualObject).Error; err != nil {
		t.Fatalf("create individual object failed: %v", err)
	}

	contentJSON := buildCalculationRuleFileJSON(t)
	ruleFile := model.RuleFile{
		AssessmentID: session.ID,
		RuleName:     "Session Rule",
		ContentJSON:  contentJSON,
		FilePath:     "data/rules/test_assessment/session_rule.json",
	}
	if err := sessionDB.Create(&ruleFile).Error; err != nil {
		t.Fatalf("create rule file failed: %v", err)
	}

	return calculationFixture{
		db:                 sessionDB,
		service:            service,
		claims:             claims,
		sessionID:          session.ID,
		teamObjectID:       teamObject.ID,
		individualObjectID: individualObject.ID,
	}
}

func buildCalculationRuleFileJSON(t *testing.T) string {
	t.Helper()
	payload := map[string]any{
		"version": 3,
		"scopedRules": []map[string]any{
			{
				"id":                     "team_rule",
				"applicablePeriods":      []string{"Q1", "Q2", "Q3", "Q4", "YEAR_END"},
				"applicableObjectGroups": []string{"dept"},
				"scoreModules": []map[string]any{
					{
						"id":                "base_performance",
						"moduleKey":         "base_performance",
						"moduleName":        "Base",
						"weight":            100,
						"calculationMethod": "direct_input",
					},
				},
				"grades": defaultGradeRules(),
			},
			{
				"id":                     "individual_rule",
				"applicablePeriods":      []string{"Q1", "Q2", "Q3", "Q4", "YEAR_END"},
				"applicableObjectGroups": []string{"dept_main"},
				"scoreModules": []map[string]any{
					{
						"id":                "base_performance",
						"moduleKey":         "base_performance",
						"moduleName":        "Base",
						"weight":            100,
						"calculationMethod": "direct_input",
					},
				},
				"grades": defaultGradeRules(),
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal rule file payload failed: %v", err)
	}
	return string(raw)
}

func replaceCalculationFixtureRuleContent(t *testing.T, fixture calculationFixture, contentJSON string) {
	t.Helper()
	if err := fixture.db.Model(&model.RuleFile{}).
		Where("assessment_id = ?", fixture.sessionID).
		Update("content_json", contentJSON).Error; err != nil {
		t.Fatalf("update fixture rule content failed: %v", err)
	}
}

func buildRuleContentJSONWithCustomModule(t *testing.T, customScript string) string {
	t.Helper()
	payload := map[string]any{
		"version": 3,
		"scopedRules": []map[string]any{
			{
				"id":                     "individual_rule",
				"applicablePeriods":      []string{"Q1", "Q2", "Q3", "Q4", "YEAR_END"},
				"applicableObjectGroups": []string{"dept_main"},
				"scoreModules": []map[string]any{
					{
						"id":                "base_performance",
						"moduleKey":         "base_performance",
						"moduleName":        "Base",
						"weight":            50,
						"calculationMethod": "direct_input",
					},
					{
						"id":                "derived_score",
						"moduleKey":         "derived_score",
						"moduleName":        "Derived",
						"weight":            50,
						"calculationMethod": "custom_script",
						"customScript":      customScript,
					},
				},
				"grades": defaultGradeRules(),
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal custom module rule file payload failed: %v", err)
	}
	return string(raw)
}

func buildRuleContentJSONWithExtraConditionScript(
	t *testing.T,
	gradeScript string,
	logic string,
	extraConditionEnabled bool,
) string {
	t.Helper()
	payload := map[string]any{
		"version": 3,
		"scopedRules": []map[string]any{
			{
				"id":                     "individual_rule",
				"applicablePeriods":      []string{"Q1", "Q2", "Q3", "Q4", "YEAR_END"},
				"applicableObjectGroups": []string{"dept_main"},
				"scoreModules": []map[string]any{
					{
						"id":                "base_performance",
						"moduleKey":         "base_performance",
						"moduleName":        "Base",
						"weight":            100,
						"calculationMethod": "direct_input",
					},
				},
				"grades": []map[string]any{
					{
						"id":    "grade_a",
						"title": "A",
						"scoreNode": map[string]any{
							"hasUpperLimit": true,
							"upperScore":    100,
							"upperOperator": "<=",
							"hasLowerLimit": true,
							"lowerScore":    90,
							"lowerOperator": ">=",
						},
						"extraConditionEnabled": extraConditionEnabled,
						"extraConditionScript":  gradeScript,
						"conditionLogic":        logic,
					},
					{
						"id":    "grade_b",
						"title": "B",
						"scoreNode": map[string]any{
							"hasUpperLimit": true,
							"upperScore":    89.99,
							"upperOperator": "<=",
							"hasLowerLimit": true,
							"lowerScore":    80,
							"lowerOperator": ">=",
						},
						"extraConditionEnabled": false,
						"extraConditionScript":  "",
						"conditionLogic":        "and",
					},
					{
						"id":    "grade_c",
						"title": "C",
						"scoreNode": map[string]any{
							"hasUpperLimit": true,
							"upperScore":    79.99,
							"upperOperator": "<=",
							"hasLowerLimit": false,
							"lowerScore":    0,
							"lowerOperator": ">=",
						},
						"extraConditionEnabled": false,
						"extraConditionScript":  "",
						"conditionLogic":        "and",
					},
				},
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal extra-condition rule file payload failed: %v", err)
	}
	return string(raw)
}

func buildRuleContentJSONWithLookupModuleScript(t *testing.T, moduleScript string) string {
	t.Helper()
	payload := map[string]any{
		"version": 3,
		"scopedRules": []map[string]any{
			{
				"id":                     "team_rule",
				"applicablePeriods":      []string{"Q1", "Q2", "Q3", "Q4", "YEAR_END"},
				"applicableObjectGroups": []string{"dept"},
				"scoreModules": []map[string]any{
					{
						"id":                "base_performance",
						"moduleKey":         "base_performance",
						"moduleName":        "Base",
						"weight":            100,
						"calculationMethod": "direct_input",
					},
				},
				"grades": defaultGradeRules(),
			},
			{
				"id":                     "individual_rule",
				"applicablePeriods":      []string{"Q1", "Q2", "Q3", "Q4", "YEAR_END"},
				"applicableObjectGroups": []string{"dept_main"},
				"scoreModules": []map[string]any{
					{
						"id":                "base_performance",
						"moduleKey":         "base_performance",
						"moduleName":        "Base",
						"weight":            50,
						"calculationMethod": "direct_input",
					},
					{
						"id":                "derived_score",
						"moduleKey":         "derived_score",
						"moduleName":        "Derived",
						"weight":            50,
						"calculationMethod": "custom_script",
						"customScript":      moduleScript,
					},
				},
				"grades": defaultGradeRules(),
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal lookup module rule file payload failed: %v", err)
	}
	return string(raw)
}

func buildRuleContentJSONWithLookupGradeScript(t *testing.T, gradeScript string) string {
	t.Helper()
	payload := map[string]any{
		"version": 3,
		"scopedRules": []map[string]any{
			{
				"id":                     "team_rule",
				"applicablePeriods":      []string{"Q1", "Q2", "Q3", "Q4", "YEAR_END"},
				"applicableObjectGroups": []string{"dept"},
				"scoreModules": []map[string]any{
					{
						"id":                "base_performance",
						"moduleKey":         "base_performance",
						"moduleName":        "Base",
						"weight":            100,
						"calculationMethod": "direct_input",
					},
				},
				"grades": defaultGradeRules(),
			},
			{
				"id":                     "individual_rule",
				"applicablePeriods":      []string{"Q1", "Q2", "Q3", "Q4", "YEAR_END"},
				"applicableObjectGroups": []string{"dept_main"},
				"scoreModules": []map[string]any{
					{
						"id":                "base_performance",
						"moduleKey":         "base_performance",
						"moduleName":        "Base",
						"weight":            100,
						"calculationMethod": "direct_input",
					},
				},
				"grades": []map[string]any{
					{
						"id":    "grade_a",
						"title": "A",
						"scoreNode": map[string]any{
							"hasUpperLimit": true,
							"upperScore":    100,
							"upperOperator": "<=",
							"hasLowerLimit": true,
							"lowerScore":    90,
							"lowerOperator": ">=",
						},
						"extraConditionEnabled": true,
						"extraConditionScript":  gradeScript,
						"conditionLogic":        "or",
					},
					{
						"id":    "grade_b",
						"title": "B",
						"scoreNode": map[string]any{
							"hasUpperLimit": true,
							"upperScore":    89.99,
							"upperOperator": "<=",
							"hasLowerLimit": true,
							"lowerScore":    80,
							"lowerOperator": ">=",
						},
						"extraConditionEnabled": false,
						"extraConditionScript":  "",
						"conditionLogic":        "and",
					},
				},
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal lookup grade rule file payload failed: %v", err)
	}
	return string(raw)
}

func buildRuleContentJSONWithVoteModule(t *testing.T) string {
	t.Helper()
	payload := map[string]any{
		"version": 3,
		"scopedRules": []map[string]any{
			{
				"id":                     "individual_rule",
				"applicablePeriods":      []string{"Q1", "Q2", "Q3", "Q4", "YEAR_END"},
				"applicableObjectGroups": []string{"dept_main"},
				"scoreModules": []map[string]any{
					{
						"id":                "vote_module",
						"moduleKey":         "vote_module",
						"moduleName":        "Vote Module",
						"weight":            100,
						"calculationMethod": "vote",
						"detail": map[string]any{
							"voteConfig": map[string]any{
								"gradeScores": []map[string]any{
									{"label": "优秀", "score": 100},
									{"label": "良好", "score": 85},
									{"label": "一般", "score": 70},
									{"label": "较差", "score": 60},
								},
								"voterSubjects": []map[string]any{
									{"label": "主体A", "weight": 0.6},
									{"label": "主体B", "weight": 0.4},
								},
							},
						},
					},
				},
				"grades": defaultGradeRules(),
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal vote module rule file payload failed: %v", err)
	}
	return string(raw)
}

func defaultGradeRules() []map[string]any {
	return []map[string]any{
		{
			"id":    "grade_a",
			"title": "A",
			"scoreNode": map[string]any{
				"hasUpperLimit": true,
				"upperScore":    100,
				"upperOperator": "<=",
				"hasLowerLimit": true,
				"lowerScore":    90,
				"lowerOperator": ">=",
			},
			"conditionLogic": "and",
		},
		{
			"id":    "grade_b",
			"title": "B",
			"scoreNode": map[string]any{
				"hasUpperLimit": true,
				"upperScore":    89.99,
				"upperOperator": "<=",
				"hasLowerLimit": true,
				"lowerScore":    80,
				"lowerOperator": ">=",
			},
			"conditionLogic": "and",
		},
		{
			"id":    "grade_c",
			"title": "C",
			"scoreNode": map[string]any{
				"hasUpperLimit": true,
				"upperScore":    79.99,
				"upperOperator": "<=",
				"hasLowerLimit": false,
				"lowerScore":    0,
				"lowerOperator": ">=",
			},
			"conditionLogic": "and",
		},
	}
}

func almostEqual(left, right float64) bool {
	return math.Abs(left-right) < 0.000001
}
