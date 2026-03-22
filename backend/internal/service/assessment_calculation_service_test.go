package service

import (
	"context"
	"encoding/json"
	"math"
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

	periodCodes := []string{"Q1", "Q2", "Q3", "Q4", "YEAR_END"}
	for index, code := range periodCodes {
		period := model.AssessmentSessionPeriod{
			AssessmentID:   session.ID,
			PeriodCode:     code,
			PeriodName:     code,
			RuleBindingKey: code,
			SortOrder:      index + 1,
		}
		if err := db.Create(&period).Error; err != nil {
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
	if err := db.Create(&teamObject).Error; err != nil {
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
	if err := db.Create(&individualObject).Error; err != nil {
		t.Fatalf("create individual object failed: %v", err)
	}

	contentJSON := buildCalculationRuleFileJSON(t)
	ruleFile := model.RuleFile{
		AssessmentID: session.ID,
		RuleName:     "Session Rule",
		ContentJSON:  contentJSON,
		FilePath:     "data/rules/test_assessment/session_rule.json",
	}
	if err := db.Create(&ruleFile).Error; err != nil {
		t.Fatalf("create rule file failed: %v", err)
	}

	return calculationFixture{
		db:                 db,
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
