package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"assessv2/backend/internal/config"
	"assessv2/backend/internal/database"
	"gorm.io/gorm"
)

const (
	periodTemplatesSettingKey     = "assessment.period_templates"
	defaultPeriodRangeSettingKey  = "assessment.default_period_range"
	backupDirName                 = "migration-backups"
	backupTimestampLayout         = "20060102-150405"
	legacyFlatAssessmentDBName    = "assess.db"
	yearlyAssessmentDBRelativeFmt = "%s/assess.db"
)

var yearDirPattern = regexp.MustCompile(`^\d{4}$`)

type summary struct {
	Path     string
	Actions  []string
	Warnings []string
}

type templateItem struct {
	PeriodCode string `json:"periodCode"`
	PeriodName string `json:"periodName"`
	SortOrder  int    `json:"sortOrder"`
}

type templateEnvelope struct {
	Items []templateItem `json:"items"`
}

func main() {
	dataRootFlag := flag.String("data-root", filepath.Join("..", "data"), "data root directory")
	dryRunFlag := flag.Bool("dry-run", false, "preview changes without writing")
	flag.Parse()

	dataRoot, err := filepath.Abs(strings.TrimSpace(*dataRootFlag))
	if err != nil {
		log.Fatalf("resolve data root failed: %v", err)
	}
	if info, statErr := os.Stat(dataRoot); statErr != nil || !info.IsDir() {
		log.Fatalf("invalid data root: %s", dataRoot)
	}

	dbPaths, err := discoverAssessmentDatabases(dataRoot)
	if err != nil {
		log.Fatalf("discover assessment dbs failed: %v", err)
	}
	if len(dbPaths) == 0 {
		fmt.Printf("no assessment database found under %s\n", dataRoot)
		return
	}

	backupRoot := ""
	if !*dryRunFlag {
		backupRoot = filepath.Join(dataRoot, backupDirName, time.Now().Format(backupTimestampLayout))
		if err := os.MkdirAll(backupRoot, 0o755); err != nil {
			log.Fatalf("create backup dir failed: %v", err)
		}
	}

	fmt.Printf("data root: %s\n", dataRoot)
	fmt.Printf("mode: %s\n", ternary(*dryRunFlag, "dry-run", "apply"))
	if backupRoot != "" {
		fmt.Printf("backup root: %s\n", backupRoot)
	}
	fmt.Printf("target db count: %d\n", len(dbPaths))

	results := make([]summary, 0, len(dbPaths))
	var changedCount int
	for _, dbPath := range dbPaths {
		result, err := migrateOneDB(dbPath, dataRoot, backupRoot, *dryRunFlag)
		if err != nil {
			log.Fatalf("migrate %s failed: %v", dbPath, err)
		}
		if len(result.Actions) > 0 {
			changedCount++
		}
		results = append(results, result)
	}

	fmt.Println("migration summary:")
	for _, item := range results {
		fmt.Printf("- %s\n", item.Path)
		if len(item.Actions) == 0 {
			fmt.Println("  actions: none")
		} else {
			for _, action := range item.Actions {
				fmt.Printf("  action: %s\n", action)
			}
		}
		for _, warning := range item.Warnings {
			fmt.Printf("  warning: %s\n", warning)
		}
	}
	fmt.Printf("completed. changed=%d/%d\n", changedCount, len(results))
}

func discoverAssessmentDatabases(dataRoot string) ([]string, error) {
	items := make([]string, 0, 8)

	legacyFlat := filepath.Join(dataRoot, legacyFlatAssessmentDBName)
	if fileExists(legacyFlat) {
		items = append(items, legacyFlat)
	}

	entries, err := os.ReadDir(dataRoot)
	if err != nil {
		return nil, fmt.Errorf("read data root failed: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if !yearDirPattern.MatchString(name) {
			continue
		}
		candidate := filepath.Join(dataRoot, fmt.Sprintf(yearlyAssessmentDBRelativeFmt, name))
		if fileExists(candidate) {
			items = append(items, candidate)
		}
	}

	sort.Strings(items)
	return items, nil
}

func migrateOneDB(dbPath, dataRoot, backupRoot string, dryRun bool) (summary, error) {
	result := summary{Path: dbPath}
	db, sqlDB, err := openSQLite(dbPath)
	if err != nil {
		return result, err
	}
	defer func() { _ = sqlDB.Close() }()

	if _, err := sqlDB.Exec("PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("wal checkpoint failed: %v", err))
	}

	if !dryRun {
		if err := backupSQLiteBundle(dataRoot, backupRoot, dbPath); err != nil {
			return result, err
		}
		result.Actions = append(result.Actions, "backup sqlite bundle")
	}

	if err := db.Exec("PRAGMA foreign_keys = OFF;").Error; err != nil {
		return result, fmt.Errorf("disable foreign keys failed: %w", err)
	}
	defer func() {
		_ = db.Exec("PRAGMA foreign_keys = ON;").Error
	}()

	if err := db.Transaction(func(tx *gorm.DB) error {
		dropped, err := dropColumnIfExists(tx, dryRun, "assessment_years", "start_date")
		if err != nil {
			return err
		}
		if dropped {
			result.Actions = append(result.Actions, "drop assessment_years.start_date")
		}

		dropped, err = dropColumnIfExists(tx, dryRun, "assessment_years", "end_date")
		if err != nil {
			return err
		}
		if dropped {
			result.Actions = append(result.Actions, "drop assessment_years.end_date")
		}

		dropped, err = dropColumnIfExists(tx, dryRun, "assessment_periods", "start_date")
		if err != nil {
			return err
		}
		if dropped {
			result.Actions = append(result.Actions, "drop assessment_periods.start_date")
		}

		dropped, err = dropColumnIfExists(tx, dryRun, "assessment_periods", "end_date")
		if err != nil {
			return err
		}
		if dropped {
			result.Actions = append(result.Actions, "drop assessment_periods.end_date")
		}

		removed, err := deleteSettingIfExists(tx, dryRun, defaultPeriodRangeSettingKey)
		if err != nil {
			return err
		}
		if removed {
			result.Actions = append(result.Actions, "remove system setting assessment.default_period_range")
		}

		rewriteDone, rewriteWarning, err := rewritePeriodTemplatesSetting(tx, dryRun)
		if err != nil {
			return err
		}
		if rewriteDone {
			result.Actions = append(result.Actions, "rewrite assessment.period_templates to code/name/sortOrder")
		}
		if rewriteWarning != "" {
			result.Warnings = append(result.Warnings, rewriteWarning)
		}

		return nil
	}); err != nil {
		return result, err
	}

	return result, nil
}

func openSQLite(path string) (*gorm.DB, *sql.DB, error) {
	cfg := config.DatabaseConfig{
		Path:          path,
		ForeignKeys:   false,
		JournalMode:   "WAL",
		Synchronous:   "NORMAL",
		BusyTimeoutMS: 5000,
		MaxOpenConns:  1,
		MaxIdleConns:  1,
	}
	db, err := database.NewSQLite(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("open sqlite %s failed: %w", path, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, fmt.Errorf("open sql handle failed: %w", err)
	}
	return db, sqlDB, nil
}

func dropColumnIfExists(tx *gorm.DB, dryRun bool, tableName, columnName string) (bool, error) {
	exists, err := columnExists(tx, tableName, columnName)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	if dryRun {
		return true, nil
	}
	query := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", tableName, columnName)
	if err := tx.Exec(query).Error; err != nil {
		return false, fmt.Errorf("drop column %s.%s failed: %w", tableName, columnName, err)
	}
	return true, nil
}

func deleteSettingIfExists(tx *gorm.DB, dryRun bool, key string) (bool, error) {
	tableExists, err := hasTable(tx, "system_settings")
	if err != nil {
		return false, err
	}
	if !tableExists {
		return false, nil
	}

	var count int64
	if err := tx.Table("system_settings").Where("setting_key = ?", key).Count(&count).Error; err != nil {
		return false, fmt.Errorf("query setting %s failed: %w", key, err)
	}
	if count == 0 {
		return false, nil
	}
	if dryRun {
		return true, nil
	}
	if err := tx.Exec("DELETE FROM system_settings WHERE setting_key = ?", key).Error; err != nil {
		return false, fmt.Errorf("delete setting %s failed: %w", key, err)
	}
	return true, nil
}

func rewritePeriodTemplatesSetting(tx *gorm.DB, dryRun bool) (bool, string, error) {
	tableExists, err := hasTable(tx, "system_settings")
	if err != nil {
		return false, "", err
	}
	if !tableExists {
		return false, "", nil
	}

	var row struct {
		SettingValue string
	}
	result := tx.Table("system_settings").
		Select("setting_value").
		Where("setting_key = ?", periodTemplatesSettingKey).
		Limit(1).
		Scan(&row)
	if result.Error != nil {
		return false, "", fmt.Errorf("query %s failed: %w", periodTemplatesSettingKey, result.Error)
	}
	if result.RowsAffected == 0 {
		return false, "", nil
	}

	items, warning, err := parseTemplateItems(row.SettingValue)
	if err != nil {
		return false, warning, nil
	}
	normalized := normalizeTemplateItems(items)
	if len(normalized) == 0 {
		return false, "assessment.period_templates has no valid items; keep original value", nil
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return false, "", fmt.Errorf("marshal normalized templates failed: %w", err)
	}
	if strings.TrimSpace(string(raw)) == strings.TrimSpace(row.SettingValue) {
		return false, warning, nil
	}
	if dryRun {
		return true, warning, nil
	}
	if err := tx.Exec(
		"UPDATE system_settings SET setting_value = ?, setting_type = 'json', updated_at = ? WHERE setting_key = ?",
		string(raw),
		time.Now().Unix(),
		periodTemplatesSettingKey,
	).Error; err != nil {
		return false, "", fmt.Errorf("update %s failed: %w", periodTemplatesSettingKey, err)
	}
	return true, warning, nil
}

func parseTemplateItems(raw string) ([]templateItem, string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, "", fmt.Errorf("empty template")
	}

	var direct []templateItem
	if err := json.Unmarshal([]byte(text), &direct); err == nil {
		return direct, "", nil
	}

	var envelope templateEnvelope
	if err := json.Unmarshal([]byte(text), &envelope); err == nil {
		return envelope.Items, "", nil
	}

	return nil, "assessment.period_templates is not valid JSON array/object; keep original value", fmt.Errorf("invalid json")
}

func normalizeTemplateItems(items []templateItem) []templateItem {
	output := make([]templateItem, 0, len(items))
	seen := make(map[string]struct{}, len(items))

	for idx, item := range items {
		code := strings.ToUpper(strings.TrimSpace(item.PeriodCode))
		name := strings.TrimSpace(item.PeriodName)
		if code == "" || name == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}

		sortOrder := item.SortOrder
		if sortOrder <= 0 {
			sortOrder = idx + 1
		}
		output = append(output, templateItem{
			PeriodCode: code,
			PeriodName: name,
			SortOrder:  sortOrder,
		})
	}

	sort.SliceStable(output, func(i, j int) bool {
		if output[i].SortOrder != output[j].SortOrder {
			return output[i].SortOrder < output[j].SortOrder
		}
		return output[i].PeriodCode < output[j].PeriodCode
	})
	for idx := range output {
		output[idx].SortOrder = idx + 1
	}
	return output
}

func columnExists(tx *gorm.DB, tableName, columnName string) (bool, error) {
	rows, err := tx.Raw(fmt.Sprintf("PRAGMA table_info(%s)", tableName)).Rows()
	if err != nil {
		return false, fmt.Errorf("query table_info %s failed: %w", tableName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return false, fmt.Errorf("scan table_info %s failed: %w", tableName, err)
		}
		if strings.EqualFold(strings.TrimSpace(name), columnName) {
			return true, nil
		}
	}
	return false, nil
}

func hasTable(tx *gorm.DB, tableName string) (bool, error) {
	var count int64
	if err := tx.Raw(
		"SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = ?",
		tableName,
	).Scan(&count).Error; err != nil {
		return false, fmt.Errorf("query sqlite_master for %s failed: %w", tableName, err)
	}
	return count > 0, nil
}

func backupSQLiteBundle(dataRoot, backupRoot, dbPath string) error {
	relative, err := filepath.Rel(dataRoot, dbPath)
	if err != nil {
		return fmt.Errorf("resolve backup relative path failed: %w", err)
	}
	targetMain := filepath.Join(backupRoot, relative)
	if err := os.MkdirAll(filepath.Dir(targetMain), 0o755); err != nil {
		return fmt.Errorf("create backup directory failed: %w", err)
	}

	if err := copyIfExists(dbPath, targetMain); err != nil {
		return err
	}
	if err := copyIfExists(dbPath+"-wal", targetMain+"-wal"); err != nil {
		return err
	}
	if err := copyIfExists(dbPath+"-shm", targetMain+"-shm"); err != nil {
		return err
	}
	return nil
}

func copyIfExists(src, dst string) error {
	if !fileExists(src) {
		return nil
	}
	input, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s failed: %w", src, err)
	}
	if err := os.WriteFile(dst, input, 0o644); err != nil {
		return fmt.Errorf("write %s failed: %w", dst, err)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func ternary(condition bool, whenTrue, whenFalse string) string {
	if condition {
		return whenTrue
	}
	return whenFalse
}
