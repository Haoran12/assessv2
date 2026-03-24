package main

import (
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

type sessionRow struct {
	ID             uint
	AssessmentName string
	DataDir        string
}

type tableCopyStats struct {
	Periods      int
	ObjectGroups int
	Objects      int
	ModuleScores int
	RuleFiles    int
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
		totalPlanned.add(planned)

		fmt.Printf(
			"[plan] assessment_id=%d name=%s target=%s periods=%d groups=%d objects=%d module_scores=%d rule_files=%d\n",
			session.ID,
			session.AssessmentName,
			filepath.Join(targetDir, sessionBusinessSQLiteFileName),
			planned.Periods,
			planned.ObjectGroups,
			planned.Objects,
			planned.ModuleScores,
			planned.RuleFiles,
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
			"[done] assessment_id=%d periods=%d groups=%d objects=%d module_scores=%d rule_files=%d\n",
			session.ID,
			applied.Periods,
			applied.ObjectGroups,
			applied.Objects,
			applied.ModuleScores,
			applied.RuleFiles,
		)
	}

	if !*apply {
		fmt.Printf(
			"dry-run finished sessions=%d periods=%d groups=%d objects=%d module_scores=%d rule_files=%d\n",
			len(sessions),
			totalPlanned.Periods,
			totalPlanned.ObjectGroups,
			totalPlanned.Objects,
			totalPlanned.ModuleScores,
			totalPlanned.RuleFiles,
		)
		return
	}
	fmt.Printf(
		"migration finished sessions=%d periods=%d groups=%d objects=%d module_scores=%d rule_files=%d\n",
		len(sessions),
		totalApplied.Periods,
		totalApplied.ObjectGroups,
		totalApplied.Objects,
		totalApplied.ModuleScores,
		totalApplied.RuleFiles,
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
		return nil
	})
	if err != nil {
		return tableCopyStats{}, err
	}
	return stats, nil
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
}
