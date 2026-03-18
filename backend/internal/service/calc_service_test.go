package service

import (
	"testing"

	"assessv2/backend/internal/model"
)

func TestParseVoteGradeScoresSetting(t *testing.T) {
	t.Parallel()

	parsed, err := parseVoteGradeScoresSetting(`{"EXCELLENT":95,"good":80,"average":70,"poor":60}`)
	if err != nil {
		t.Fatalf("expected valid vote grade scores, got err=%v", err)
	}
	if len(parsed) != 4 || !floatEquals(parsed["excellent"], 95) || !floatEquals(parsed["good"], 80) {
		t.Fatalf("unexpected parsed scores: %+v", parsed)
	}

	invalidCases := []string{
		`{"excellent":95,"good":80,"average":70}`,
		`{"excellent":95,"good":80,"average":70,"poor":60,"bad":1}`,
		`{"excellent":120,"good":80,"average":70,"poor":60}`,
	}
	for _, value := range invalidCases {
		if _, err := parseVoteGradeScoresSetting(value); err == nil {
			t.Fatalf("expected invalid vote grade score setting value=%s", value)
		}
	}
}

func TestCalculateVoteModuleRawScoreUsesConfiguredScores(t *testing.T) {
	t.Parallel()

	module := model.ScoreModule{ModuleCode: "vote"}
	groups := []model.VoteGroup{
		{ID: 1, Weight: 1, MaxScore: 100},
	}
	rows := []voteAggRow{
		{VoteGroupID: 1, GradeOption: "excellent", Count: 1},
		{VoteGroupID: 1, GradeOption: "good", Count: 1},
	}

	configured := map[string]float64{
		"excellent": 90,
		"good":      80,
		"average":   70,
		"poor":      60,
	}
	got := calculateVoteModuleRawScore(module, groups, rows, configured)
	if !floatEquals(got, 85) {
		t.Fatalf("expected configured score=85, got=%v", got)
	}

	defaultScore := calculateVoteModuleRawScore(module, groups, rows, nil)
	if !floatEquals(defaultScore, 92.5) {
		t.Fatalf("expected default score=92.5, got=%v", defaultScore)
	}
}
