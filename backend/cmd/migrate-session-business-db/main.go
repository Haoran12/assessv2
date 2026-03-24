package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"assessv2/backend/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const sessionBusinessSQLiteFileName = "assess.db"
const legacySessionDefaultObjectsFileName = "default_objects.json"

type sessionRow struct {
	ID             uint
	AssessmentName string
	DataDir        string
}

type tableCopyStats struct {
	Periods          int
	ObjectGroups     int
	Objects          int
	ModuleScores     int
	RuleFiles        int
	DefaultSnapshots int
}

type legacySessionDefaultObjectSnapshotFile struct {
	Items []legacySessionDefaultObjectSnapshotItem `json:"items"`
}

type legacySessionDefaultObjectSnapshotItem struct {
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

func main() {
	sourceDBPath := flag.String("db", defaultSourceDBPath(), "source business sqlite db path")
	dataRoot := flag.String("data-root", defaultDataRoot(), "assessment data root")
	assessmentID := flag.Uint("assessment-id", 0, "only migrate one assessment id (0 = all)")
	apply := flag.Bool("apply", false, "apply migration (default: dry-run)")
	flag.Parse()

	sourceDB, err := gorm.Open(sqlite.Open(strings.TrimSpace(*sourceDBPath)), &gorm.Config{})
	if err != nil {
		log.Fatalf("open source business db failed: %v", err)
	}

	sessions, err := loadSessions(sourceDB, *assessmentID)
	if err != nil {
		log.Fatalf("load sessions failed: %v", err)
	}
	if len(sessions) == 0 {
		fmt.Println("no assessment sessions found")
		return
	}

	totalPlanned := tableCopyStats{}
	totalApplied := tableCopyStats{}
	for _, session := range sessions {
		targetDir := resolveSessionDataDir(session.DataDir, session.AssessmentName, strings.TrimSpace(*dataRoot))
		planned, err := planCopyStats(sourceDB, session.ID)
		if err != nil {
			log.Fatalf("plan failed for assessment_id=%d: %v", session.ID, err)
		}
		planned.DefaultSnapshots = estimateLegacySnapshotCount(targetDir, planned.Objects)
		totalPlanned.add(planned)

		fmt.Printf(
			"[plan] assessment_id=%d name=%s target=%s periods=%d groups=%d objects=%d module_scores=%d rule_files=%d default_snapshots=%d\n",
			session.ID,
			session.AssessmentName,
			filepath.Join(targetDir, sessionBusinessSQLiteFileName),
			planned.Periods,
			planned.ObjectGroups,
			planned.Objects,
			planned.ModuleScores,
			planned.RuleFiles,
			planned.DefaultSnapshots,
		)

		if !*apply {
			continue
		}

		applied, err := applySessionMigration(sourceDB, session, targetDir)
		if err != nil {
			log.Fatalf("apply failed for assessment_id=%d: %v", session.ID, err)
		}
		totalApplied.add(applied)

		fmt.Printf(
			"[done] assessment_id=%d periods=%d groups=%d objects=%d module_scores=%d rule_files=%d default_snapshots=%d\n",
			session.ID,
			applied.Periods,
			applied.ObjectGroups,
			applied.Objects,
			applied.ModuleScores,
			applied.RuleFiles,
			applied.DefaultSnapshots,
		)
	}

	if !*apply {
		fmt.Printf(
			"dry-run finished sessions=%d periods=%d groups=%d objects=%d module_scores=%d rule_files=%d default_snapshots=%d\n",
			len(sessions),
			totalPlanned.Periods,
			totalPlanned.ObjectGroups,
			totalPlanned.Objects,
			totalPlanned.ModuleScores,
			totalPlanned.RuleFiles,
			totalPlanned.DefaultSnapshots,
		)
		return
	}
	fmt.Printf(
		"migration finished sessions=%d periods=%d groups=%d objects=%d module_scores=%d rule_files=%d default_snapshots=%d\n",
		len(sessions),
		totalApplied.Periods,
		totalApplied.ObjectGroups,
		totalApplied.Objects,
		totalApplied.ModuleScores,
		totalApplied.RuleFiles,
		totalApplied.DefaultSnapshots,
	)
}

func loadSessions(sourceDB *gorm.DB, assessmentID uint) ([]sessionRow, error) {
	query := sourceDB.Table("assessment_sessions").
		Select("id, assessment_name, data_dir").
		Order("id ASC")
	if assessmentID > 0 {
		query = query.Where("id = ?", assessmentID)
	}
	items := make([]sessionRow, 0, 16)
	if err := query.Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func planCopyStats(sourceDB *gorm.DB, assessmentID uint) (tableCopyStats, error) {
	result := tableCopyStats{}

	periods, err := countRowsForAssessment(sourceDB, &model.AssessmentSessionPeriod{}, assessmentID)
	if err != nil {
		return result, err
	}
	groups, err := countRowsForAssessment(sourceDB, &model.AssessmentObjectGroup{}, assessmentID)
	if err != nil {
		return result, err
	}
	objects, err := countRowsForAssessment(sourceDB, &model.AssessmentSessionObject{}, assessmentID)
	if err != nil {
		return result, err
	}
	scores, err := countRowsForAssessment(sourceDB, &model.AssessmentObjectModuleScore{}, assessmentID)
	if err != nil {
		return result, err
	}
	rules, err := countRowsForAssessment(sourceDB, &model.RuleFile{}, assessmentID)
	if err != nil {
		return result, err
	}
	result.Periods = periods
	result.ObjectGroups = groups
	result.Objects = objects
	result.ModuleScores = scores
	result.RuleFiles = rules
	return result, nil
}

func countRowsForAssessment(sourceDB *gorm.DB, modelValue any, assessmentID uint) (int, error) {
	if !sourceDB.Migrator().HasTable(modelValue) {
		return 0, nil
	}
	var count int64
	if err := sourceDB.Model(modelValue).Where("assessment_id = ?", assessmentID).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func applySessionMigration(sourceDB *gorm.DB, session sessionRow, targetDir string) (tableCopyStats, error) {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return tableCopyStats{}, fmt.Errorf("create target dir: %w", err)
	}

	targetDBPath := filepath.Join(targetDir, sessionBusinessSQLiteFileName)
	targetDB, err := gorm.Open(sqlite.Open(targetDBPath), &gorm.Config{})
	if err != nil {
		return tableCopyStats{}, fmt.Errorf("open target session db: %w", err)
	}
	if err := targetDB.AutoMigrate(
		&model.AssessmentSessionPeriod{},
		&model.AssessmentObjectGroup{},
		&model.AssessmentSessionObject{},
		&model.AssessmentObjectModuleScore{},
		&model.RuleFile{},
		&model.SessionDefaultObjectSnapshot{},
	); err != nil {
		return tableCopyStats{}, fmt.Errorf("automigrate target session schema: %w", err)
	}

	stats := tableCopyStats{}
	err = targetDB.Transaction(func(tx *gorm.DB) error {
		periods, err := copyPeriods(sourceDB, tx, session.ID)
		if err != nil {
			return err
		}
		stats.Periods = periods

		groups, err := copyObjectGroups(sourceDB, tx, session.ID)
		if err != nil {
			return err
		}
		stats.ObjectGroups = groups

		objects, err := copyObjects(sourceDB, tx, session.ID)
		if err != nil {
			return err
		}
		stats.Objects = objects

		moduleScores, err := copyModuleScores(sourceDB, tx, session.ID)
		if err != nil {
			return err
		}
		stats.ModuleScores = moduleScores

		ruleFiles, err := copyRuleFiles(sourceDB, tx, session.ID)
		if err != nil {
			return err
		}
		stats.RuleFiles = ruleFiles

		snapshots, err := copyDefaultSnapshots(tx, session.ID, targetDir)
		if err != nil {
			return err
		}
		stats.DefaultSnapshots = snapshots
		return nil
	})
	if err != nil {
		return tableCopyStats{}, err
	}
	return stats, nil
}

func copyDefaultSnapshots(targetDB *gorm.DB, assessmentID uint, targetDir string) (int, error) {
	if err := targetDB.Where("assessment_id = ?", assessmentID).Delete(&model.SessionDefaultObjectSnapshot{}).Error; err != nil {
		return 0, fmt.Errorf("clear default snapshot rows: %w", err)
	}

	items, loaded, err := loadLegacyDefaultSnapshotItems(targetDir)
	if err != nil {
		return 0, err
	}
	if !loaded {
		objects := make([]model.AssessmentSessionObject, 0, 200)
		if err := targetDB.
			Where("assessment_id = ?", assessmentID).
			Order("sort_order ASC, id ASC").
			Find(&objects).Error; err != nil {
			return 0, fmt.Errorf("query target objects for default snapshot: %w", err)
		}
		items = buildDefaultSnapshotItemsFromObjects(objects)
	}
	if len(items) == 0 {
		return 0, nil
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
	if err := targetDB.CreateInBatches(rows, 200).Error; err != nil {
		return 0, fmt.Errorf("insert default snapshot rows: %w", err)
	}
	return len(rows), nil
}

func buildDefaultSnapshotItemsFromObjects(objects []model.AssessmentSessionObject) []legacySessionDefaultObjectSnapshotItem {
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

	items := make([]legacySessionDefaultObjectSnapshotItem, 0, len(objects))
	for _, item := range objects {
		row := legacySessionDefaultObjectSnapshotItem{
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

func loadLegacyDefaultSnapshotItems(targetDir string) ([]legacySessionDefaultObjectSnapshotItem, bool, error) {
	path := filepath.Join(targetDir, legacySessionDefaultObjectsFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read legacy default snapshot json: %w", err)
	}

	payload := legacySessionDefaultObjectSnapshotFile{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, false, fmt.Errorf("parse legacy default snapshot json: %w", err)
	}
	return payload.Items, true, nil
}

func estimateLegacySnapshotCount(targetDir string, fallback int) int {
	items, loaded, err := loadLegacyDefaultSnapshotItems(targetDir)
	if err != nil || !loaded {
		return fallback
	}
	return len(items)
}

func copyPeriods(sourceDB, targetDB *gorm.DB, assessmentID uint) (int, error) {
	if err := targetDB.Where("assessment_id = ?", assessmentID).Delete(&model.AssessmentSessionPeriod{}).Error; err != nil {
		return 0, fmt.Errorf("clear periods: %w", err)
	}
	if !sourceDB.Migrator().HasTable(&model.AssessmentSessionPeriod{}) {
		return 0, nil
	}
	rows := make([]model.AssessmentSessionPeriod, 0, 16)
	if err := sourceDB.Where("assessment_id = ?", assessmentID).Order("id ASC").Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("query periods: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	if err := targetDB.CreateInBatches(rows, 200).Error; err != nil {
		return 0, fmt.Errorf("insert periods: %w", err)
	}
	return len(rows), nil
}

func copyObjectGroups(sourceDB, targetDB *gorm.DB, assessmentID uint) (int, error) {
	if err := targetDB.Where("assessment_id = ?", assessmentID).Delete(&model.AssessmentObjectGroup{}).Error; err != nil {
		return 0, fmt.Errorf("clear object groups: %w", err)
	}
	if !sourceDB.Migrator().HasTable(&model.AssessmentObjectGroup{}) {
		return 0, nil
	}
	rows := make([]model.AssessmentObjectGroup, 0, 16)
	if err := sourceDB.Where("assessment_id = ?", assessmentID).Order("id ASC").Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("query object groups: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	if err := targetDB.CreateInBatches(rows, 200).Error; err != nil {
		return 0, fmt.Errorf("insert object groups: %w", err)
	}
	return len(rows), nil
}

func copyObjects(sourceDB, targetDB *gorm.DB, assessmentID uint) (int, error) {
	if err := targetDB.Where("assessment_id = ?", assessmentID).Delete(&model.AssessmentSessionObject{}).Error; err != nil {
		return 0, fmt.Errorf("clear objects: %w", err)
	}
	if !sourceDB.Migrator().HasTable(&model.AssessmentSessionObject{}) {
		return 0, nil
	}
	rows := make([]model.AssessmentSessionObject, 0, 200)
	if err := sourceDB.Where("assessment_id = ?", assessmentID).Order("id ASC").Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("query objects: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	if err := targetDB.CreateInBatches(rows, 200).Error; err != nil {
		return 0, fmt.Errorf("insert objects: %w", err)
	}
	return len(rows), nil
}

func copyModuleScores(sourceDB, targetDB *gorm.DB, assessmentID uint) (int, error) {
	if err := targetDB.Where("assessment_id = ?", assessmentID).Delete(&model.AssessmentObjectModuleScore{}).Error; err != nil {
		return 0, fmt.Errorf("clear module scores: %w", err)
	}
	if !sourceDB.Migrator().HasTable(&model.AssessmentObjectModuleScore{}) {
		return 0, nil
	}
	rows := make([]model.AssessmentObjectModuleScore, 0, 500)
	if err := sourceDB.Where("assessment_id = ?", assessmentID).Order("id ASC").Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("query module scores: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	if err := targetDB.CreateInBatches(rows, 200).Error; err != nil {
		return 0, fmt.Errorf("insert module scores: %w", err)
	}
	return len(rows), nil
}

func copyRuleFiles(sourceDB, targetDB *gorm.DB, assessmentID uint) (int, error) {
	if err := targetDB.Where("assessment_id = ?", assessmentID).Delete(&model.RuleFile{}).Error; err != nil {
		return 0, fmt.Errorf("clear rule files: %w", err)
	}
	if !sourceDB.Migrator().HasTable(&model.RuleFile{}) {
		return 0, nil
	}
	rows := make([]model.RuleFile, 0, 16)
	if err := sourceDB.Where("assessment_id = ?", assessmentID).Order("id ASC").Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("query rule files: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	if err := targetDB.CreateInBatches(rows, 200).Error; err != nil {
		return 0, fmt.Errorf("insert rule files: %w", err)
	}
	return len(rows), nil
}

func defaultSourceDBPath() string {
	if value := strings.TrimSpace(os.Getenv("ASSESS_SQLITE_PATH")); value != "" {
		return value
	}
	return filepath.Join("data", "assess.db")
}

func defaultDataRoot() string {
	if value := strings.TrimSpace(os.Getenv("ASSESS_DATA_ROOT")); value != "" {
		return value
	}
	return "data"
}

func resolveSessionDataDir(dataDir string, assessmentName string, dataRoot string) string {
	text := strings.TrimSpace(dataDir)
	if text == "" {
		root := strings.TrimSpace(dataRoot)
		if root == "" {
			root = "data"
		}
		return filepath.Clean(filepath.Join(root, assessmentName))
	}
	if filepath.IsAbs(text) {
		return filepath.Clean(text)
	}

	root := strings.TrimSpace(dataRoot)
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

func (t *tableCopyStats) add(other tableCopyStats) {
	t.Periods += other.Periods
	t.ObjectGroups += other.ObjectGroups
	t.Objects += other.Objects
	t.ModuleScores += other.ModuleScores
	t.RuleFiles += other.RuleFiles
	t.DefaultSnapshots += other.DefaultSnapshots
}
