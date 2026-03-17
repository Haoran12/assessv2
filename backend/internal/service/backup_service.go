package service

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

const (
	BackupTypeManual        = "manual"
	BackupTypeAuto          = "auto"
	BackupTypeBeforeImport  = "before_import"
	BackupTypeBeforeRestore = "before_restore"

	restoreConfirmText = "CONFIRM_RESTORE"
)

var validBackupTypes = map[string]struct{}{
	BackupTypeManual:        {},
	BackupTypeAuto:          {},
	BackupTypeBeforeImport:  {},
	BackupTypeBeforeRestore: {},
}

type BackupService struct {
	db        *gorm.DB
	auditRepo *repository.AuditRepository
	dbPath    string
	backupDir string

	autoBackupOnce sync.Once
	autoBackupMu   sync.Mutex
	lastAutoDay    string
}

type BackupListInput struct {
	Page     int
	PageSize int
	Type     string
}

type BackupRecordDTO struct {
	ID          uint   `json:"id"`
	BackupName  string `json:"backupName"`
	BackupType  string `json:"backupType"`
	FileSize    int64  `json:"fileSize"`
	Description string `json:"description"`
	CreatedBy   *uint  `json:"createdBy,omitempty"`
	CreatedAt   int64  `json:"createdAt"`
}

type BackupListResult struct {
	Items    []BackupRecordDTO `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}

func NewBackupService(db *gorm.DB, auditRepo *repository.AuditRepository, sqlitePath string) *BackupService {
	backupRoot := filepath.Join(filepath.Dir(strings.TrimSpace(sqlitePath)), "backups")
	if strings.TrimSpace(sqlitePath) == "" {
		backupRoot = filepath.Join(".", "data", "backups")
	}
	return &BackupService{
		db:        db,
		auditRepo: auditRepo,
		dbPath:    sqlitePath,
		backupDir: backupRoot,
	}
}

func (s *BackupService) StartAutoBackup(ctx context.Context) {
	s.autoBackupOnce.Do(func() {
		go s.autoBackupLoop(ctx)
	})
}

func (s *BackupService) List(ctx context.Context, input BackupListInput) (*BackupListResult, error) {
	page := input.Page
	if page <= 0 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}

	query := s.db.WithContext(ctx).Model(&model.BackupRecord{})
	backupType := strings.TrimSpace(input.Type)
	if backupType != "" {
		if !isValidBackupType(backupType) {
			return nil, ErrInvalidBackupType
		}
		query = query.Where("backup_type = ?", backupType)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count backup records: %w", err)
	}

	var records []model.BackupRecord
	if err := query.
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("failed to query backup records: %w", err)
	}

	items := make([]BackupRecordDTO, 0, len(records))
	for _, item := range records {
		items = append(items, backupRecordToDTO(item))
	}

	return &BackupListResult{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *BackupService) CreateManual(
	ctx context.Context,
	operatorID uint,
	description string,
	ipAddress string,
	userAgent string,
) (*BackupRecordDTO, error) {
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID)
	record, err := s.createBackup(ctx, BackupTypeManual, operatorRef, description, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}
	dto := backupRecordToDTO(*record)
	return &dto, nil
}

func (s *BackupService) Delete(
	ctx context.Context,
	operatorID uint,
	backupID uint,
	ipAddress string,
	userAgent string,
) error {
	var record model.BackupRecord
	if err := s.db.WithContext(ctx).Where("id = ?", backupID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrBackupNotFound
		}
		return fmt.Errorf("failed to query backup record: %w", err)
	}

	before := map[string]any{
		"id":          record.ID,
		"backup_name": record.BackupName,
		"backup_path": record.BackupPath,
		"backup_type": record.BackupType,
		"file_size":   record.FileSize,
		"description": record.Description,
		"created_by":  record.CreatedBy,
		"created_at":  record.CreatedAt,
	}

	if removeErr := os.Remove(record.BackupPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return fmt.Errorf("failed to delete backup file: %w", removeErr)
	}

	if err := s.db.WithContext(ctx).Delete(&model.BackupRecord{}, record.ID).Error; err != nil {
		return fmt.Errorf("failed to delete backup record: %w", err)
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		&operatorID,
		"delete",
		"backups",
		&targetID,
		map[string]any{
			"event":  "delete_backup",
			"before": before,
		},
		ipAddress,
		userAgent,
	))

	return nil
}

func (s *BackupService) ResolveDownloadPath(ctx context.Context, backupID uint) (string, string, error) {
	var record model.BackupRecord
	if err := s.db.WithContext(ctx).Where("id = ?", backupID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", ErrBackupNotFound
		}
		return "", "", fmt.Errorf("failed to query backup record: %w", err)
	}
	if _, err := os.Stat(record.BackupPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", ErrBackupNotFound
		}
		return "", "", err
	}
	return record.BackupPath, record.BackupName, nil
}

func (s *BackupService) Restore(
	ctx context.Context,
	operatorID uint,
	backupID uint,
	confirmText string,
	ipAddress string,
	userAgent string,
) error {
	if strings.TrimSpace(confirmText) != restoreConfirmText {
		return ErrBackupConfirmMismatch
	}

	var record model.BackupRecord
	if err := s.db.WithContext(ctx).Where("id = ?", backupID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrBackupNotFound
		}
		return fmt.Errorf("failed to query backup record: %w", err)
	}

	beforeRestore, err := s.createBackup(
		ctx,
		BackupTypeBeforeRestore,
		resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID),
		fmt.Sprintf("before restore backup id=%d", backupID),
		ipAddress,
		userAgent,
	)
	if err != nil {
		return fmt.Errorf("failed to create pre-restore backup: %w", err)
	}

	restoreSourcePath := filepath.Join(os.TempDir(), fmt.Sprintf("assessv2_restore_%d_%d.db", backupID, time.Now().UnixNano()))
	defer func() {
		_ = os.Remove(restoreSourcePath)
	}()
	if err := decompressGzipFile(record.BackupPath, restoreSourcePath); err != nil {
		return fmt.Errorf("failed to decompress backup file: %w", err)
	}

	if err := s.restoreFromSQLite(ctx, restoreSourcePath); err != nil {
		return err
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		&operatorID,
		"restore",
		"backups",
		&targetID,
		map[string]any{
			"event":                  "restore_backup",
			"backup_id":              record.ID,
			"backup_name":            record.BackupName,
			"before_restore_backup":  beforeRestore.ID,
			"restore_confirmation":   restoreConfirmText,
			"restore_source_path":    record.BackupPath,
			"restore_source_created": record.CreatedAt,
		},
		ipAddress,
		userAgent,
	))

	return nil
}

func (s *BackupService) createBackup(
	ctx context.Context,
	backupType string,
	createdBy *uint,
	description string,
	ipAddress string,
	userAgent string,
) (*model.BackupRecord, error) {
	if !isValidBackupType(backupType) {
		return nil, ErrInvalidBackupType
	}

	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	snapshotPath := filepath.Join(os.TempDir(), fmt.Sprintf("assessv2_snapshot_%d.db", time.Now().UnixNano()))
	defer func() {
		_ = os.Remove(snapshotPath)
	}()
	if err := s.snapshotToSQLite(ctx, snapshotPath); err != nil {
		return nil, err
	}

	now := time.Now()
	backupName := fmt.Sprintf("assessv2_%s_%s.db.gz", now.Format("20060102_150405"), backupType)
	backupPath := filepath.Join(s.backupDir, backupName)
	if err := compressFileToGzip(snapshotPath, backupPath); err != nil {
		return nil, fmt.Errorf("failed to compress backup file: %w", err)
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat backup file: %w", err)
	}

	record := &model.BackupRecord{
		BackupName:  backupName,
		BackupPath:  backupPath,
		BackupType:  backupType,
		FileSize:    info.Size(),
		Description: strings.TrimSpace(description),
		CreatedBy:   createdBy,
		CreatedAt:   now.Unix(),
	}
	if err := s.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, fmt.Errorf("failed to create backup record: %w", err)
	}

	_ = s.enforceRetention(ctx)

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		createdBy,
		"backup",
		"backups",
		&targetID,
		map[string]any{
			"event":        "create_backup",
			"backup_name":  record.BackupName,
			"backup_type":  record.BackupType,
			"backup_path":  record.BackupPath,
			"file_size":    record.FileSize,
			"description":  record.Description,
			"retentionDir": s.backupDir,
		},
		ipAddress,
		userAgent,
	))

	return record, nil
}

func (s *BackupService) snapshotToSQLite(ctx context.Context, snapshotPath string) error {
	if removeErr := os.Remove(snapshotPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return fmt.Errorf("failed to cleanup stale snapshot file: %w", removeErr)
	}

	_ = s.db.WithContext(ctx).Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error
	statement := fmt.Sprintf("VACUUM INTO '%s'", escapeSQLiteString(snapshotPath))
	if err := s.db.WithContext(ctx).Exec(statement).Error; err != nil {
		return fmt.Errorf("failed to generate sqlite snapshot: %w", err)
	}
	return nil
}

func (s *BackupService) restoreFromSQLite(ctx context.Context, sourcePath string) error {
	alias := fmt.Sprintf("backupdb_%d", time.Now().UnixNano())
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
			return fmt.Errorf("failed to disable foreign keys before restore: %w", err)
		}
		reEnableFK := true
		defer func() {
			if reEnableFK {
				_ = tx.Exec("PRAGMA foreign_keys = ON").Error
			}
		}()

		if err := tx.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS %s", escapeSQLiteString(sourcePath), quoteSQLiteIdentifier(alias))).Error; err != nil {
			return fmt.Errorf("failed to attach restore source database: %w", err)
		}

		tables, err := listSQLiteTables(tx, "main")
		if err != nil {
			return err
		}

		excluded := map[string]struct{}{
			"schema_migrations": {},
			"audit_logs":        {},
			"backups":           {},
		}

		for _, tableName := range tables {
			if _, skipped := excluded[tableName]; skipped {
				continue
			}

			exists, err := sqliteTableExists(tx, alias, tableName)
			if err != nil {
				return err
			}
			if !exists {
				continue
			}

			columns, err := sharedSQLiteColumns(tx, alias, tableName)
			if err != nil {
				return err
			}
			if len(columns) == 0 {
				continue
			}

			if err := tx.Exec(fmt.Sprintf("DELETE FROM %s", quoteSQLiteIdentifier(tableName))).Error; err != nil {
				return fmt.Errorf("failed to clear table %s: %w", tableName, err)
			}

			columnList := joinSQLiteColumns(columns)
			insertSQL := fmt.Sprintf(
				"INSERT INTO %s (%s) SELECT %s FROM %s.%s",
				quoteSQLiteIdentifier(tableName),
				columnList,
				columnList,
				quoteSQLiteIdentifier(alias),
				quoteSQLiteIdentifier(tableName),
			)
			if err := tx.Exec(insertSQL).Error; err != nil {
				return fmt.Errorf("failed to restore table %s: %w", tableName, err)
			}
		}

		_ = tx.Exec("DELETE FROM sqlite_sequence").Error
		_ = tx.Exec(
			fmt.Sprintf(
				"INSERT INTO sqlite_sequence(name, seq) SELECT name, seq FROM %s.sqlite_sequence",
				quoteSQLiteIdentifier(alias),
			),
		).Error

		if err := tx.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
			return fmt.Errorf("failed to enable foreign keys after restore: %w", err)
		}
		reEnableFK = false
		return nil
	})
	if err != nil {
		return err
	}
	_ = s.db.WithContext(ctx).Exec(fmt.Sprintf("DETACH DATABASE %s", quoteSQLiteIdentifier(alias))).Error
	return nil
}

func (s *BackupService) enforceRetention(ctx context.Context) error {
	retentionDays := s.readIntSetting(ctx, "backup.retention_days", 7)
	maxCount := s.readIntSetting(ctx, "backup.max_count", 30)

	var records []model.BackupRecord
	if err := s.db.WithContext(ctx).
		Order("created_at DESC").
		Find(&records).Error; err != nil {
		return fmt.Errorf("failed to query backups for retention: %w", err)
	}
	if len(records) == 0 {
		return nil
	}

	cutoffAt := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	toDelete := make([]model.BackupRecord, 0)
	for index, item := range records {
		olderThanRetention := item.CreatedAt < cutoffAt
		overMaxCount := maxCount > 0 && index >= maxCount
		if olderThanRetention || overMaxCount {
			toDelete = append(toDelete, item)
		}
	}

	for _, item := range toDelete {
		if removeErr := os.Remove(item.BackupPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			log.Printf("backup retention remove file failed id=%d path=%s err=%v", item.ID, item.BackupPath, removeErr)
			continue
		}
		if err := s.db.WithContext(ctx).Delete(&model.BackupRecord{}, item.ID).Error; err != nil {
			log.Printf("backup retention remove record failed id=%d err=%v", item.ID, err)
		}
	}
	return nil
}

func (s *BackupService) autoBackupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	s.tryAutoBackup(context.Background(), time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.tryAutoBackup(context.Background(), now)
		}
	}
}

func (s *BackupService) tryAutoBackup(ctx context.Context, now time.Time) {
	if !s.readBoolSetting(ctx, "backup.auto_enabled", true) {
		return
	}

	location := s.readTimeLocationSetting(ctx, "system.timezone", time.Local)
	now = now.In(location)
	targetHour, targetMinute := s.readBackupTimeSetting(ctx, "backup.auto_time", 2, 0)
	if now.Hour() != targetHour || now.Minute() != targetMinute {
		return
	}

	dayKey := now.Format("2006-01-02")
	s.autoBackupMu.Lock()
	if s.lastAutoDay == dayKey {
		s.autoBackupMu.Unlock()
		return
	}
	s.lastAutoDay = dayKey
	s.autoBackupMu.Unlock()

	_, err := s.createBackup(
		ctx,
		BackupTypeAuto,
		nil,
		fmt.Sprintf("auto backup at %s", now.Format(time.RFC3339)),
		"",
		"auto-backup-scheduler",
	)
	if err != nil {
		log.Printf("auto backup failed: %v", err)
	}
}

func (s *BackupService) readIntSetting(ctx context.Context, key string, fallback int) int {
	var setting model.SystemSetting
	if err := s.db.WithContext(ctx).Where("setting_key = ?", key).First(&setting).Error; err != nil {
		return fallback
	}
	value, err := strconv.Atoi(strings.TrimSpace(setting.SettingValue))
	if err != nil {
		return fallback
	}
	return value
}

func (s *BackupService) readBoolSetting(ctx context.Context, key string, fallback bool) bool {
	var setting model.SystemSetting
	if err := s.db.WithContext(ctx).Where("setting_key = ?", key).First(&setting).Error; err != nil {
		return fallback
	}
	value, err := strconv.ParseBool(strings.TrimSpace(setting.SettingValue))
	if err != nil {
		return fallback
	}
	return value
}

func (s *BackupService) readBackupTimeSetting(ctx context.Context, key string, fallbackHour, fallbackMinute int) (int, int) {
	var setting model.SystemSetting
	if err := s.db.WithContext(ctx).Where("setting_key = ?", key).First(&setting).Error; err != nil {
		return fallbackHour, fallbackMinute
	}
	parsed, err := time.Parse("15:04", strings.TrimSpace(setting.SettingValue))
	if err != nil {
		return fallbackHour, fallbackMinute
	}
	return parsed.Hour(), parsed.Minute()
}

func (s *BackupService) readTimeLocationSetting(ctx context.Context, key string, fallback *time.Location) *time.Location {
	var setting model.SystemSetting
	if err := s.db.WithContext(ctx).Where("setting_key = ?", key).First(&setting).Error; err != nil {
		return fallback
	}
	location, err := time.LoadLocation(strings.TrimSpace(setting.SettingValue))
	if err != nil {
		return fallback
	}
	return location
}

func backupRecordToDTO(record model.BackupRecord) BackupRecordDTO {
	return BackupRecordDTO{
		ID:          record.ID,
		BackupName:  record.BackupName,
		BackupType:  record.BackupType,
		FileSize:    record.FileSize,
		Description: record.Description,
		CreatedBy:   record.CreatedBy,
		CreatedAt:   record.CreatedAt,
	}
}

func isValidBackupType(backupType string) bool {
	_, ok := validBackupTypes[strings.TrimSpace(backupType)]
	return ok
}

func compressFileToGzip(sourcePath, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer target.Close()

	gzipWriter := gzip.NewWriter(target)
	if _, err := io.Copy(gzipWriter, source); err != nil {
		_ = gzipWriter.Close()
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		return err
	}
	return target.Sync()
}

func decompressGzipFile(sourcePath, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	reader, err := gzip.NewReader(source)
	if err != nil {
		return err
	}
	defer reader.Close()

	target, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer target.Close()

	if _, err := io.Copy(target, reader); err != nil {
		return err
	}
	return target.Sync()
}

func escapeSQLiteString(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func quoteSQLiteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func joinSQLiteColumns(columns []string) string {
	parts := make([]string, 0, len(columns))
	for _, item := range columns {
		parts = append(parts, quoteSQLiteIdentifier(item))
	}
	return strings.Join(parts, ", ")
}

func listSQLiteTables(tx *gorm.DB, schema string) ([]string, error) {
	type tableRow struct {
		Name string `gorm:"column:name"`
	}
	var rows []tableRow
	query := fmt.Sprintf(
		"SELECT name FROM %s.sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%%' ORDER BY name ASC",
		schema,
	)
	if err := tx.Raw(query).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list sqlite tables: %w", err)
	}
	result := make([]string, 0, len(rows))
	for _, item := range rows {
		result = append(result, item.Name)
	}
	return result, nil
}

func sqliteTableExists(tx *gorm.DB, schema string, tableName string) (bool, error) {
	var count int64
	query := fmt.Sprintf(
		"SELECT COUNT(1) FROM %s.sqlite_master WHERE type='table' AND name = ?",
		schema,
	)
	if err := tx.Raw(query, tableName).Scan(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check sqlite table exists for %s.%s: %w", schema, tableName, err)
	}
	return count > 0, nil
}

func sharedSQLiteColumns(tx *gorm.DB, backupSchema string, tableName string) ([]string, error) {
	mainColumns, err := listSQLiteColumns(tx, "main", tableName)
	if err != nil {
		return nil, err
	}
	backupColumns, err := listSQLiteColumns(tx, backupSchema, tableName)
	if err != nil {
		return nil, err
	}
	backupSet := make(map[string]struct{}, len(backupColumns))
	for _, item := range backupColumns {
		backupSet[item] = struct{}{}
	}
	shared := make([]string, 0, len(mainColumns))
	for _, item := range mainColumns {
		if _, exists := backupSet[item]; exists {
			shared = append(shared, item)
		}
	}
	return shared, nil
}

func listSQLiteColumns(tx *gorm.DB, schema string, tableName string) ([]string, error) {
	type pragmaRow struct {
		Name string `gorm:"column:name"`
	}
	query := fmt.Sprintf(
		"PRAGMA %s.table_info(%s)",
		schema,
		quoteSQLiteIdentifier(tableName),
	)
	var rows []pragmaRow
	if err := tx.Raw(query).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list sqlite columns for %s.%s: %w", schema, tableName, err)
	}
	result := make([]string, 0, len(rows))
	for _, item := range rows {
		result = append(result, item.Name)
	}
	return result, nil
}
