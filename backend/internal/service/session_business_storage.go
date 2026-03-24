package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"assessv2/backend/internal/model"
	"gorm.io/gorm"
)

const legacySessionDefaultObjectsFileName = "default_objects.json"

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

// syncSessionBusinessDataFile is intentionally a no-op.
// Session business data is sourced from data/{assessment}/assess.db only.
func syncSessionBusinessDataFile(_ context.Context, _ *AssessmentSessionSummary) error {
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
		items := buildSnapshotItemsFromObjects(objects)
		return saveSessionDefaultObjectSnapshotRows(sessionDB, summary.ID, items)
	}); err != nil {
		return err
	}
	return nil
}

func loadSessionDefaultObjectSnapshot(summary *AssessmentSessionSummary) ([]sessionDefaultObjectSnapshotItem, bool, error) {
	if summary == nil {
		return nil, false, ErrInvalidParam
	}

	rows := make([]model.SessionDefaultObjectSnapshot, 0, 128)
	if err := withSessionBusinessDB(context.Background(), summary, func(sessionDB *gorm.DB) error {
		if err := sessionDB.
			Where("assessment_id = ?", summary.ID).
			Order("sort_order ASC, id ASC").
			Find(&rows).Error; err != nil {
			return fmt.Errorf("query session default snapshot rows: %w", err)
		}
		return nil
	}); err != nil {
		return nil, false, err
	}
	if len(rows) == 0 {
		return nil, false, nil
	}

	items := make([]sessionDefaultObjectSnapshotItem, 0, len(rows))
	for _, row := range rows {
		item := sessionDefaultObjectSnapshotItem{
			ObjectType: row.ObjectType,
			GroupCode:  row.GroupCode,
			TargetType: row.TargetType,
			TargetID:   row.TargetID,
			ObjectName: row.ObjectName,
			SortOrder:  row.SortOrder,
			IsActive:   row.IsActive,
		}
		if strings.TrimSpace(row.ParentTargetType) != "" && row.ParentTargetID > 0 {
			item.ParentTargetType = row.ParentTargetType
			item.ParentTargetID = row.ParentTargetID
		}
		items = append(items, item)
	}
	return items, true, nil
}

func saveSessionDefaultObjectSnapshotRows(
	sessionDB *gorm.DB,
	assessmentID uint,
	items []sessionDefaultObjectSnapshotItem,
) error {
	if sessionDB == nil || assessmentID == 0 {
		return ErrInvalidParam
	}
	if err := sessionDB.Where("assessment_id = ?", assessmentID).Delete(&model.SessionDefaultObjectSnapshot{}).Error; err != nil {
		return fmt.Errorf("clear session default snapshot rows: %w", err)
	}
	if len(items) == 0 {
		return nil
	}

	rows := make([]model.SessionDefaultObjectSnapshot, 0, len(items))
	for _, item := range items {
		row := model.SessionDefaultObjectSnapshot{
			AssessmentID: assessmentID,
			ObjectType:   item.ObjectType,
			GroupCode:    item.GroupCode,
			TargetType:   item.TargetType,
			TargetID:     item.TargetID,
			ObjectName:   item.ObjectName,
			SortOrder:    item.SortOrder,
			IsActive:     item.IsActive,
		}
		if strings.TrimSpace(item.ParentTargetType) != "" && item.ParentTargetID > 0 {
			row.ParentTargetType = item.ParentTargetType
			row.ParentTargetID = item.ParentTargetID
		}
		rows = append(rows, row)
	}
	if err := sessionDB.CreateInBatches(rows, 200).Error; err != nil {
		return fmt.Errorf("insert session default snapshot rows: %w", err)
	}
	return nil
}

func buildSnapshotItemsFromObjects(objects []model.AssessmentSessionObject) []sessionDefaultObjectSnapshotItem {
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
	return items
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
