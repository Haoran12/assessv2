package service

import "testing"

func floatPtr(value float64) *float64 {
	return &value
}

func TestCalculateTotalScore(t *testing.T) {
	modules := []RuleEngineScoreModule{
		{ModuleKey: "m1", Weight: 70},
		{ModuleKey: "m2", Weight: 30},
	}
	moduleScores := map[string]float64{
		"m1": 80,
		"m2": 90,
	}

	got := CalculateTotalScore(moduleScores, modules, 5)
	want := 88.0
	if got != want {
		t.Fatalf("unexpected total score, got=%v want=%v", got, want)
	}
}

func TestAssignGradesByGroupWithQuotaIteration(t *testing.T) {
	items := []RuleEngineObject{
		{ObjectID: 1, GroupKey: "g1", TotalScore: 98},
		{ObjectID: 2, GroupKey: "g1", TotalScore: 95},
		{ObjectID: 3, GroupKey: "g1", TotalScore: 93},
		{ObjectID: 4, GroupKey: "g1", TotalScore: 88},
		{ObjectID: 5, GroupKey: "g1", TotalScore: 80},
	}

	gradeRules := []RuleEngineGradeRule{
		{
			Title:    "A",
			MaxRatio: floatPtr(0.2),
			ScoreNode: RuleEngineGradeScoreNode{
				HasLowerLimit: true,
				LowerScore:    90,
				LowerOperator: ">=",
			},
		},
		{
			Title:    "B",
			MaxRatio: floatPtr(0.4),
			ScoreNode: RuleEngineGradeScoreNode{
				HasLowerLimit: true,
				LowerScore:    85,
				LowerOperator: ">=",
			},
		},
		{
			Title: "C",
			ScoreNode: RuleEngineGradeScoreNode{
				HasLowerLimit: true,
				LowerScore:    0,
				LowerOperator: ">=",
			},
		},
	}

	result := AssignGradesByGroup(items, gradeRules, nil)
	if len(result) != len(items) {
		t.Fatalf("unexpected result length: %d", len(result))
	}

	gradeByID := map[uint]string{}
	for _, item := range result {
		gradeByID[item.ObjectID] = item.Grade
	}

	if gradeByID[1] != "A" {
		t.Fatalf("object 1 should be A, got=%s", gradeByID[1])
	}
	if gradeByID[2] != "B" || gradeByID[3] != "B" {
		t.Fatalf("object 2 and 3 should be B, got=%s/%s", gradeByID[2], gradeByID[3])
	}
	if gradeByID[4] != "C" || gradeByID[5] != "C" {
		t.Fatalf("object 4 and 5 should be C, got=%s/%s", gradeByID[4], gradeByID[5])
	}
}

func TestAssignGradesConditionLogic(t *testing.T) {
	items := []RuleEngineObject{
		{ObjectID: 1, GroupKey: "g1", TotalScore: 85},
	}

	gradeRules := []RuleEngineGradeRule{
		{
			Title:                "A",
			ExtraConditionScript: "custom",
			ConditionLogic:       "or",
			ScoreNode: RuleEngineGradeScoreNode{
				HasLowerLimit: true,
				LowerScore:    90,
				LowerOperator: ">=",
			},
		},
	}

	evaluator := func(object RuleEngineObject, rule RuleEngineGradeRule) (bool, error) {
		return object.ObjectID == 1 && rule.Title == "A", nil
	}

	result := AssignGradesByGroup(items, gradeRules, evaluator)
	if result[0].Grade != "A" {
		t.Fatalf("expected grade A, got=%s", result[0].Grade)
	}
}
