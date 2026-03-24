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

type ruleFileRow struct {
	ID             uint
	AssessmentName string
	DataDir        string
	FilePath       string
}

func main() {
	dbPath := flag.String("db", defaultDBPath(), "business sqlite db path")
	dataRoot := flag.String("data-root", defaultDataRoot(), "assessment data root")
	apply := flag.Bool("apply", false, "apply changes (default: dry-run)")
	flag.Parse()

	db, err := gorm.Open(sqlite.Open(strings.TrimSpace(*dbPath)), &gorm.Config{})
	if err != nil {
		log.Fatalf("open sqlite failed: %v", err)
	}

	rows := make([]ruleFileRow, 0, 32)
	if err := db.Table("rule_files AS r").
		Select("r.id, s.assessment_name, s.data_dir, r.file_path").
		Joins("JOIN assessment_sessions s ON s.id = r.assessment_id").
		Order("r.id ASC").
		Scan(&rows).Error; err != nil {
		log.Fatalf("query rule files failed: %v", err)
	}

	planCount := 0
	appliedCount := 0
	for _, row := range rows {
		targetPath := buildTargetPath(row, strings.TrimSpace(*dataRoot))
		if targetPath == "" {
			continue
		}

		currentPath := resolvePath(strings.TrimSpace(row.FilePath), strings.TrimSpace(*dataRoot))
		if currentPath != "" && samePath(currentPath, targetPath) {
			continue
		}

		planCount++
		fmt.Printf("[plan] rule_id=%d from=%s to=%s\n", row.ID, currentPath, targetPath)
		if !*apply {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			log.Fatalf("create target dir failed for rule_id=%d: %v", row.ID, err)
		}

		finalTarget := targetPath
		if currentPath != "" && fileExists(currentPath) && !samePath(currentPath, finalTarget) {
			if fileExists(finalTarget) {
				finalTarget = withMigrationSuffix(finalTarget, row.ID)
			}
			if err := moveFileWithFallback(currentPath, finalTarget); err != nil {
				log.Fatalf("move file failed for rule_id=%d: %v", row.ID, err)
			}
		}

		if err := db.Table("rule_files").Where("id = ?", row.ID).Update("file_path", finalTarget).Error; err != nil {
			log.Fatalf("update file_path failed for rule_id=%d: %v", row.ID, err)
		}
		appliedCount++
		fmt.Printf("[done] rule_id=%d target=%s\n", row.ID, finalTarget)
	}

	if *apply {
		fmt.Printf("migration finished, planned=%d applied=%d\n", planCount, appliedCount)
	} else {
		fmt.Printf("dry-run finished, planned=%d (use --apply to execute)\n", planCount)
	}
}

func buildTargetPath(row ruleFileRow, dataRoot string) string {
	fileName := strings.TrimSpace(filepath.Base(strings.TrimSpace(row.FilePath)))
	if fileName == "" || fileName == "." {
		fileName = fmt.Sprintf("rule_%d.json", row.ID)
	}

	sessionDir := strings.TrimSpace(row.DataDir)
	if sessionDir != "" {
		sessionDir = resolvePath(sessionDir, dataRoot)
		return filepath.Join(sessionDir, fileName)
	}

	if dataRoot == "" {
		return ""
	}
	return filepath.Join(dataRoot, strings.TrimSpace(row.AssessmentName), fileName)
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

func withMigrationSuffix(path string, ruleID uint) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)
	dir := filepath.Dir(path)
	suffix := time.Now().UnixNano()
	return filepath.Join(dir, fmt.Sprintf("%s_migrated_%d_%d%s", base, ruleID, suffix, ext))
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
