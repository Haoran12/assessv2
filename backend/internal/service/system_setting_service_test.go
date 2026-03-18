package service

import "testing"

func TestValidateObjectLinkTypesSetting(t *testing.T) {
	t.Parallel()

	validCases := []string{
		`["member","owner","evaluator"]`,
		`["member","OWNER"]`,
	}
	for _, value := range validCases {
		if err := validateSettingValue("assessment.object_link_types", "json", value); err != nil {
			t.Fatalf("expected valid object_link_types value=%s, got err=%v", value, err)
		}
	}

	invalidCases := []string{
		`[]`,
		`{}`,
		`["member","member"]`,
		`["member"," MEMBER "]`,
		`["this_value_is_definitely_longer_than_thirty_chars"]`,
		`[1,2]`,
	}
	for _, value := range invalidCases {
		if err := validateSettingValue("assessment.object_link_types", "json", value); err == nil {
			t.Fatalf("expected invalid object_link_types value=%s", value)
		}
	}
}

func TestValidateVoteGradeScoresSetting(t *testing.T) {
	t.Parallel()

	validCases := []string{
		`{"excellent":100,"good":85,"average":70,"poor":60}`,
		`{"EXCELLENT":100,"good":85.5,"average":70,"poor":60}`,
	}
	for _, value := range validCases {
		if err := validateSettingValue("vote.grade_scores", "json", value); err != nil {
			t.Fatalf("expected valid vote.grade_scores value=%s, got err=%v", value, err)
		}
	}

	invalidCases := []string{
		`{}`,
		`[]`,
		`{"excellent":100,"good":85,"average":70}`,
		`{"excellent":100,"good":85,"average":70,"poor":60,"bad":10}`,
		`{"excellent":100,"good":"85","average":70,"poor":60}`,
		`{"excellent":101,"good":85,"average":70,"poor":60}`,
		`{"excellent":-1,"good":85,"average":70,"poor":60}`,
	}
	for _, value := range invalidCases {
		if err := validateSettingValue("vote.grade_scores", "json", value); err == nil {
			t.Fatalf("expected invalid vote.grade_scores value=%s", value)
		}
	}
}
