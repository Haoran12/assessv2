package service

import (
	"sort"
	"strings"
)

type RuleEngineScoreModule struct {
	ModuleKey         string
	Weight            float64
	CalculationMethod string
	CustomScript      string
}

type RuleEngineGradeScoreNode struct {
	HasUpperLimit bool
	UpperScore    float64
	UpperOperator string
	HasLowerLimit bool
	LowerScore    float64
	LowerOperator string
}

type RuleEngineGradeRule struct {
	Title                 string
	ScoreNode             RuleEngineGradeScoreNode
	ExtraConditionScript  string
	ExtraConditionEnabled bool
	ConditionLogic        string
	// MaxRatio is in [0,1], nil means no limit.
	MaxRatio *float64
}

type RuleEngineObject struct {
	ObjectID       uint
	GroupKey       string
	PeriodCode     string
	ObjectType     string
	TargetID       uint
	TargetType     string
	ParentObjectID *uint

	ModuleScores map[string]float64
	ExtraAdjust  float64

	TotalScore float64
	Rank       int
	Grade      string
}

type GradeConditionEvaluator func(object RuleEngineObject, rule RuleEngineGradeRule) (bool, error)

func CalculateWeightedScore(moduleScores map[string]float64, modules []RuleEngineScoreModule) float64 {
	totalWeight := 0.0
	weightedSum := 0.0
	for _, module := range modules {
		if module.Weight <= 0 {
			continue
		}
		score := moduleScores[module.ModuleKey]
		weightedSum += score * module.Weight
		totalWeight += module.Weight
	}
	if totalWeight <= 0 {
		return 0
	}
	return weightedSum / totalWeight
}

func CalculateTotalScore(moduleScores map[string]float64, modules []RuleEngineScoreModule, extraAdjust float64) float64 {
	return CalculateWeightedScore(moduleScores, modules) + extraAdjust
}

func RankObjectsByGroup(items []RuleEngineObject) []RuleEngineObject {
	result := make([]RuleEngineObject, len(items))
	copy(result, items)

	groupIndexes := buildGroupIndex(result)
	for _, indexes := range groupIndexes {
		sort.SliceStable(indexes, func(i, j int) bool {
			left := result[indexes[i]]
			right := result[indexes[j]]
			if left.TotalScore == right.TotalScore {
				return left.ObjectID < right.ObjectID
			}
			return left.TotalScore > right.TotalScore
		})
		for rank, idx := range indexes {
			result[idx].Rank = rank + 1
		}
	}
	return result
}

func AssignGradesByGroup(
	items []RuleEngineObject,
	gradeRules []RuleEngineGradeRule,
	evaluator GradeConditionEvaluator,
) []RuleEngineObject {
	ranked := RankObjectsByGroup(items)
	if len(gradeRules) == 0 || len(ranked) == 0 {
		return ranked
	}

	groupIndexes := buildGroupIndex(ranked)
	for _, indexes := range groupIndexes {
		assignGradesForGroup(ranked, indexes, gradeRules, evaluator)
	}
	return ranked
}

func assignGradesForGroup(
	items []RuleEngineObject,
	groupIndexes []int,
	gradeRules []RuleEngineGradeRule,
	evaluator GradeConditionEvaluator,
) {
	for _, idx := range groupIndexes {
		items[idx].Grade = ""
		for _, rule := range gradeRules {
			if matchGradeRule(items[idx], rule, evaluator) {
				items[idx].Grade = rule.Title
				break
			}
		}
	}

	if !hasAnyQuota(gradeRules) {
		return
	}

	groupSize := len(groupIndexes)
	maxRounds := len(gradeRules)*groupSize + 1
	for round := 0; round < maxRounds; round++ {
		changed := false

		for gradeIndex, rule := range gradeRules {
			limit, limited := quotaLimit(rule, groupSize)
			if !limited {
				continue
			}

			current := make([]int, 0, groupSize)
			for _, idx := range groupIndexes {
				if items[idx].Grade == rule.Title {
					current = append(current, idx)
				}
			}
			if len(current) <= limit {
				continue
			}

			sort.SliceStable(current, func(i, j int) bool {
				left := items[current[i]]
				right := items[current[j]]
				if left.Rank == right.Rank {
					if left.TotalScore == right.TotalScore {
						return left.ObjectID < right.ObjectID
					}
					return left.TotalScore > right.TotalScore
				}
				return left.Rank < right.Rank
			})

			overflow := current[limit:]
			for _, idx := range overflow {
				for lowerIndex := gradeIndex + 1; lowerIndex < len(gradeRules); lowerIndex++ {
					lowerRule := gradeRules[lowerIndex]
					if matchGradeRule(items[idx], lowerRule, evaluator) {
						if items[idx].Grade != lowerRule.Title {
							items[idx].Grade = lowerRule.Title
							changed = true
						}
						break
					}
				}
			}
		}

		if !changed || !hasQuotaViolation(items, groupIndexes, gradeRules, groupSize) {
			break
		}
	}
}

func matchGradeRule(item RuleEngineObject, rule RuleEngineGradeRule, evaluator GradeConditionEvaluator) bool {
	node := rule.ScoreNode
	scorePass := true
	if node.HasLowerLimit {
		switch node.LowerOperator {
		case ">":
			scorePass = scorePass && item.TotalScore > node.LowerScore
		default:
			scorePass = scorePass && item.TotalScore >= node.LowerScore
		}
	}
	if node.HasUpperLimit {
		switch node.UpperOperator {
		case "<":
			scorePass = scorePass && item.TotalScore < node.UpperScore
		default:
			scorePass = scorePass && item.TotalScore <= node.UpperScore
		}
	}

	if !rule.ExtraConditionEnabled || strings.TrimSpace(rule.ExtraConditionScript) == "" {
		return scorePass
	}

	conditionPass := false
	if evaluator != nil {
		ok, err := evaluator(item, rule)
		if err == nil {
			conditionPass = ok
		}
	}

	if normalizeConditionLogic(rule.ConditionLogic) == "or" {
		return scorePass || conditionPass
	}
	return scorePass && conditionPass
}

func normalizeConditionLogic(value string) string {
	if value == "or" || value == "OR" {
		return "or"
	}
	return "and"
}

func buildGroupIndex(items []RuleEngineObject) map[string][]int {
	result := map[string][]int{}
	for idx, item := range items {
		key := item.GroupKey
		result[key] = append(result[key], idx)
	}
	return result
}

func hasAnyQuota(gradeRules []RuleEngineGradeRule) bool {
	for _, rule := range gradeRules {
		if _, limited := quotaLimit(rule, 1); limited {
			return true
		}
	}
	return false
}

func quotaLimit(rule RuleEngineGradeRule, groupSize int) (int, bool) {
	if rule.MaxRatio == nil {
		return 0, false
	}
	ratio := *rule.MaxRatio
	if ratio >= 1 {
		return groupSize, true
	}
	if ratio <= 0 {
		return 0, true
	}
	limit := int(float64(groupSize) * ratio)
	if limit < 0 {
		limit = 0
	}
	if limit > groupSize {
		limit = groupSize
	}
	return limit, true
}

func hasQuotaViolation(items []RuleEngineObject, groupIndexes []int, gradeRules []RuleEngineGradeRule, groupSize int) bool {
	for _, rule := range gradeRules {
		limit, limited := quotaLimit(rule, groupSize)
		if !limited {
			continue
		}
		count := 0
		for _, idx := range groupIndexes {
			if items[idx].Grade == rule.Title {
				count++
			}
		}
		if count > limit {
			return true
		}
	}
	return false
}
