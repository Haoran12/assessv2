package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"assessv2/backend/internal/api/router"
	"assessv2/backend/internal/config"
	"assessv2/backend/internal/database"
	"assessv2/backend/internal/migration"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const preferredDataYearFileName = ".assessment_year"

func detectDesktopFirstUse() (bool, error) {
	exePath, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("resolve executable path: %w", err)
	}
	dataRoot := filepath.Join(filepath.Dir(exePath), "data")
	return isDesktopDataRootFresh(dataRoot)
}

func isDesktopDataRootFresh(dataRoot string) (bool, error) {
	info, err := os.Stat(dataRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, fmt.Errorf("check data root status: %w", err)
	}
	if !info.IsDir() {
		return false, nil
	}

	hasExistingData, err := hasExistingDesktopData(dataRoot)
	if err != nil {
		return false, err
	}
	return !hasExistingData, nil
}

func hasExistingDesktopData(dataRoot string) (bool, error) {
	if fileExists(filepath.Join(dataRoot, "accounts", "accounts.db")) {
		return true, nil
	}
	if fileExists(filepath.Join(dataRoot, "accounts.db")) {
		return true, nil
	}
	if fileExists(filepath.Join(dataRoot, "assess.db")) {
		return true, nil
	}

	entries, err := os.ReadDir(dataRoot)
	if err != nil {
		return false, fmt.Errorf("read data root: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, ok := parseAssessmentYear(entry.Name()); !ok {
			continue
		}
		if fileExists(filepath.Join(dataRoot, entry.Name(), "assess.db")) {
			return true, nil
		}
	}

	return false, nil
}

func bootstrapBackend() (config.Config, []*sql.DB, *gin.Engine, error) {
	if err := prepareDesktopEnv(); err != nil {
		return config.Config{}, nil, nil, err
	}

	cfg := config.Load()
	yearDB, err := openSQLiteAndApplyMigrationsWithHardReset(
		context.Background(),
		cfg.Database,
		cfg.BusinessMigrationsDir,
		"assessment",
	)
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("initialize assessment sqlite: %w", err)
	}

	accountDBConfig := cfg.Database
	accountDBConfig.Path = cfg.AccountsDatabasePath
	accountDB, err := openSQLiteAndApplyMigrationsWithHardReset(
		context.Background(),
		accountDBConfig,
		cfg.AccountsMigrationsDir,
		"accounts",
	)
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("initialize accounts sqlite: %w", err)
	}

	yearSQLDB, err := yearDB.DB()
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("get assessment sql db handle: %w", err)
	}
	accountSQLDB, err := accountDB.DB()
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("get accounts sql db handle: %w", err)
	}

	if err := database.SeedAssessmentData(yearDB); err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("seed assessment baseline data: %w", err)
	}
	if err := database.SeedAccountsData(accountDB, cfg.DefaultPassword); err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("seed account baseline data: %w", err)
	}

	engine := router.NewWithDatabases(cfg, yearDB, accountDB)
	return cfg, []*sql.DB{yearSQLDB, accountSQLDB}, engine, nil
}

func openSQLiteAndApplyMigrationsWithHardReset(
	ctx context.Context,
	dbConfig config.DatabaseConfig,
	migrationsDir string,
	databaseName string,
) (*gorm.DB, error) {
	db, err := database.NewSQLite(dbConfig)
	if err != nil {
		return nil, err
	}

	applyErr := applyMigrations(ctx, db, migrationsDir)
	if applyErr == nil {
		return db, nil
	}

	var checksumErr *migration.ChecksumMismatchError
	if !errors.As(applyErr, &checksumErr) {
		return nil, applyErr
	}

	if err := closeSQLiteDB(db); err != nil {
		return nil, fmt.Errorf("close %s database before reset: %w", databaseName, err)
	}

	backupPath, err := backupAndResetIncompatibleSQLite(dbConfig.Path, databaseName)
	if err != nil {
		return nil, err
	}

	reopenedDB, err := database.NewSQLite(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("re-open %s database after reset: %w", databaseName, err)
	}
	if err := applyMigrations(ctx, reopenedDB, migrationsDir); err != nil {
		return nil, fmt.Errorf("apply %s migrations after reset: %w", databaseName, err)
	}

	fmt.Printf(
		"[desktop] detected incompatible %s migration history (version=%d file=%s), backed up old DB to %s and rebuilt schema\n",
		databaseName,
		checksumErr.Version,
		checksumErr.File,
		backupPath,
	)

	return reopenedDB, nil
}

func applyMigrations(ctx context.Context, db *gorm.DB, migrationsDir string) error {
	manager, err := migration.NewManager(db, migrationsDir)
	if err != nil {
		return err
	}
	_, err = manager.Up(ctx)
	return err
}

func closeSQLiteDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func backupAndResetIncompatibleSQLite(dbPath, databaseName string) (string, error) {
	if strings.TrimSpace(dbPath) == "" {
		return "", fmt.Errorf("reset %s database failed: empty db path", databaseName)
	}
	if !fileExists(dbPath) {
		return "", fmt.Errorf("reset %s database failed: db file not found: %s", databaseName, dbPath)
	}

	backupRoot := filepath.Join(filepath.Dir(dbPath), "incompatible", time.Now().Format("20060102150405"))
	backupMain := filepath.Join(backupRoot, fmt.Sprintf("%s.db", databaseName))
	if err := moveSQLiteWithSidecars(dbPath, backupMain); err != nil {
		return "", fmt.Errorf("backup incompatible %s database: %w", databaseName, err)
	}
	return backupMain, nil
}

func prepareDesktopEnv() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)
	dataRoot := filepath.Join(exeDir, "data")
	if err := os.MkdirAll(dataRoot, 0o755); err != nil {
		return fmt.Errorf("create data root: %w", err)
	}

	if os.Getenv("ASSESS_DATA_ROOT") == "" {
		if err := os.Setenv("ASSESS_DATA_ROOT", dataRoot); err != nil {
			return err
		}
	}

	if os.Getenv("ASSESS_SERVER_HOST") == "" {
		if err := os.Setenv("ASSESS_SERVER_HOST", "127.0.0.1"); err != nil {
			return err
		}
	}
	if os.Getenv("ASSESS_SERVER_PORT") == "" {
		if err := os.Setenv("ASSESS_SERVER_PORT", "8080"); err != nil {
			return err
		}
	}

	if os.Getenv("ASSESS_SQLITE_PATH") == "" {
		sqlitePath, err := defaultSQLitePath()
		if err != nil {
			return err
		}
		if err := os.Setenv("ASSESS_SQLITE_PATH", sqlitePath); err != nil {
			return err
		}
	}

	if os.Getenv("ASSESS_BUSINESS_MIGRATIONS_DIR") == "" || os.Getenv("ASSESS_ACCOUNTS_MIGRATIONS_DIR") == "" {
		migrationsRoot, err := ensureEmbeddedMigrationsDir(dataRoot)
		if err != nil {
			// Development fallback when embedded runtime assets are unavailable.
			migrationsRoot, err = resolveMigrationsRoot()
			if err != nil {
				return err
			}
		}
		if os.Getenv("ASSESS_MIGRATIONS_DIR") == "" {
			if err := os.Setenv("ASSESS_MIGRATIONS_DIR", migrationsRoot); err != nil {
				return err
			}
		}
		if os.Getenv("ASSESS_BUSINESS_MIGRATIONS_DIR") == "" {
			if err := os.Setenv("ASSESS_BUSINESS_MIGRATIONS_DIR", filepath.Join(migrationsRoot, "business")); err != nil {
				return err
			}
		}
		if os.Getenv("ASSESS_ACCOUNTS_MIGRATIONS_DIR") == "" {
			if err := os.Setenv("ASSESS_ACCOUNTS_MIGRATIONS_DIR", filepath.Join(migrationsRoot, "accounts")); err != nil {
				return err
			}
		}
	}

	if os.Getenv("ASSESS_ACCOUNTS_SQLITE_PATH") == "" {
		accountsPath, err := defaultAccountsSQLitePath()
		if err != nil {
			return err
		}
		if err := os.Setenv("ASSESS_ACCOUNTS_SQLITE_PATH", accountsPath); err != nil {
			return err
		}
	}

	return nil
}

func ensureDesktopDataLayoutCompatibility(exeDir, dataRoot string) error {
	if err := migrateLegacyFlatAssessmentDB(exeDir, dataRoot); err != nil {
		return err
	}
	if err := migrateLegacyAccountsDB(exeDir, dataRoot); err != nil {
		return err
	}
	return nil
}

func migrateLegacyFlatAssessmentDB(exeDir, dataRoot string) error {
	flatMain := filepath.Join(dataRoot, "assess.db")
	if fileExists(flatMain) {
		return nil
	}

	preferredYear := resolvePreferredDataYear(exeDir)
	preferredYearMain := filepath.Join(dataRoot, strconv.Itoa(preferredYear), "assess.db")
	if fileExists(preferredYearMain) {
		if err := moveSQLiteWithSidecars(preferredYearMain, flatMain); err != nil {
			return fmt.Errorf("migrate preferred yearly assessment db to flat layout: %w", err)
		}
		return nil
	}

	if latestYear, ok := detectLatestDataYearDir(exeDir); ok {
		latestYearMain := filepath.Join(dataRoot, strconv.Itoa(latestYear), "assess.db")
		if fileExists(latestYearMain) {
			if err := moveSQLiteWithSidecars(latestYearMain, flatMain); err != nil {
				return fmt.Errorf("migrate latest yearly assessment db to flat layout: %w", err)
			}
			return nil
		}
	}

	return nil
}

func migrateLegacyAccountsDB(exeDir, dataRoot string) error {
	accountsMain := filepath.Join(dataRoot, "accounts", "accounts.db")
	if fileExists(accountsMain) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(accountsMain), 0o755); err != nil {
		return fmt.Errorf("create accounts dir for legacy migration: %w", err)
	}

	legacyAccountsMain := filepath.Join(dataRoot, "accounts.db")
	if fileExists(legacyAccountsMain) {
		if err := moveSQLiteWithSidecars(legacyAccountsMain, accountsMain); err != nil {
			return fmt.Errorf("migrate legacy flat accounts db: %w", err)
		}
		return nil
	}

	flatAssessmentMain := filepath.Join(dataRoot, "assess.db")
	if fileExists(flatAssessmentMain) {
		if err := copySQLiteWithSidecars(flatAssessmentMain, accountsMain); err != nil {
			return fmt.Errorf("bootstrap accounts db from flat assessment db: %w", err)
		}
		return nil
	}

	preferredYear := resolvePreferredDataYear(exeDir)
	preferredYearMain := filepath.Join(dataRoot, strconv.Itoa(preferredYear), "assess.db")
	if fileExists(preferredYearMain) {
		if err := copySQLiteWithSidecars(preferredYearMain, accountsMain); err != nil {
			return fmt.Errorf("bootstrap accounts db from preferred year db: %w", err)
		}
		return nil
	}

	if latestYear, ok := detectLatestDataYearDir(exeDir); ok {
		latestYearMain := filepath.Join(dataRoot, strconv.Itoa(latestYear), "assess.db")
		if fileExists(latestYearMain) {
			if err := copySQLiteWithSidecars(latestYearMain, accountsMain); err != nil {
				return fmt.Errorf("bootstrap accounts db from latest year db: %w", err)
			}
			return nil
		}
	}

	return nil
}

func moveSQLiteWithSidecars(srcMain, dstMain string) error {
	if err := os.MkdirAll(filepath.Dir(dstMain), 0o755); err != nil {
		return err
	}

	if err := moveFile(srcMain, dstMain); err != nil {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		src := srcMain + suffix
		if !fileExists(src) {
			continue
		}
		if err := moveFile(src, dstMain+suffix); err != nil {
			return err
		}
	}
	return nil
}

func copySQLiteWithSidecars(srcMain, dstMain string) error {
	if err := os.MkdirAll(filepath.Dir(dstMain), 0o755); err != nil {
		return err
	}

	if err := copyFile(srcMain, dstMain); err != nil {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		src := srcMain + suffix
		if !fileExists(src) {
			continue
		}
		if err := copyFile(src, dstMain+suffix); err != nil {
			return err
		}
	}
	return nil
}

func moveFile(srcPath, dstPath string) error {
	if err := os.Rename(srcPath, dstPath); err == nil {
		return nil
	}
	if err := copyFile(srcPath, dstPath); err != nil {
		return err
	}
	if err := os.Remove(srcPath); err != nil {
		return err
	}
	return nil
}

func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return dst.Sync()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func defaultSQLitePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)

	dataDir := filepath.Join(exeDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", fmt.Errorf("create sqlite data dir: %w", err)
	}

	return filepath.Join(dataDir, "assess.db"), nil
}

func defaultAccountsSQLitePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)

	accountsDir := filepath.Join(exeDir, "data", "accounts")
	if err := os.MkdirAll(accountsDir, 0o755); err != nil {
		return "", fmt.Errorf("create accounts data dir: %w", err)
	}
	return filepath.Join(accountsDir, "accounts.db"), nil
}

func resolvePreferredDataYear(exeDir string) int {
	if value := strings.TrimSpace(os.Getenv("ASSESS_DATA_YEAR")); value != "" {
		if parsed, ok := parseAssessmentYear(value); ok {
			return parsed
		}
	}

	if fromFile, ok := loadPreferredDataYearFromFile(exeDir); ok {
		return fromFile
	}

	if fromDataDir, ok := detectLatestDataYearDir(exeDir); ok {
		return fromDataDir
	}

	return time.Now().Year()
}

func parseAssessmentYear(text string) (int, bool) {
	year, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return 0, false
	}
	if year < 2000 || year > 3000 {
		return 0, false
	}
	return year, true
}

func preferredDataYearFilePath(exeDir string) string {
	return filepath.Join(exeDir, "data", preferredDataYearFileName)
}

func loadPreferredDataYearFromFile(exeDir string) (int, bool) {
	content, err := os.ReadFile(preferredDataYearFilePath(exeDir))
	if err != nil {
		return 0, false
	}
	return parseAssessmentYear(string(content))
}

func detectLatestDataYearDir(exeDir string) (int, bool) {
	dataRoot := filepath.Join(exeDir, "data")
	entries, err := os.ReadDir(dataRoot)
	if err != nil {
		return 0, false
	}

	years := make([]int, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		year, ok := parseAssessmentYear(entry.Name())
		if !ok {
			continue
		}
		years = append(years, year)
	}

	if len(years) == 0 {
		return 0, false
	}
	sort.Ints(years)
	return years[len(years)-1], true
}

func persistPreferredDataYear(_ int) error {
	return fmt.Errorf("preferred data year is deprecated in session-based mode")
}

func ensureEmbeddedMigrationsDir(dataRoot string) (string, error) {
	root := strings.TrimSpace(dataRoot)
	if root == "" {
		return "", fmt.Errorf("empty data root for embedded migrations")
	}
	targetDir := filepath.Join(root, "runtime", "migrations")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("create migration runtime dir: %w", err)
	}

	sqlCount := 0
	err := fs.WalkDir(embeddedRuntimeAssets, "runtime/migrations", func(assetPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			relativeDir := strings.TrimPrefix(assetPath, "runtime/migrations")
			relativeDir = strings.TrimPrefix(relativeDir, "/")
			relativeDir = strings.TrimPrefix(relativeDir, "\\")
			if relativeDir == "" {
				return nil
			}
			return os.MkdirAll(filepath.Join(targetDir, filepath.FromSlash(relativeDir)), 0o755)
		}
		if filepath.Ext(d.Name()) != ".sql" {
			return nil
		}

		content, err := fs.ReadFile(embeddedRuntimeAssets, assetPath)
		if err != nil {
			return fmt.Errorf("read embedded migration %s: %w", assetPath, err)
		}

		relativeFile := strings.TrimPrefix(assetPath, "runtime/migrations/")
		targetFile := filepath.Join(targetDir, filepath.FromSlash(relativeFile))
		if err := os.MkdirAll(filepath.Dir(targetFile), 0o755); err != nil {
			return fmt.Errorf("create runtime migration dir: %w", err)
		}
		if err := os.WriteFile(targetFile, content, 0o644); err != nil {
			return fmt.Errorf("write runtime migration %s: %w", relativeFile, err)
		}
		sqlCount++
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk embedded migrations: %w", err)
	}

	if sqlCount == 0 {
		return "", fmt.Errorf("no embedded migration sql files found")
	}

	return targetDir, nil
}

func resolveMigrationsRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working dir: %w", err)
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)

	candidates := []string{
		filepath.Join(exeDir, "migrations"),
		filepath.Join(exeDir, "..", "migrations"),
		filepath.Join(exeDir, "..", "..", "..", "migrations"),
		filepath.Join(exeDir, "..", "..", "..", "..", "backend", "migrations"),
		filepath.Join(cwd, "..", "migrations"),
		filepath.Join(cwd, "migrations"),
	}

	for _, candidate := range candidates {
		resolved, ok := existingDir(candidate)
		if ok && hasSplitMigrationDirs(resolved) {
			return resolved, nil
		}
	}

	return "", fmt.Errorf("unable to locate split migrations root directory")
}

func hasSplitMigrationDirs(root string) bool {
	_, businessExists := existingDir(filepath.Join(root, "business"))
	_, accountsExists := existingDir(filepath.Join(root, "accounts"))
	return businessExists && accountsExists
}

func existingDir(path string) (string, bool) {
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return resolved, true
}
