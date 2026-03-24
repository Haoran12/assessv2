package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"assessv2/backend/internal/model"
	"gorm.io/gorm"
)

const (
	sessionBusinessDataFileName    = "business_data.json"
	sessionDefaultObjectsFileName  = "default_objects.json"
	sessionBusinessDataFileVersion = 1
)

type sessionBusinessDataFile struct {
	Version      int                                 `json:"version"`
	ExportedAt   int64                               `json:"exportedAt"`
	Session      sessionBusinessDataSessionMeta      `json:"session"`
	Periods      []model.AssessmentSessionPeriod     `json:"periods"`
	ObjectGroups []model.AssessmentObjectGroup       `json:"objectGroups"`
	Objects      []model.AssessmentSessionObject     `json:"objects"`
	ModuleScores []model.AssessmentObjectModuleScore `json:"moduleScores"`
	RuleFiles    []model.RuleFile                    `json:"ruleFiles"`
}

type sessionBusinessDataSessionMeta struct {
	ID             uint   `json:"id"`
	AssessmentName string `json:"assessmentName"`
	DisplayName    string `json:"displayName"`
	Year           int    `json:"year"`
	OrganizationID uint   `json:"organizationId"`
	DataDir        string `json:"dataDir"`
}

type sessionDefaultObjectSnapshotFile struct {
	Version      int                                `json:"version"`
	ExportedAt   int64                              `json:"exportedAt"`
	AssessmentID uint                               `json:"assessmentId"`
	Items        []sessionDefaultObjectSnapshotItem `json:"items"`
}

type sessionDefaultObjectSnapshotItem struct {
	ObjectType       string `json:"objectType"`
	GroupCode        string `json:"groupCode"`
	TargetType       string `json:"targetType"`
	TargetID         uint   `json:"targetId"`
	ObjectName       string `json:"objectName"`
	ParentTargetType string `json:"parentTargetType,omitempty"`
	ParentTargetID   uint   `json:"parentTargetId,omitempty"`
	SortOrder        int    `json:"sortOrder"`
	IsActive         bool   `json:"isActive"`
}

func syncSessionBusinessDataFile(ctx context.Context, summary *AssessmentSessionSummary) error {
	if summary == nil {
		return ErrInvalidParam
	}
	dataDir := resolveSessionDataDir(summary.DataDir, summary.AssessmentName)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create session data directory: %w", err)
	}

	periods := make([]model.AssessmentSessionPeriod, 0, 8)
	objectGroups := make([]model.AssessmentObjectGroup, 0, 16)
	objects := make([]model.AssessmentSessionObject, 0, 128)
	moduleScores := make([]model.AssessmentObjectModuleScore, 0, 256)
	ruleFiles := make([]model.RuleFile, 0, 8)

	if err := withSessionBusinessDB(ctx, summary, func(sessionDB *gorm.DB) error {
		if err := sessionDB.
			Where("assessment_id = ?", summary.ID).
			Order("sort_order ASC, id ASC").
			Find(&periods).Error; err != nil {
			return fmt.Errorf("query session periods for bundle: %w", err)
		}
		if err := sessionDB.
			Where("assessment_id = ?", summary.ID).
			Order("sort_order ASC, id ASC").
			Find(&objectGroups).Error; err != nil {
			return fmt.Errorf("query session object groups for bundle: %w", err)
		}
		if err := sessionDB.
			Where("assessment_id = ?", summary.ID).
			Order("sort_order ASC, id ASC").
			Find(&objects).Error; err != nil {
			return fmt.Errorf("query session objects for bundle: %w", err)
		}
		if err := sessionDB.
			Where("assessment_id = ?", summary.ID).
			Order("period_code ASC, object_id ASC, module_key ASC, id ASC").
			Find(&moduleScores).Error; err != nil {
			return fmt.Errorf("query session module scores for bundle: %w", err)
		}
		if err := sessionDB.
			Where("assessment_id = ?", summary.ID).
			Order("updated_at DESC, id DESC").
			Find(&ruleFiles).Error; err != nil {
			return fmt.Errorf("query session rule files for bundle: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	payload := sessionBusinessDataFile{
		Version:    sessionBusinessDataFileVersion,
		ExportedAt: time.Now().Unix(),
		Session: sessionBusinessDataSessionMeta{
			ID:             summary.ID,
			AssessmentName: summary.AssessmentName,
			DisplayName:    summary.DisplayName,
			Year:           summary.Year,
			OrganizationID: summary.OrganizationID,
			DataDir:        dataDir,
		},
		Periods:      periods,
		ObjectGroups: objectGroups,
		Objects:      objects,
		ModuleScores: moduleScores,
		RuleFiles:    ruleFiles,
	}
	targetPath := filepath.Join(dataDir, sessionBusinessDataFileName)
	if err := writeJSONFile(targetPath, payload); err != nil {
		return fmt.Errorf("write session business data file: %w", err)
	}
	snapshotPath := filepath.Join(dataDir, sessionDefaultObjectsFileName)
	if _, statErr := os.Stat(snapshotPath); os.IsNotExist(statErr) {
		if err := writeDefaultObjectSnapshot(summary, objects); err != nil {
			return err
		}
	}
	return nil
}

func persistSessionDefaultObjectSnapshot(
	ctx context.Context,
	summary *AssessmentSessionSummary,
) error {
	if summary == nil {
		return ErrInvalidParam
	}

	objects := make([]model.AssessmentSessionObject, 0, 128)
	if err := withSessionBusinessDB(ctx, summary, func(sessionDB *gorm.DB) error {
		if err := sessionDB.
			Where("assessment_id = ?", summary.ID).
			Order("sort_order ASC, id ASC").
			Find(&objects).Error; err != nil {
			return fmt.Errorf("query session objects for default snapshot: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	return writeDefaultObjectSnapshot(summary, objects)
}

func writeDefaultObjectSnapshot(summary *AssessmentSessionSummary, objects []model.AssessmentSessionObject) error {
	if summary == nil {
		return ErrInvalidParam
	}
	dataDir := resolveSessionDataDir(summary.DataDir, summary.AssessmentName)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create session data directory for default snapshot: %w", err)
	}

	parentTargetByObjectID := make(map[uint]struct {
		TargetType string
		TargetID   uint
	}, len(objects))
	for _, item := range objects {
		parentTargetByObjectID[item.ID] = struct {
			TargetType string
			TargetID   uint
		}{
			TargetType: item.TargetType,
			TargetID:   item.TargetID,
		}
	}

	items := make([]sessionDefaultObjectSnapshotItem, 0, len(objects))
	for _, item := range objects {
		row := sessionDefaultObjectSnapshotItem{
			ObjectType: item.ObjectType,
			GroupCode:  item.GroupCode,
			TargetType: item.TargetType,
			TargetID:   item.TargetID,
			ObjectName: item.ObjectName,
			SortOrder:  item.SortOrder,
			IsActive:   item.IsActive,
		}
		if item.ParentObjectID != nil && *item.ParentObjectID > 0 {
			if parentTarget, ok := parentTargetByObjectID[*item.ParentObjectID]; ok {
				row.ParentTargetType = parentTarget.TargetType
				row.ParentTargetID = parentTarget.TargetID
			}
		}
		items = append(items, row)
	}

	payload := sessionDefaultObjectSnapshotFile{
		Version:      sessionBusinessDataFileVersion,
		ExportedAt:   time.Now().Unix(),
		AssessmentID: summary.ID,
		Items:        items,
	}

	targetPath := filepath.Join(dataDir, sessionDefaultObjectsFileName)
	if err := writeJSONFile(targetPath, payload); err != nil {
		return fmt.Errorf("write session default object snapshot: %w", err)
	}
	return nil
}

func loadSessionDefaultObjectSnapshot(summary *AssessmentSessionSummary) ([]sessionDefaultObjectSnapshotItem, bool, error) {
	if summary == nil {
		return nil, false, ErrInvalidParam
	}
	dataDir := resolveSessionDataDir(summary.DataDir, summary.AssessmentName)
	snapshotPath := filepath.Join(dataDir, sessionDefaultObjectsFileName)
	raw, err := os.ReadFile(snapshotPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	payload := sessionDefaultObjectSnapshotFile{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, false, err
	}
	return payload.Items, true, nil
}

func writeJSONFile(path string, payload any) error {
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o644); err != nil {
		return err
	}
	_ = os.Remove(path)
	return os.Rename(tmpPath, path)
}

func resolveSessionDataDir(dataDir string, assessmentName string) string {
	text := strings.TrimSpace(dataDir)
	if text == "" {
		root := strings.TrimSpace(os.Getenv("ASSESS_DATA_ROOT"))
		if root == "" {
			root = "data"
		}
		return filepath.Clean(filepath.Join(root, assessmentName))
	}
	if filepath.IsAbs(text) {
		return filepath.Clean(text)
	}

	root := strings.TrimSpace(os.Getenv("ASSESS_DATA_ROOT"))
	if root == "" {
		root = "data"
	}

	normalized := strings.ReplaceAll(text, "\\", "/")
	if strings.HasPrefix(strings.ToLower(normalized), "data/") {
		relative := strings.TrimPrefix(normalized, "data/")
		return filepath.Clean(filepath.Join(root, filepath.FromSlash(relative)))
	}
	return filepath.Clean(filepath.Join(root, filepath.FromSlash(normalized)))
}
