package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"assessv2/backend/internal/model"
)

func (s *RuleManagementService) normalizeRuleContentByPeriodBindings(
	ctx context.Context,
	sessionID uint,
	contentJSON string,
) (string, error) {
	text := strings.TrimSpace(contentJSON)
	if text == "" || sessionID == 0 {
		return contentJSON, nil
	}

	periods, err := s.listSessionPeriods(ctx, sessionID)
	if err != nil {
		return "", err
	}
	if len(periods) == 0 {
		return contentJSON, nil
	}

	normalized, err := normalizeRuleContentByPeriodBindings(text, periods)
	if err != nil {
		return "", fmt.Errorf("failed to normalize rule content by period bindings: %w", err)
	}
	return normalized, nil
}

func normalizeRuleContentByPeriodBindings(contentJSON string, periods []model.AssessmentSessionPeriod) (string, error) {
	raw := map[string]any{}
	if err := json.Unmarshal([]byte(contentJSON), &raw); err != nil {
		// Keep current behavior: invalid JSON is validated later.
		return contentJSON, nil
	}

	scopedRaw, ok := raw["scopedRules"]
	if !ok {
		return contentJSON, nil
	}
	scopedList, ok := scopedRaw.([]any)
	if !ok {
		return contentJSON, nil
	}

	index := buildPeriodBindingIndex(periods)
	if len(index.periodToBinding) == 0 {
		return contentJSON, nil
	}

	// First pass: expand each scoped rule period set by ruleBindingKey peers.
	for i := range scopedList {
		row, ok := scopedList[i].(map[string]any)
		if !ok {
			continue
		}
		periodCodes := normalizeUpperCodeList(anyToStringSlice(row["applicablePeriods"]))
		if len(periodCodes) == 0 {
			continue
		}
		row["applicablePeriods"] = expandPeriodCodesByBinding(periodCodes, index)
		row["applicableObjectGroups"] = normalizeGroupCodeList(anyToStringSlice(row["applicableObjectGroups"]))
	}

	// Second pass: de-conflict period+group ownership so one combination maps to one scoped rule.
	ownerByKey := map[string]int{}
	for i := range scopedList {
		row, ok := scopedList[i].(map[string]any)
		if !ok {
			continue
		}
		periodCodes := normalizeUpperCodeList(anyToStringSlice(row["applicablePeriods"]))
		groupCodes := normalizeGroupCodeList(anyToStringSlice(row["applicableObjectGroups"]))
		if len(periodCodes) == 0 || len(groupCodes) == 0 {
			row["applicablePeriods"] = periodCodes
			row["applicableObjectGroups"] = groupCodes
			continue
		}

		filteredPeriods := make([]string, 0, len(periodCodes))
		for _, periodCode := range periodCodes {
			conflict := false
			for _, groupCode := range groupCodes {
				key := periodCode + "|" + groupCode
				if owner, exists := ownerByKey[key]; exists && owner != i {
					conflict = true
					break
				}
			}
			if conflict {
				continue
			}
			filteredPeriods = append(filteredPeriods, periodCode)
			for _, groupCode := range groupCodes {
				ownerByKey[periodCode+"|"+groupCode] = i
			}
		}
		row["applicablePeriods"] = filteredPeriods
		row["applicableObjectGroups"] = groupCodes
	}

	raw["scopedRules"] = scopedList
	bytes, err := json.Marshal(raw)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

type periodBindingIndex struct {
	periodToBinding  map[string]string
	bindingToPeriods map[string][]string
}

func buildPeriodBindingIndex(periods []model.AssessmentSessionPeriod) periodBindingIndex {
	result := periodBindingIndex{
		periodToBinding:  make(map[string]string, len(periods)),
		bindingToPeriods: make(map[string][]string, len(periods)),
	}
	seenByBinding := map[string]map[string]struct{}{}
	for _, period := range periods {
		code := strings.ToUpper(strings.TrimSpace(period.PeriodCode))
		if code == "" {
			continue
		}
		bindingKey := strings.ToUpper(strings.TrimSpace(period.RuleBindingKey))
		if bindingKey == "" {
			bindingKey = code
		}
		result.periodToBinding[code] = bindingKey
		if _, exists := seenByBinding[bindingKey]; !exists {
			seenByBinding[bindingKey] = map[string]struct{}{}
		}
		if _, exists := seenByBinding[bindingKey][code]; exists {
			continue
		}
		seenByBinding[bindingKey][code] = struct{}{}
		result.bindingToPeriods[bindingKey] = append(result.bindingToPeriods[bindingKey], code)
	}
	return result
}

func expandPeriodCodesByBinding(periodCodes []string, index periodBindingIndex) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(periodCodes))
	for _, periodCode := range periodCodes {
		code := strings.ToUpper(strings.TrimSpace(periodCode))
		if code == "" {
			continue
		}
		bindingKey, exists := index.periodToBinding[code]
		if !exists {
			if _, hit := seen[code]; !hit {
				seen[code] = struct{}{}
				result = append(result, code)
			}
			continue
		}
		peers := index.bindingToPeriods[bindingKey]
		if len(peers) == 0 {
			if _, hit := seen[code]; !hit {
				seen[code] = struct{}{}
				result = append(result, code)
			}
			continue
		}
		for _, peer := range peers {
			if _, hit := seen[peer]; hit {
				continue
			}
			seen[peer] = struct{}{}
			result = append(result, peer)
		}
	}
	return result
}

func anyToStringSlice(value any) []string {
	result := make([]string, 0, 8)
	switch items := value.(type) {
	case []string:
		for _, item := range items {
			text := strings.TrimSpace(item)
			if text != "" {
				result = append(result, text)
			}
		}
	case []any:
		for _, item := range items {
			if item == nil {
				continue
			}
			text, ok := item.(string)
			if !ok {
				continue
			}
			text = strings.TrimSpace(text)
			if text != "" {
				result = append(result, text)
			}
		}
	}
	return result
}

func normalizeUpperCodeList(items []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		code := strings.ToUpper(strings.TrimSpace(item))
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result
}

func normalizeGroupCodeList(items []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		code := strings.TrimSpace(item)
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result
}
