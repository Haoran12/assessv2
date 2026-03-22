package service

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type OrgPackageCreateInput struct {
	RootOrganizationID     uint
	Description            string
	IncludeEmployeeHistory bool
}

type OrgPackageRestoreInput struct {
	ConfirmText              string
	Mode                     string
	TargetRootOrganizationID uint
}

type OrgPackageListInput struct {
	Page               int
	PageSize           int
	RootOrganizationID *uint
}

type OrgPackageRecordDTO struct {
	ID                        uint             `json:"id"`
	BackupName                string           `json:"backupName"`
	FileSize                  int64            `json:"fileSize"`
	Description               string           `json:"description"`
	CreatedBy                 *uint            `json:"createdBy,omitempty"`
	CreatedAt                 int64            `json:"createdAt"`
	RootOrganizationID        uint             `json:"rootOrganizationId"`
	FormatVersion             string           `json:"formatVersion"`
	ChecksumSHA256            string           `json:"checksumSha256"`
	ScopedOrganizationIDs     []uint           `json:"scopedOrganizationIds,omitempty"`
	TableRowCounts            map[string]int64 `json:"tableRowCounts,omitempty"`
	SanitizedHistoryRefsCount int64            `json:"sanitizedHistoryRefsCount"`
}

type OrgPackageListResult struct {
	Items    []OrgPackageRecordDTO `json:"items"`
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"pageSize"`
}

type orgPackageManifest struct {
	PackageType               string           `json:"packageType"`
	FormatVersion             string           `json:"formatVersion"`
	CreatedAt                 int64            `json:"createdAt"`
	RootOrganizationID        uint             `json:"rootOrganizationId"`
	ScopedOrganizationIDs     []uint           `json:"scopedOrganizationIds"`
	TableRowCounts            map[string]int64 `json:"tableRowCounts"`
	BusinessSchemaVersion     int              `json:"businessSchemaVersion"`
	IncludeEmployeeHistory    bool             `json:"includeEmployeeHistory"`
	SanitizedHistoryRefsCount int64            `json:"sanitizedHistoryRefsCount"`
	Checksum                  string           `json:"checksum"`
}

func (s *BackupService) CreateOrgPackage(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	input OrgPackageCreateInput,
	ipAddress string,
	userAgent string,
) (*OrgPackageRecordDTO, error) {
	if input.RootOrganizationID == 0 {
		return nil, ErrInvalidParam
	}
	if err := s.ensureOrgPackageScopeAccess(ctx, claims, input.RootOrganizationID); err != nil {
		return nil, err
	}
	if err := s.ensureOrganizationExists(ctx, input.RootOrganizationID); err != nil {
		return nil, err
	}

	scopedOrgIDs, err := resolveOrganizationIDs(ctx, s.db, input.RootOrganizationID, true)
	if err != nil {
		return nil, err
	}
	if len(scopedOrgIDs) == 0 {
		return nil, ErrOrganizationNotFound
	}
	sort.Slice(scopedOrgIDs, func(i, j int) bool { return scopedOrgIDs[i] < scopedOrgIDs[j] })

	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	sourceSnapshotPath := filepath.Join(os.TempDir(), fmt.Sprintf("assessv2_orgpkg_source_%d.db", time.Now().UnixNano()))
	defer func() {
		_ = os.Remove(sourceSnapshotPath)
	}()
	if err := s.snapshotToSQLite(ctx, sourceSnapshotPath); err != nil {
		return nil, err
	}

	dataDBPath := filepath.Join(os.TempDir(), fmt.Sprintf("assessv2_orgpkg_data_%d.db", time.Now().UnixNano()))
	defer func() {
		_ = os.Remove(dataDBPath)
	}()

	tableRowCounts, sanitizedHistoryRefsCount, err := buildOrgPackageDataDB(sourceSnapshotPath, dataDBPath, scopedOrgIDs, input.IncludeEmployeeHistory)
	if err != nil {
		return nil, err
	}
	dataChecksum, err := computeFileSHA256(dataDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to compute data checksum: %w", err)
	}

	now := time.Now()
	manifest := orgPackageManifest{
		PackageType:               BackupContentTypeOrgLogical,
		FormatVersion:             backupFormatVersionOrgPackage,
		CreatedAt:                 now.Unix(),
		RootOrganizationID:        input.RootOrganizationID,
		ScopedOrganizationIDs:     scopedOrgIDs,
		TableRowCounts:            tableRowCounts,
		BusinessSchemaVersion:     s.readBusinessSchemaVersion(ctx),
		IncludeEmployeeHistory:    input.IncludeEmployeeHistory,
		SanitizedHistoryRefsCount: sanitizedHistoryRefsCount,
		Checksum:                  dataChecksum,
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal org package manifest: %w", err)
	}

	backupName := fmt.Sprintf("assessv2_orgpkg_%s_org%d.tar.gz", now.Format("20060102_150405"), input.RootOrganizationID)
	backupPath := filepath.Join(s.backupDir, backupName)
	if err := writeOrgPackageArchive(backupPath, manifestJSON, dataDBPath, now); err != nil {
		return nil, err
	}
	info, err := os.Stat(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat org package file: %w", err)
	}
	archiveChecksum, err := computeFileSHA256(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to compute package checksum: %w", err)
	}

	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID)
	record := &model.BackupRecord{
		BackupName:     backupName,
		BackupPath:     backupPath,
		BackupType:     BackupTypeManual,
		ContentType:    BackupContentTypeOrgLogical,
		ScopeType:      BackupScopeTypeOrganization,
		ScopeOrgID:     uintPtr(input.RootOrganizationID),
		FormatVersion:  backupFormatVersionOrgPackage,
		ChecksumSHA256: archiveChecksum,
		ManifestJSON:   string(manifestJSON),
		FileSize:       info.Size(),
		Description:    strings.TrimSpace(input.Description),
		CreatedBy:      operatorRef,
		CreatedAt:      now.Unix(),
	}
	if err := s.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, fmt.Errorf("failed to create org package backup record: %w", err)
	}

	_ = s.enforceRetention(ctx)

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		operatorRef,
		"backup",
		"backups",
		&targetID,
		map[string]any{
			"event":                     "create_org_package",
			"backup_name":               record.BackupName,
			"content_type":              record.ContentType,
			"scope_org_id":              record.ScopeOrgID,
			"format_version":            record.FormatVersion,
			"include_employee_history":  input.IncludeEmployeeHistory,
			"scoped_organization_count": len(scopedOrgIDs),
			"file_size":                 record.FileSize,
			"description":               record.Description,
		},
		ipAddress,
		userAgent,
	))

	dto := orgPackageRecordFromRecord(*record)
	dto.ScopedOrganizationIDs = append([]uint(nil), manifest.ScopedOrganizationIDs...)
	dto.TableRowCounts = manifest.TableRowCounts
	dto.SanitizedHistoryRefsCount = manifest.SanitizedHistoryRefsCount
	return &dto, nil
}

func (s *BackupService) ListOrgPackages(ctx context.Context, claims *auth.Claims, input OrgPackageListInput) (*OrgPackageListResult, error) {
	page := input.Page
	if page <= 0 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}

	query := s.db.WithContext(ctx).
		Model(&model.BackupRecord{}).
		Where("content_type = ? AND scope_type = ?", BackupContentTypeOrgLogical, BackupScopeTypeOrganization)

	if input.RootOrganizationID != nil && *input.RootOrganizationID > 0 {
		query = query.Where("scope_org_id = ?", *input.RootOrganizationID)
	}

	if !isRootClaims(claims) {
		scope, err := requireOrgWriteScope(ctx, s.db, claims)
		if err != nil {
			return nil, err
		}
		if !scope.unrestricted {
			allowed := make([]uint, 0, len(scope.allowedOrgID))
			for orgID := range scope.allowedOrgID {
				allowed = append(allowed, orgID)
			}
			if len(allowed) == 0 {
				return &OrgPackageListResult{Items: []OrgPackageRecordDTO{}, Total: 0, Page: page, PageSize: pageSize}, nil
			}
			query = query.Where("scope_org_id IN ?", allowed)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count org package records: %w", err)
	}

	var records []model.BackupRecord
	if err := query.
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("failed to query org package records: %w", err)
	}

	items := make([]OrgPackageRecordDTO, 0, len(records))
	for _, record := range records {
		dto := orgPackageRecordFromRecord(record)
		if manifest, ok := parseOrgPackageManifest(record.ManifestJSON); ok {
			dto.ScopedOrganizationIDs = manifest.ScopedOrganizationIDs
			dto.TableRowCounts = manifest.TableRowCounts
			dto.SanitizedHistoryRefsCount = manifest.SanitizedHistoryRefsCount
		}
		items = append(items, dto)
	}

	return &OrgPackageListResult{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *BackupService) ResolveOrgPackageDownloadPath(ctx context.Context, claims *auth.Claims, backupID uint) (string, string, error) {
	record, err := s.getOrgPackageRecord(ctx, backupID)
	if err != nil {
		return "", "", err
	}
	if record.ScopeOrgID == nil || *record.ScopeOrgID == 0 {
		return "", "", ErrBackupPackageBroken
	}
	if err := s.ensureOrgPackageScopeAccess(ctx, claims, *record.ScopeOrgID); err != nil {
		return "", "", err
	}
	if _, err := os.Stat(record.BackupPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", ErrBackupNotFound
		}
		return "", "", err
	}
	return record.BackupPath, record.BackupName, nil
}

func (s *BackupService) RestoreOrgPackage(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	backupID uint,
	input OrgPackageRestoreInput,
	ipAddress string,
	userAgent string,
) error {
	if strings.TrimSpace(input.ConfirmText) != orgRestoreConfirmText {
		return ErrBackupConfirmMismatch
	}
	if strings.TrimSpace(input.Mode) != RestoreModeReplaceScope {
		return ErrInvalidBackupRestoreMode
	}
	if input.TargetRootOrganizationID == 0 {
		return ErrInvalidParam
	}

	record, err := s.getOrgPackageRecord(ctx, backupID)
	if err != nil {
		return err
	}
	if record.ScopeOrgID == nil || *record.ScopeOrgID == 0 {
		return ErrBackupPackageBroken
	}
	if err := s.ensureOrgPackageScopeAccess(ctx, claims, *record.ScopeOrgID); err != nil {
		return err
	}
	if *record.ScopeOrgID != input.TargetRootOrganizationID {
		return ErrBackupTargetMismatch
	}

	if _, statErr := os.Stat(record.BackupPath); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return ErrBackupNotFound
		}
		return fmt.Errorf("failed to access org package file: %w", statErr)
	}

	if strings.TrimSpace(record.ChecksumSHA256) != "" {
		actualChecksum, checksumErr := computeFileSHA256(record.BackupPath)
		if checksumErr != nil {
			return fmt.Errorf("failed to compute package checksum: %w", checksumErr)
		}
		if !strings.EqualFold(actualChecksum, strings.TrimSpace(record.ChecksumSHA256)) {
			return ErrBackupPackageBroken
		}
	}

	manifest, packageDataPath, cleanup, err := extractOrgPackageArchive(record.BackupPath)
	if err != nil {
		return err
	}
	defer cleanup()

	if manifest.PackageType != BackupContentTypeOrgLogical ||
		manifest.FormatVersion != backupFormatVersionOrgPackage ||
		manifest.RootOrganizationID != input.TargetRootOrganizationID ||
		manifest.RootOrganizationID != *record.ScopeOrgID {
		return ErrBackupPackageBroken
	}
	if strings.TrimSpace(manifest.Checksum) != "" {
		dataChecksum, checksumErr := computeFileSHA256(packageDataPath)
		if checksumErr != nil {
			return fmt.Errorf("failed to compute org package data checksum: %w", checksumErr)
		}
		if !strings.EqualFold(dataChecksum, strings.TrimSpace(manifest.Checksum)) {
			return ErrBackupPackageBroken
		}
	}

	beforeRestore, err := s.createBackup(
		ctx,
		BackupTypeBeforeRestore,
		resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID),
		fmt.Sprintf("before org package restore backup id=%d", backupID),
		ipAddress,
		userAgent,
	)
	if err != nil {
		return fmt.Errorf("failed to create pre-restore backup: %w", err)
	}

	if err := s.restoreOrgPackageReplaceScope(ctx, packageDataPath, input.TargetRootOrganizationID); err != nil {
		return err
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		&operatorID,
		"restore",
		"backups",
		&targetID,
		map[string]any{
			"event":                    "restore_org_package",
			"backup_id":                record.ID,
			"backup_name":              record.BackupName,
			"target_root_organization": input.TargetRootOrganizationID,
			"restore_mode":             input.Mode,
			"before_restore_backup":    beforeRestore.ID,
		},
		ipAddress,
		userAgent,
	))

	return nil
}

func (s *BackupService) ensureOrgPackageScopeAccess(ctx context.Context, claims *auth.Claims, organizationID uint) error {
	if claims == nil {
		return ErrForbidden
	}
	if organizationID == 0 {
		return ErrInvalidParam
	}
	if isRootClaims(claims) {
		return nil
	}
	scope, err := requireOrgWriteScope(ctx, s.db, claims)
	if err != nil {
		return err
	}
	if !scope.allowsOrganization(organizationID) {
		return ErrForbidden
	}
	return nil
}

func (s *BackupService) ensureOrganizationExists(ctx context.Context, organizationID uint) error {
	var count int64
	if err := s.db.WithContext(ctx).Table("organizations").Where("id = ? AND deleted_at IS NULL", organizationID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to verify organization: %w", err)
	}
	if count == 0 {
		return ErrOrganizationNotFound
	}
	return nil
}

func (s *BackupService) getOrgPackageRecord(ctx context.Context, backupID uint) (*model.BackupRecord, error) {
	var record model.BackupRecord
	if err := s.db.WithContext(ctx).
		Where("id = ? AND content_type = ? AND scope_type = ?", backupID, BackupContentTypeOrgLogical, BackupScopeTypeOrganization).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBackupNotFound
		}
		return nil, fmt.Errorf("failed to query org package record: %w", err)
	}
	return &record, nil
}

func (s *BackupService) readBusinessSchemaVersion(ctx context.Context) int {
	type row struct {
		Version int `gorm:"column:version"`
	}
	var item row
	if err := s.db.WithContext(ctx).Raw("SELECT COALESCE(MAX(version), 0) AS version FROM schema_migrations").Scan(&item).Error; err != nil {
		return 0
	}
	return item.Version
}

func (s *BackupService) restoreOrgPackageReplaceScope(ctx context.Context, packageDataPath string, targetRootOrganizationID uint) error {
	scopedOrgIDs, err := resolveOrganizationIDs(ctx, s.db, targetRootOrganizationID, true)
	if err != nil {
		return err
	}
	if len(scopedOrgIDs) == 0 {
		return ErrOrganizationNotFound
	}

	orgPlaceholders := sqlPlaceholders(len(scopedOrgIDs))
	orgArgs := uintArgs(scopedOrgIDs)
	assessmentScopeSQL := fmt.Sprintf("assessment_id IN (SELECT id FROM assessment_sessions WHERE organization_id IN (%s))", orgPlaceholders)
	employeeScopeSQL := fmt.Sprintf("employee_id IN (SELECT id FROM employees WHERE organization_id IN (%s))", orgPlaceholders)

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
			return fmt.Errorf("failed to disable foreign keys: %w", err)
		}
		reEnableFK := true
		defer func() {
			if reEnableFK {
				_ = tx.Exec("PRAGMA foreign_keys = ON").Error
			}
		}()

		packageSchema := "orgpkg"
		attachSQL := fmt.Sprintf("ATTACH DATABASE '%s' AS %s", escapeSQLiteString(packageDataPath), quoteSQLiteIdentifier(packageSchema))
		if err := tx.Exec(attachSQL).Error; err != nil {
			return fmt.Errorf("failed to attach org package database: %w", err)
		}
		defer func() {
			_ = tx.Exec(fmt.Sprintf("DETACH DATABASE %s", quoteSQLiteIdentifier(packageSchema))).Error
		}()

		if err := tx.Exec("DELETE FROM assessment_object_module_scores WHERE "+assessmentScopeSQL, orgArgs...).Error; err != nil {
			return fmt.Errorf("failed to clear assessment_object_module_scores: %w", err)
		}
		if err := tx.Exec("DELETE FROM assessment_session_objects WHERE "+assessmentScopeSQL, orgArgs...).Error; err != nil {
			return fmt.Errorf("failed to clear assessment_session_objects: %w", err)
		}
		if hasRuleBindings, _ := sqliteTableExists(tx, "main", "assessment_rule_bindings"); hasRuleBindings {
			if err := tx.Exec("DELETE FROM assessment_rule_bindings WHERE "+assessmentScopeSQL, orgArgs...).Error; err != nil {
				return fmt.Errorf("failed to clear assessment_rule_bindings: %w", err)
			}
		}
		if err := tx.Exec("DELETE FROM rule_files WHERE "+assessmentScopeSQL, orgArgs...).Error; err != nil {
			return fmt.Errorf("failed to clear rule_files: %w", err)
		}
		if err := tx.Exec("DELETE FROM assessment_object_groups WHERE "+assessmentScopeSQL, orgArgs...).Error; err != nil {
			return fmt.Errorf("failed to clear assessment_object_groups: %w", err)
		}
		if err := tx.Exec("DELETE FROM assessment_session_periods WHERE "+assessmentScopeSQL, orgArgs...).Error; err != nil {
			return fmt.Errorf("failed to clear assessment_session_periods: %w", err)
		}
		if err := tx.Exec("DELETE FROM assessment_sessions WHERE organization_id IN ("+orgPlaceholders+")", orgArgs...).Error; err != nil {
			return fmt.Errorf("failed to clear assessment_sessions: %w", err)
		}
		if hasHistory, _ := sqliteTableExists(tx, "main", "employee_history"); hasHistory {
			if err := tx.Exec("DELETE FROM employee_history WHERE "+employeeScopeSQL, orgArgs...).Error; err != nil {
				return fmt.Errorf("failed to clear employee_history: %w", err)
			}
		}
		if err := tx.Exec("DELETE FROM employees WHERE organization_id IN ("+orgPlaceholders+")", orgArgs...).Error; err != nil {
			return fmt.Errorf("failed to clear employees: %w", err)
		}
		if err := tx.Exec("DELETE FROM departments WHERE organization_id IN ("+orgPlaceholders+")", orgArgs...).Error; err != nil {
			return fmt.Errorf("failed to clear departments: %w", err)
		}

		orgDeleteArgs := append([]any{}, orgArgs...)
		orgDeleteArgs = append(orgDeleteArgs, targetRootOrganizationID)
		if err := tx.Exec("DELETE FROM organizations WHERE id IN ("+orgPlaceholders+") AND id <> ?", orgDeleteArgs...).Error; err != nil {
			return fmt.Errorf("failed to clear organizations: %w", err)
		}

		if err := copyAttachedTableIntoMain(tx, packageSchema, "organizations", "id IN ("+orgPlaceholders+")", orgArgs...); err != nil {
			return err
		}
		if err := copyAttachedTableIntoMain(tx, packageSchema, "departments", "organization_id IN ("+orgPlaceholders+")", orgArgs...); err != nil {
			return err
		}
		if err := copyAttachedTableIntoMain(tx, packageSchema, "employees", "organization_id IN ("+orgPlaceholders+")", orgArgs...); err != nil {
			return err
		}
		if hasHistoryInPkg, _ := sqliteTableExists(tx, packageSchema, "employee_history"); hasHistoryInPkg {
			if err := copyAttachedTableIntoMain(
				tx,
				packageSchema,
				"employee_history",
				"employee_id IN (SELECT id FROM "+quoteSQLiteIdentifier(packageSchema)+".employees WHERE organization_id IN ("+orgPlaceholders+"))",
				orgArgs...,
			); err != nil {
				return err
			}
		}
		if err := copyAttachedTableIntoMain(tx, packageSchema, "assessment_sessions", "organization_id IN ("+orgPlaceholders+")", orgArgs...); err != nil {
			return err
		}
		if err := copyAttachedTableIntoMain(
			tx,
			packageSchema,
			"assessment_session_periods",
			"assessment_id IN (SELECT id FROM "+quoteSQLiteIdentifier(packageSchema)+".assessment_sessions WHERE organization_id IN ("+orgPlaceholders+"))",
			orgArgs...,
		); err != nil {
			return err
		}
		if err := copyAttachedTableIntoMain(
			tx,
			packageSchema,
			"assessment_object_groups",
			"assessment_id IN (SELECT id FROM "+quoteSQLiteIdentifier(packageSchema)+".assessment_sessions WHERE organization_id IN ("+orgPlaceholders+"))",
			orgArgs...,
		); err != nil {
			return err
		}
		if err := copyAttachedTableIntoMain(
			tx,
			packageSchema,
			"rule_files",
			"assessment_id IN (SELECT id FROM "+quoteSQLiteIdentifier(packageSchema)+".assessment_sessions WHERE organization_id IN ("+orgPlaceholders+"))",
			orgArgs...,
		); err != nil {
			return err
		}
		if hasRuleBindingsMain, _ := sqliteTableExists(tx, "main", "assessment_rule_bindings"); hasRuleBindingsMain {
			if hasRuleBindingsPkg, _ := sqliteTableExists(tx, packageSchema, "assessment_rule_bindings"); hasRuleBindingsPkg {
				if err := copyAttachedTableIntoMain(
					tx,
					packageSchema,
					"assessment_rule_bindings",
					"assessment_id IN (SELECT id FROM "+quoteSQLiteIdentifier(packageSchema)+".assessment_sessions WHERE organization_id IN ("+orgPlaceholders+"))",
					orgArgs...,
				); err != nil {
					return err
				}
			}
		}
		if err := copyAttachedTableIntoMain(
			tx,
			packageSchema,
			"assessment_session_objects",
			"assessment_id IN (SELECT id FROM "+quoteSQLiteIdentifier(packageSchema)+".assessment_sessions WHERE organization_id IN ("+orgPlaceholders+"))",
			orgArgs...,
		); err != nil {
			return err
		}
		if err := copyAttachedTableIntoMain(
			tx,
			packageSchema,
			"assessment_object_module_scores",
			"assessment_id IN (SELECT id FROM "+quoteSQLiteIdentifier(packageSchema)+".assessment_sessions WHERE organization_id IN ("+orgPlaceholders+"))",
			orgArgs...,
		); err != nil {
			return err
		}

		if err := tx.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
			return fmt.Errorf("failed to enable foreign keys after restore: %w", err)
		}
		reEnableFK = false

		violations, err := foreignKeyViolationCount(tx)
		if err != nil {
			return err
		}
		if violations > 0 {
			return fmt.Errorf("foreign key check failed with %d violations", violations)
		}
		return nil
	})
}

func buildOrgPackageDataDB(sourcePath string, targetPath string, scopedOrgIDs []uint, includeEmployeeHistory bool) (map[string]int64, int64, error) {
	if removeErr := os.Remove(targetPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return nil, 0, fmt.Errorf("failed to cleanup stale org package data db: %w", removeErr)
	}

	targetDB, err := gorm.Open(sqlite.Open(targetPath), &gorm.Config{})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open org package data db: %w", err)
	}
	sqlDB, err := targetDB.DB()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to acquire org package sql db: %w", err)
	}
	defer func() {
		_ = sqlDB.Close()
	}()

	rowCounts := map[string]int64{}
	var sanitizedHistoryRefsCount int64

	err = targetDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
			return fmt.Errorf("failed to disable foreign keys for export db: %w", err)
		}
		defer func() {
			_ = tx.Exec("PRAGMA foreign_keys = ON").Error
		}()

		sourceSchema := "source_main"
		attachSQL := fmt.Sprintf("ATTACH DATABASE '%s' AS %s", escapeSQLiteString(sourcePath), quoteSQLiteIdentifier(sourceSchema))
		if err := tx.Exec(attachSQL).Error; err != nil {
			return fmt.Errorf("failed to attach source snapshot: %w", err)
		}
		defer func() {
			_ = tx.Exec(fmt.Sprintf("DETACH DATABASE %s", quoteSQLiteIdentifier(sourceSchema))).Error
		}()

		tables := []string{
			"organizations",
			"departments",
			"employees",
			"assessment_sessions",
			"assessment_session_periods",
			"assessment_object_groups",
			"assessment_session_objects",
			"assessment_object_module_scores",
			"rule_files",
		}
		if includeEmployeeHistory {
			tables = append(tables, "employee_history")
		}
		hasRuleBindings, err := sqliteTableExists(tx, sourceSchema, "assessment_rule_bindings")
		if err != nil {
			return err
		}
		if hasRuleBindings {
			tables = append(tables, "assessment_rule_bindings")
		}

		for _, tableName := range tables {
			if err := cloneSQLiteTableWithIndexes(tx, sourceSchema, tableName); err != nil {
				return err
			}
		}

		orgPlaceholders := sqlPlaceholders(len(scopedOrgIDs))
		orgArgs := uintArgs(scopedOrgIDs)

		if rowCounts["organizations"], err = copyRowsIntoMainFromSource(tx, sourceSchema, "organizations", "id IN ("+orgPlaceholders+") AND deleted_at IS NULL", orgArgs...); err != nil {
			return err
		}
		if rowCounts["departments"], err = copyRowsIntoMainFromSource(tx, sourceSchema, "departments", "organization_id IN ("+orgPlaceholders+") AND deleted_at IS NULL", orgArgs...); err != nil {
			return err
		}
		if rowCounts["employees"], err = copyRowsIntoMainFromSource(tx, sourceSchema, "employees", "organization_id IN ("+orgPlaceholders+") AND deleted_at IS NULL", orgArgs...); err != nil {
			return err
		}
		if includeEmployeeHistory {
			if rowCounts["employee_history"], err = copyRowsIntoMainFromSource(tx, sourceSchema, "employee_history", "employee_id IN (SELECT id FROM "+quoteSQLiteIdentifier(sourceSchema)+".employees WHERE organization_id IN ("+orgPlaceholders+") AND deleted_at IS NULL)", orgArgs...); err != nil {
				return err
			}
			if err := tx.Raw(`
SELECT COUNT(1) AS cnt
FROM employee_history
WHERE (old_organization_id IS NOT NULL AND old_organization_id NOT IN (SELECT id FROM organizations))
   OR (new_organization_id IS NOT NULL AND new_organization_id NOT IN (SELECT id FROM organizations))
   OR (old_department_id IS NOT NULL AND old_department_id NOT IN (SELECT id FROM departments))
   OR (new_department_id IS NOT NULL AND new_department_id NOT IN (SELECT id FROM departments))
`).Scan(&sanitizedHistoryRefsCount).Error; err != nil {
				return fmt.Errorf("failed to count sanitized history refs: %w", err)
			}
			if err := tx.Exec(`
UPDATE employee_history
SET
    old_organization_id = CASE
        WHEN old_organization_id IS NOT NULL AND old_organization_id NOT IN (SELECT id FROM organizations) THEN NULL
        ELSE old_organization_id
    END,
    new_organization_id = CASE
        WHEN new_organization_id IS NOT NULL AND new_organization_id NOT IN (SELECT id FROM organizations) THEN NULL
        ELSE new_organization_id
    END,
    old_department_id = CASE
        WHEN old_department_id IS NOT NULL AND old_department_id NOT IN (SELECT id FROM departments) THEN NULL
        ELSE old_department_id
    END,
    new_department_id = CASE
        WHEN new_department_id IS NOT NULL AND new_department_id NOT IN (SELECT id FROM departments) THEN NULL
        ELSE new_department_id
    END
`).Error; err != nil {
				return fmt.Errorf("failed to sanitize employee_history refs: %w", err)
			}
		}
		if rowCounts["assessment_sessions"], err = copyRowsIntoMainFromSource(tx, sourceSchema, "assessment_sessions", "organization_id IN ("+orgPlaceholders+")", orgArgs...); err != nil {
			return err
		}
		assessmentScopeInSource := "assessment_id IN (SELECT id FROM " + quoteSQLiteIdentifier(sourceSchema) + ".assessment_sessions WHERE organization_id IN (" + orgPlaceholders + "))"
		if rowCounts["assessment_session_periods"], err = copyRowsIntoMainFromSource(tx, sourceSchema, "assessment_session_periods", assessmentScopeInSource, orgArgs...); err != nil {
			return err
		}
		if rowCounts["assessment_object_groups"], err = copyRowsIntoMainFromSource(tx, sourceSchema, "assessment_object_groups", assessmentScopeInSource, orgArgs...); err != nil {
			return err
		}
		if rowCounts["assessment_session_objects"], err = copyRowsIntoMainFromSource(tx, sourceSchema, "assessment_session_objects", assessmentScopeInSource, orgArgs...); err != nil {
			return err
		}
		if rowCounts["assessment_object_module_scores"], err = copyRowsIntoMainFromSource(tx, sourceSchema, "assessment_object_module_scores", assessmentScopeInSource, orgArgs...); err != nil {
			return err
		}
		if rowCounts["rule_files"], err = copyRowsIntoMainFromSource(tx, sourceSchema, "rule_files", assessmentScopeInSource, orgArgs...); err != nil {
			return err
		}
		if hasRuleBindings {
			if rowCounts["assessment_rule_bindings"], err = copyRowsIntoMainFromSource(tx, sourceSchema, "assessment_rule_bindings", assessmentScopeInSource, orgArgs...); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return rowCounts, sanitizedHistoryRefsCount, nil
}

func orgPackageRecordFromRecord(record model.BackupRecord) OrgPackageRecordDTO {
	rootOrgID := uint(0)
	if record.ScopeOrgID != nil {
		rootOrgID = *record.ScopeOrgID
	}
	return OrgPackageRecordDTO{
		ID:                 record.ID,
		BackupName:         record.BackupName,
		FileSize:           record.FileSize,
		Description:        record.Description,
		CreatedBy:          record.CreatedBy,
		CreatedAt:          record.CreatedAt,
		RootOrganizationID: rootOrgID,
		FormatVersion:      strings.TrimSpace(record.FormatVersion),
		ChecksumSHA256:     strings.TrimSpace(record.ChecksumSHA256),
	}
}

func parseOrgPackageManifest(raw string) (*orgPackageManifest, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, false
	}
	var manifest orgPackageManifest
	if err := json.Unmarshal([]byte(text), &manifest); err != nil {
		return nil, false
	}
	return &manifest, true
}

func writeOrgPackageArchive(archivePath string, manifestJSON []byte, dataDBPath string, modTime time.Time) error {
	file, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("failed to create org package archive: %w", err)
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	if err := tarWriter.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0o644, Size: int64(len(manifestJSON)), ModTime: modTime}); err != nil {
		return fmt.Errorf("failed to write manifest header: %w", err)
	}
	if _, err := tarWriter.Write(manifestJSON); err != nil {
		return fmt.Errorf("failed to write manifest content: %w", err)
	}

	dataStat, err := os.Stat(dataDBPath)
	if err != nil {
		return fmt.Errorf("failed to stat org package data db: %w", err)
	}
	if err := tarWriter.WriteHeader(&tar.Header{Name: "data.db", Mode: 0o644, Size: dataStat.Size(), ModTime: modTime}); err != nil {
		return fmt.Errorf("failed to write data db header: %w", err)
	}
	dataFile, err := os.Open(dataDBPath)
	if err != nil {
		return fmt.Errorf("failed to open org package data db: %w", err)
	}
	defer dataFile.Close()
	if _, err := io.Copy(tarWriter, dataFile); err != nil {
		return fmt.Errorf("failed to write data db content: %w", err)
	}

	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}
	return file.Sync()
}

func extractOrgPackageArchive(archivePath string) (*orgPackageManifest, string, func(), error) {
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return nil, "", func() {}, ErrBackupPackageBroken
	}
	defer archiveFile.Close()

	gzReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		return nil, "", func() {}, ErrBackupPackageBroken
	}
	defer gzReader.Close()

	tempDir, err := os.MkdirTemp("", "assessv2_orgpkg_extract_*")
	if err != nil {
		return nil, "", func() {}, fmt.Errorf("failed to create temp extraction dir: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	tarReader := tar.NewReader(gzReader)
	manifestPath := ""
	dataPath := ""
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			cleanup()
			return nil, "", func() {}, ErrBackupPackageBroken
		}
		name := filepath.Base(strings.TrimSpace(header.Name))
		if name == "." || name == "" {
			continue
		}

		targetPath := filepath.Join(tempDir, name)
		outFile, createErr := os.Create(targetPath)
		if createErr != nil {
			cleanup()
			return nil, "", func() {}, fmt.Errorf("failed to extract org package file: %w", createErr)
		}
		if _, copyErr := io.Copy(outFile, tarReader); copyErr != nil {
			_ = outFile.Close()
			cleanup()
			return nil, "", func() {}, fmt.Errorf("failed to read org package entry: %w", copyErr)
		}
		if syncErr := outFile.Sync(); syncErr != nil {
			_ = outFile.Close()
			cleanup()
			return nil, "", func() {}, fmt.Errorf("failed to sync org package entry: %w", syncErr)
		}
		_ = outFile.Close()

		switch name {
		case "manifest.json":
			manifestPath = targetPath
		case "data.db":
			dataPath = targetPath
		}
	}

	if manifestPath == "" || dataPath == "" {
		cleanup()
		return nil, "", func() {}, ErrBackupPackageBroken
	}

	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		cleanup()
		return nil, "", func() {}, fmt.Errorf("failed to read org package manifest: %w", err)
	}
	var manifest orgPackageManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		cleanup()
		return nil, "", func() {}, ErrBackupPackageBroken
	}

	return &manifest, dataPath, cleanup, nil
}

func cloneSQLiteTableWithIndexes(tx *gorm.DB, sourceSchema string, tableName string) error {
	var createTableSQL string
	query := fmt.Sprintf("SELECT sql FROM %s.sqlite_master WHERE type = 'table' AND name = ?", quoteSQLiteIdentifier(sourceSchema))
	if err := tx.Raw(query, tableName).Scan(&createTableSQL).Error; err != nil {
		return fmt.Errorf("failed to query table DDL for %s.%s: %w", sourceSchema, tableName, err)
	}
	createTableSQL = strings.TrimSpace(createTableSQL)
	if createTableSQL == "" {
		return fmt.Errorf("missing table DDL for %s.%s", sourceSchema, tableName)
	}
	if err := tx.Exec(createTableSQL).Error; err != nil {
		return fmt.Errorf("failed to create table %s in export db: %w", tableName, err)
	}

	type idxRow struct {
		SQL string `gorm:"column:sql"`
	}
	var indexes []idxRow
	indexQuery := fmt.Sprintf("SELECT sql FROM %s.sqlite_master WHERE type = 'index' AND tbl_name = ? AND sql IS NOT NULL", quoteSQLiteIdentifier(sourceSchema))
	if err := tx.Raw(indexQuery, tableName).Scan(&indexes).Error; err != nil {
		return fmt.Errorf("failed to query index DDL for %s.%s: %w", sourceSchema, tableName, err)
	}
	for _, idx := range indexes {
		statement := strings.TrimSpace(idx.SQL)
		if statement == "" {
			continue
		}
		if err := tx.Exec(statement).Error; err != nil {
			return fmt.Errorf("failed to create index for table %s: %w", tableName, err)
		}
	}
	return nil
}

func copyRowsIntoMainFromSource(tx *gorm.DB, sourceSchema string, tableName string, whereClause string, args ...any) (int64, error) {
	columns, err := sharedSQLiteColumns(tx, sourceSchema, tableName)
	if err != nil {
		return 0, err
	}
	if len(columns) == 0 {
		return 0, nil
	}
	columnList := joinSQLiteColumns(columns)
	sourceTable := quoteSQLiteIdentifier(sourceSchema) + "." + quoteSQLiteIdentifier(tableName)

	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s", quoteSQLiteIdentifier(tableName), columnList, columnList, sourceTable)
	countSQL := fmt.Sprintf("SELECT COUNT(1) FROM %s", sourceTable)
	trimmedWhere := strings.TrimSpace(whereClause)
	if trimmedWhere != "" {
		insertSQL += " WHERE " + trimmedWhere
		countSQL += " WHERE " + trimmedWhere
	}
	if err := tx.Exec(insertSQL, args...).Error; err != nil {
		return 0, fmt.Errorf("failed to copy rows for table %s: %w", tableName, err)
	}
	var rowCount int64
	if err := tx.Raw(countSQL, args...).Scan(&rowCount).Error; err != nil {
		return 0, fmt.Errorf("failed to count copied rows for table %s: %w", tableName, err)
	}
	return rowCount, nil
}

func copyAttachedTableIntoMain(tx *gorm.DB, sourceSchema string, tableName string, whereClause string, args ...any) error {
	existsInMain, err := sqliteTableExists(tx, "main", tableName)
	if err != nil {
		return err
	}
	if !existsInMain {
		return nil
	}
	existsInSource, err := sqliteTableExists(tx, sourceSchema, tableName)
	if err != nil {
		return err
	}
	if !existsInSource {
		return nil
	}
	columns, err := sharedSQLiteColumns(tx, sourceSchema, tableName)
	if err != nil {
		return err
	}
	if len(columns) == 0 {
		return nil
	}

	columnList := joinSQLiteColumns(columns)
	insertSQL := fmt.Sprintf(
		"INSERT OR REPLACE INTO %s (%s) SELECT %s FROM %s.%s",
		quoteSQLiteIdentifier(tableName),
		columnList,
		columnList,
		quoteSQLiteIdentifier(sourceSchema),
		quoteSQLiteIdentifier(tableName),
	)
	if trimmed := strings.TrimSpace(whereClause); trimmed != "" {
		insertSQL += " WHERE " + trimmed
	}
	if err := tx.Exec(insertSQL, args...).Error; err != nil {
		return fmt.Errorf("failed to import table %s from package: %w", tableName, err)
	}
	return nil
}

func foreignKeyViolationCount(tx *gorm.DB) (int, error) {
	type fkRow struct {
		Table string `gorm:"column:table"`
	}
	var rows []fkRow
	if err := tx.Raw("PRAGMA foreign_key_check").Scan(&rows).Error; err != nil {
		return 0, fmt.Errorf("failed to execute foreign_key_check: %w", err)
	}
	return len(rows), nil
}

func sqlPlaceholders(count int) string {
	if count <= 0 {
		return ""
	}
	parts := make([]string, 0, count)
	for i := 0; i < count; i++ {
		parts = append(parts, "?")
	}
	return strings.Join(parts, ", ")
}

func uintArgs(items []uint) []any {
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	return result
}

func computeFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
