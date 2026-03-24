package service

import (
	"encoding/json"
	"strings"
)

type calculationRuleContent struct {
	Version      int                 `json:"version"`
	ScopedRules  []calculationScoped `json:"scopedRules"`
	Dependencies []map[string]any    `json:"dependencies"`
	Raw          map[string]any      `json:"-"`
}

type calculationScoped struct {
	ID                    string                 `json:"id"`
	ApplicablePeriods     []string               `json:"applicablePeriods"`
	ApplicableObjectGroup []string               `json:"applicableObjectGroups"`
	ScoreModules          []calculationScoreNode `json:"scoreModules"`
	Grades                []calculationGradeRule `json:"grades"`
}

type calculationScoreNode struct {
	ID                string         `json:"id"`
	ModuleKey         string         `json:"moduleKey"`
	ModuleName        string         `json:"moduleName"`
	Weight            float64        `json:"weight"`
	CalculationMethod string         `json:"calculationMethod"`
	CustomScript      string         `json:"customScript"`
	Detail            map[string]any `json:"detail,omitempty"`
	VoteConfig        map[string]any `json:"voteConfig,omitempty"`
}

type calculationGradeRule struct {
	ID                    string                    `json:"id"`
	Title                 string                    `json:"title"`
	ScoreNode             calculationGradeScoreNode `json:"scoreNode"`
	ExtraConditionScript  string                    `json:"extraConditionScript"`
	ExtraConditionEnabled *bool                     `json:"extraConditionEnabled,omitempty"`
	ConditionLogic        string                    `json:"conditionLogic"`
	MaxRatioPercent       *float64                  `json:"maxRatioPercent"`
}

type calculationGradeScoreNode struct {
	HasUpperLimit bool    `json:"hasUpperLimit"`
	UpperScore    float64 `json:"upperScore"`
	UpperOperator string  `json:"upperOperator"`
	HasLowerLimit bool    `json:"hasLowerLimit"`
	LowerScore    float64 `json:"lowerScore"`
	LowerOperator string  `json:"lowerOperator"`
}

func parseCalculationRuleContent(raw string) (*calculationRuleContent, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, ErrRuleNotFound
	}
	contentRaw := map[string]any{}
	if err := json.Unmarshal([]byte(text), &contentRaw); err != nil {
		return nil, err
	}
	content := &calculationRuleContent{}
	if err := json.Unmarshal([]byte(text), content); err != nil {
		return nil, err
	}
	content.Raw = contentRaw
	return content, nil
}

func matchScopedRule(content *calculationRuleContent, periodCode string, groupCode string) *calculationScoped {
	if content == nil {
		return nil
	}
	targetPeriod := strings.ToUpper(strings.TrimSpace(periodCode))
	targetGroup := strings.TrimSpace(groupCode)
	for index := range content.ScopedRules {
		item := &content.ScopedRules[index]
		if !containsPeriod(item.ApplicablePeriods, targetPeriod) {
			continue
		}
		if !containsGroupCode(item.ApplicableObjectGroup, targetGroup) {
			continue
		}
		return item
	}
	return nil
}

func containsPeriod(periods []string, target string) bool {
	if target == "" {
		return false
	}
	for _, item := range periods {
		if strings.ToUpper(strings.TrimSpace(item)) == target {
			return true
		}
	}
	return false
}

func containsGroupCode(groups []string, target string) bool {
	if target == "" {
		return false
	}
	for _, item := range groups {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}

func toScoreModules(modules []calculationScoreNode) []RuleEngineScoreModule {
	result := make([]RuleEngineScoreModule, 0, len(modules))
	for _, item := range modules {
		key := strings.TrimSpace(item.ModuleKey)
		if key == "" {
			key = strings.TrimSpace(item.ID)
		}
		if key == "" {
			continue
		}
		weight := item.Weight
		if weight <= 0 {
			continue
		}
		result = append(result, RuleEngineScoreModule{
			ModuleKey:         key,
			Weight:            weight,
			CalculationMethod: normalizeCalculationMethod(item.CalculationMethod),
			CustomScript:      strings.TrimSpace(item.CustomScript),
		})
	}
	return result
}

func toGradeRules(grades []calculationGradeRule) []RuleEngineGradeRule {
	result := make([]RuleEngineGradeRule, 0, len(grades))
	for _, item := range grades {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}
		rule := RuleEngineGradeRule{
			Title:                 title,
			ExtraConditionScript:  strings.TrimSpace(item.ExtraConditionScript),
			ExtraConditionEnabled: resolveGradeExtraConditionEnabled(item),
			ScoreNode: RuleEngineGradeScoreNode{
				HasUpperLimit: item.ScoreNode.HasUpperLimit,
				UpperScore:    item.ScoreNode.UpperScore,
				UpperOperator: normalizeUpperOperator(item.ScoreNode.UpperOperator),
				HasLowerLimit: item.ScoreNode.HasLowerLimit,
				LowerScore:    item.ScoreNode.LowerScore,
				LowerOperator: normalizeLowerOperator(item.ScoreNode.LowerOperator),
			},
			ConditionLogic: normalizeConditionLogic(item.ConditionLogic),
		}
		if item.MaxRatioPercent != nil {
			percent := *item.MaxRatioPercent
			if percent > 0 && percent <= 100 {
				ratio := percent / 100
				rule.MaxRatio = &ratio
			}
		}
		result = append(result, rule)
	}
	return result
}

func resolveGradeExtraConditionEnabled(rule calculationGradeRule) bool {
	if rule.ExtraConditionEnabled != nil {
		return *rule.ExtraConditionEnabled
	}
	// Backward compatibility for legacy rules without explicit switch.
	return strings.TrimSpace(rule.ExtraConditionScript) != ""
}

func normalizeUpperOperator(value string) string {
	if strings.TrimSpace(value) == "<" {
		return "<"
	}
	return "<="
}

func normalizeLowerOperator(value string) string {
	if strings.TrimSpace(value) == ">" {
		return ">"
	}
	return ">="
}

func normalizeCalculationMethod(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "custom_script":
		return "custom_script"
	case "vote":
		return "vote"
	default:
		return "direct_input"
	}
}
