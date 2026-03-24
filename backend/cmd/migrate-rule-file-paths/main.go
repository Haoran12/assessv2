package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const (
	sessionBusinessSQLiteFileName = "assess.db"
	canonicalRuleFileName         = "rule.json"
)

type sessionRow struct {
	ID             uint
	AssessmentName string
	DataDir        string
}

type ruleFileRow struct {
	ID           uint
	AssessmentID uint
	RuleName     string
	FilePath     string
	ContentJSON  string
	IsCopy       bool
	SourceRuleID *uint
	OwnerOrgID   *uint
	UpdatedAt    int64
}

func main() {
	dbPath := flag.String("db", defaultDBPath(), "main metadata sqlite db path (assessment_sessions)")
	dataRoot := flag.String("data-root", defaultDataRoot(), "assessment data root")
	assessmentID := flag.Uint("assessment-id", 0, "only process one assessment session (0 = all)")
	apply := flag.Bool("apply", false, "apply changes (default: dry-run)")
	flag.Parse()

	mainDB, err := gorm.Open(sqlite.Open(strings.TrimSpace(*dbPath)), &gorm.Config{})
	if err != nil {
		log.Fatalf("open main sqlite failed: %v", err)
	}

	sessions, err := loadSessions(mainDB, *assessmentID)
	if err != nil {
		log.Fatalf("query assessment sessions failed: %v", err)
	}
	if len(sessions) == 0 {
		fmt.Println("no assessment sessions found")
		return
	}

	planned := 0
	applied := 0
	for _, session := range sessions {
		sessionDir := resolveSessionDataDir(session.DataDir, session.AssessmentName, strings.TrimSpace(*dataRoot))
		sessionDBPath := filepath.Join(sessionDir, sessionBusinessSQLiteFileName)
		if !fileExists(sessionDBPath) {
			fmt.Printf("[skip] assessment_id=%d session_db_missing=%s\n", session.ID, sessionDBPath)
			continue
		}

		sessionDB, closeFn, err := openSQLite(sessionDBPath)
		if err != nil {
			log.Fatalf("open session db failed assessment_id=%d: %v", session.ID, err)
		}

		if !sessionDB.Migrator().HasTable("rule_files") {
			closeFn()
			fmt.Printf("[skip] assessment_id=%d no rule_files table in %s\n", session.ID, sessionDBPath)
			continue
		}

		rows, err := loadRuleFiles(sessionDB, session.ID)
		if err != nil {
			closeFn()
			log.Fatalf("query rule_files failed assessment_id=%d: %v", session.ID, err)
		}
		if len(rows) == 0 {
			closeFn()
			continue
		}

		keeper := pickKeeper(rows)
		targetPath := filepath.Join(sessionDir, canonicalRuleFileName)
		currentKeeperPath := resolvePath(strings.TrimSpace(keeper.FilePath), strings.TrimSpace(*dataRoot))

		if currentKeeperPath == "" || !samePath(currentKeeperPath, targetPath) ||
			keeper.IsCopy || keeper.SourceRuleID != nil || keeper.OwnerOrgID != nil || len(rows) > 1 {
			planned++
			fmt.Printf(
				"[plan] assessment_id=%d keep_rule_id=%d target=%s remove_legacy_rows=%d\n",
				session.ID,
				keeper.ID,
				targetPath,
				len(rows)-1,
			)
		}

		if !*apply {
			closeFn()
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			closeFn()
			log.Fatalf("create target dir failed assessment_id=%d: %v", session.ID, err)
		}

		if err := sessionDB.Transaction(func(tx *gorm.DB) error {
			// Remove duplicate rows and their files first.
			for _, row := range rows {
				if row.ID == keeper.ID {
					continue
				}
				oldPath := resolvePath(strings.TrimSpace(row.FilePath), strings.TrimSpace(*dataRoot))
				if oldPath != "" && !samePath(oldPath, targetPath) {
					if err := removeFileIfExists(oldPath); err != nil {
						return fmt.Errorf("remove legacy rule file failed: %w", err)
					}
				}
				if err := tx.Table("rule_files").Where("id = ?", row.ID).Delete(nil).Error; err != nil {
					return fmt.Errorf("delete legacy rule row failed: %w", err)
				}
			}

			// Prepare canonical target file.
			if currentKeeperPath != "" && fileExists(currentKeeperPath) && !samePath(currentKeeperPath, targetPath) {
				if fileExists(targetPath) {
					if err := removeFileIfExists(targetPath); err != nil {
						return fmt.Errorf("remove existing canonical rule file failed: %w", err)
					}
				}
				if err := moveFileWithFallback(currentKeeperPath, targetPath); err != nil {
					return fmt.Errorf("move keeper rule file failed: %w", err)
				}
			}
			if !fileExists(targetPath) {
				if err := os.WriteFile(targetPath, []byte(keeper.ContentJSON), 0o644); err != nil {
					return fmt.Errorf("write canonical rule file failed: %w", err)
				}
			}

			updates := map[string]any{
				"file_path":      targetPath,
				"is_copy":        false,
				"source_rule_id": nil,
				"owner_org_id":   nil,
				"updated_at":     time.Now().Unix(),
			}
			if err := tx.Table("rule_files").Where("id = ?", keeper.ID).Updates(updates).Error; err != nil {
				return fmt.Errorf("normalize keeper rule row failed: %w", err)
			}
			return nil
		}); err != nil {
			closeFn()
			log.Fatalf("apply cleanup failed assessment_id=%d: %v", session.ID, err)
		}

		applied++
		fmt.Printf(
			"[done] assessment_id=%d keep_rule_id=%d canonical=%s removed=%d\n",
			session.ID,
			keeper.ID,
			targetPath,
			len(rows)-1,
		)
		closeFn()
	}

	if *apply {
		fmt.Printf("cleanup finished, planned=%d applied=%d\n", planned, applied)
	} else {
		fmt.Printf("dry-run finished, planned=%d (use --apply to execute)\n", planned)
	}
}

func loadSessions(mainDB *gorm.DB, assessmentID uint) ([]sessionRow, error) {
	items := make([]sessionRow, 0, 16)
	query := mainDB.Table("assessment_sessions").Select("id, assessment_name, data_dir").Order("id ASC")
	if assessmentID > 0 {
		query = query.Where("id = ?", assessmentID)
	}
	if err := query.Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func loadRuleFiles(sessionDB *gorm.DB, assessmentID uint) ([]ruleFileRow, error) {
	items := make([]ruleFileRow, 0, 8)
	if err := sessionDB.
		Table("rule_files").
		Select("id, assessment_id, rule_name, file_path, content_json, is_copy, source_rule_id, owner_org_id, updated_at").
		Where("assessment_id = ?", assessmentID).
		Order("updated_at DESC, id DESC").
		Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func pickKeeper(rows []ruleFileRow) ruleFileRow {
	for _, row := range rows {
		if !row.IsCopy {
			return row
		}
	}
	return rows[0]
}

func openSQLite(path string) (*gorm.DB, func(), error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	return db, func() { _ = sqlDB.Close() }, nil
}

func defaultDBPath() string {
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
		relative = strings.TrimPrefix(relative, "/")
		return filepath.Clean(filepath.Join(root, filepath.FromSlash(relative)))
	}
	return filepath.Clean(filepath.Join(root, filepath.FromSlash(normalized)))
}

func resolvePath(path string, dataRoot string) string {
	text := strings.TrimSpace(path)
	if text == "" {
		return ""
	}
	if filepath.IsAbs(text) {
		return filepath.Clean(text)
	}

	normalized := strings.ReplaceAll(text, "\\", "/")
	if dataRoot != "" && strings.HasPrefix(strings.ToLower(normalized), "data/") {
		relative := strings.TrimPrefix(normalized, "data/")
		relative = strings.TrimPrefix(relative, "/")
		return filepath.Clean(filepath.Join(dataRoot, filepath.FromSlash(relative)))
	}

	if dataRoot != "" {
		return filepath.Clean(filepath.Join(dataRoot, filepath.FromSlash(normalized)))
	}
	return filepath.Clean(filepath.FromSlash(normalized))
}

func samePath(left string, right string) bool {
	return strings.EqualFold(filepath.Clean(strings.TrimSpace(left)), filepath.Clean(strings.TrimSpace(right)))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func removeFileIfExists(path string) error {
	if path == "" || !fileExists(path) {
		return nil
	}
	return os.Remove(path)
}

func moveFileWithFallback(src string, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer target.Close()

	if _, err := target.ReadFrom(source); err != nil {
		return err
	}
	if err := target.Sync(); err != nil {
		return err
	}
	return os.Remove(src)
}
