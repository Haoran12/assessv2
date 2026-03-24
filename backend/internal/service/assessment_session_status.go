package service

import (
	"strings"

	"assessv2/backend/internal/model"
)

const (
	AssessmentSessionStatusPreparing = model.AssessmentSessionStatusPreparing
	AssessmentSessionStatusActive    = model.AssessmentSessionStatusActive
	AssessmentSessionStatusCompleted = model.AssessmentSessionStatusCompleted
)

func normalizeAssessmentSessionStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case AssessmentSessionStatusPreparing:
		return AssessmentSessionStatusPreparing
	case AssessmentSessionStatusActive:
		return AssessmentSessionStatusActive
	case AssessmentSessionStatusCompleted:
		return AssessmentSessionStatusCompleted
	default:
		return ""
	}
}

func assessmentSessionStatusOrDefault(raw string) string {
	normalized := normalizeAssessmentSessionStatus(raw)
	if normalized != "" {
		return normalized
	}
	return AssessmentSessionStatusPreparing
}

func isAssessmentSessionReadOnly(raw string) bool {
	return assessmentSessionStatusOrDefault(raw) == AssessmentSessionStatusCompleted
}

func useAssessmentObjectSnapshotMode(raw string) bool {
	status := assessmentSessionStatusOrDefault(raw)
	return status == AssessmentSessionStatusActive || status == AssessmentSessionStatusCompleted
}
