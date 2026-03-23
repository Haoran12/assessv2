package service

import (
	"fmt"
	"strconv"
	"strings"

	"assessv2/backend/internal/model"
)

type expressionScoreLookup struct {
	totalScores      map[string]float64
	moduleScores     map[string]map[string]float64
	objectIDByTarget map[string]uint
}

func newExpressionScoreLookup(
	periods []model.AssessmentSessionPeriod,
	objects []model.AssessmentSessionObject,
	rawScoresByNode map[string]map[string]float64,
) *expressionScoreLookup {
	lookup := &expressionScoreLookup{
		totalScores:      make(map[string]float64, len(rawScoresByNode)),
		moduleScores:     make(map[string]map[string]float64, len(rawScoresByNode)),
		objectIDByTarget: make(map[string]uint, len(objects)),
	}
	for _, object := range objects {
		targetKey := buildExpressionTargetKey(object.TargetType, object.TargetID)
		if targetKey != "" {
			lookup.objectIDByTarget[targetKey] = object.ID
		}
	}
	for node, scores := range rawScoresByNode {
		lookup.moduleScores[node] = cloneFloatMap(scores)
	}
	return lookup
}

func (l *expressionScoreLookup) setNodeTotal(periodCode string, objectID uint, value float64) {
	if l == nil {
		return
	}
	key := buildDependencyNode(periodCode, objectID)
	l.totalScores[key] = value
}

func (l *expressionScoreLookup) setNodeModuleScores(periodCode string, objectID uint, scores map[string]float64) {
	if l == nil {
		return
	}
	key := buildDependencyNode(periodCode, objectID)
	if len(scores) == 0 {
		l.moduleScores[key] = map[string]float64{}
		return
	}
	l.moduleScores[key] = cloneFloatMap(scores)
}

func (l *expressionScoreLookup) score(periodCode string, objectID any) (float64, bool) {
	if l == nil {
		return 0, false
	}
	normalizedPeriod := normalizeExpressionPeriodCode(periodCode)
	parsedObjectID, ok := parseExpressionUint(objectID)
	if !ok {
		return 0, false
	}
	key := buildDependencyNode(normalizedPeriod, parsedObjectID)
	value, exists := l.totalScores[key]
	if !exists {
		return 0, false
	}
	return value, true
}

func (l *expressionScoreLookup) moduleScore(periodCode string, objectID any, moduleKey string) (float64, bool) {
	if l == nil {
		return 0, false
	}
	normalizedPeriod := normalizeExpressionPeriodCode(periodCode)
	parsedObjectID, ok := parseExpressionUint(objectID)
	if !ok {
		return 0, false
	}
	key := buildDependencyNode(normalizedPeriod, parsedObjectID)
	modules, exists := l.moduleScores[key]
	if !exists || len(modules) == 0 {
		return 0, false
	}
	value, moduleExists := modules[strings.TrimSpace(moduleKey)]
	if !moduleExists {
		return 0, false
	}
	return value, true
}

func (l *expressionScoreLookup) targetScore(periodCode string, targetType string, targetID any) (float64, bool) {
	if l == nil {
		return 0, false
	}
	parsedTargetID, ok := parseExpressionUint(targetID)
	if !ok {
		return 0, false
	}
	objectID, exists := l.objectIDByTarget[buildExpressionTargetKey(targetType, parsedTargetID)]
	if !exists {
		return 0, false
	}
	return l.score(periodCode, objectID)
}

func (l *expressionScoreLookup) hasScore(periodCode string, objectID any) bool {
	_, exists := l.score(periodCode, objectID)
	return exists
}

func (l *expressionScoreLookup) expressionFunctions() map[string]any {
	return map[string]any{
		"score": func(periodCode string, objectID any) float64 {
			value, _ := l.score(periodCode, objectID)
			return value
		},
		"moduleScore": func(periodCode string, objectID any, moduleKey string) float64 {
			value, _ := l.moduleScore(periodCode, objectID, moduleKey)
			return value
		},
		"targetScore": func(periodCode string, targetType string, targetID any) float64 {
			value, _ := l.targetScore(periodCode, targetType, targetID)
			return value
		},
		"hasScore": func(periodCode string, objectID any) bool {
			return l.hasScore(periodCode, objectID)
		},
	}
}

func normalizeExpressionPeriodCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func buildExpressionTargetKey(targetType string, targetID uint) string {
	normalizedTargetType := strings.ToLower(strings.TrimSpace(targetType))
	if normalizedTargetType == "" || targetID == 0 {
		return ""
	}
	return fmt.Sprintf("%s:%d", normalizedTargetType, targetID)
}

func parseExpressionUint(value any) (uint, bool) {
	switch typed := value.(type) {
	case uint:
		return typed, true
	case uint64:
		return uint(typed), true
	case uint32:
		return uint(typed), true
	case uint16:
		return uint(typed), true
	case uint8:
		return uint(typed), true
	case int:
		if typed <= 0 {
			return 0, false
		}
		return uint(typed), true
	case int64:
		if typed <= 0 {
			return 0, false
		}
		return uint(typed), true
	case int32:
		if typed <= 0 {
			return 0, false
		}
		return uint(typed), true
	case int16:
		if typed <= 0 {
			return 0, false
		}
		return uint(typed), true
	case int8:
		if typed <= 0 {
			return 0, false
		}
		return uint(typed), true
	case float64:
		if typed <= 0 {
			return 0, false
		}
		return uint(typed), true
	case float32:
		if typed <= 0 {
			return 0, false
		}
		return uint(typed), true
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return 0, false
		}
		parsed, err := strconv.ParseUint(text, 10, 64)
		if err != nil || parsed == 0 {
			return 0, false
		}
		return uint(parsed), true
	default:
		return 0, false
	}
}
