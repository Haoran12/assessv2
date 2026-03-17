package main

import (
	"context"
	"database/sql"
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
)

const preferredDataYearFileName = ".assessment_year"

func bootstrapBackend() (config.Config, []*sql.DB, *gin.Engine, error) {
	if err := prepareDesktopEnv(); err != nil {
		return config.Config{}, nil, nil, err
	}

	cfg := config.Load()
	yearDB, err := database.NewSQLite(cfg.Database)
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("initialize assessment sqlite: %w", err)
	}

	accountsPath, err := defaultAccountsSQLitePath()
	if err != nil {
		return config.Config{}, nil, nil, err
	}
	accountDBConfig := cfg.Database
	accountDBConfig.Path = accountsPath
	accountDB, err := database.NewSQLite(accountDBConfig)
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

	migrationManager, err := migration.NewManager(yearDB, cfg.BusinessMigrationsDir)
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("initialize assessment migration manager: %w", err)
	}
	if _, err := migrationManager.Up(context.Background()); err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("apply assessment migrations: %w", err)
	}

	accountMigrationManager, err := migration.NewManager(accountDB, cfg.AccountsMigrationsDir)
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("initialize accounts migration manager: %w", err)
	}
	if _, err := accountMigrationManager.Up(context.Background()); err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("apply accounts migrations: %w", err)
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
	if err := ensureDesktopDataLayoutCompatibility(exeDir, dataRoot); err != nil {
		return err
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
		migrationsRoot, err := ensureEmbeddedMigrationsDir()
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
	legacyMain := filepath.Join(dataRoot, "assess.db")
	if !fileExists(legacyMain) {
		return nil
	}

	targetYear := resolvePreferredDataYear(exeDir)
	targetDir := filepath.Join(dataRoot, strconv.Itoa(targetYear))
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create assessment year dir for legacy migration: %w", err)
	}
	targetMain := filepath.Join(targetDir, "assess.db")

	if fileExists(targetMain) {
		backupDir := filepath.Join(dataRoot, "legacy", time.Now().Format("20060102150405"))
		if err := os.MkdirAll(backupDir, 0o755); err != nil {
			return fmt.Errorf("create legacy backup dir: %w", err)
		}
		backupMain := filepath.Join(backupDir, "assess.db")
		if err := moveSQLiteWithSidecars(legacyMain, backupMain); err != nil {
			return fmt.Errorf("backup legacy flat assessment db: %w", err)
		}
		return nil
	}

	if err := moveSQLiteWithSidecars(legacyMain, targetMain); err != nil {
		return fmt.Errorf("migrate legacy flat assessment db: %w", err)
	}
	if err := persistPreferredDataYear(targetYear); err != nil {
		return fmt.Errorf("persist preferred year after legacy migration: %w", err)
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

	legacyFlatAssessmentMain := filepath.Join(dataRoot, "assess.db")
	if fileExists(legacyFlatAssessmentMain) {
		if err := copySQLiteWithSidecars(legacyFlatAssessmentMain, accountsMain); err != nil {
			return fmt.Errorf("bootstrap accounts db from legacy flat assessment db: %w", err)
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

	yearDir := strconv.Itoa(resolvePreferredDataYear(exeDir))
	dataDir := filepath.Join(exeDir, "data", yearDir)
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

func persistPreferredDataYear(year int) error {
	if year < 2000 || year > 3000 {
		return fmt.Errorf("invalid assessment year: %d", year)
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)

	dataRoot := filepath.Join(exeDir, "data")
	if err := os.MkdirAll(dataRoot, 0o755); err != nil {
		return fmt.Errorf("create data root: %w", err)
	}

	yearDir := filepath.Join(dataRoot, strconv.Itoa(year))
	if err := os.MkdirAll(yearDir, 0o755); err != nil {
		return fmt.Errorf("create assessment year data dir: %w", err)
	}

	if err := os.WriteFile(preferredDataYearFilePath(exeDir), []byte(strconv.Itoa(year)), 0o644); err != nil {
		return fmt.Errorf("write preferred assessment year: %w", err)
	}
	return nil
}

func ensureEmbeddedMigrationsDir() (string, error) {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}

	targetDir := filepath.Join(configRoot, "AssessV2", "runtime", "migrations")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("create migration runtime dir: %w", err)
	}

	sqlCount := 0
	err = fs.WalkDir(embeddedRuntimeAssets, "runtime/migrations", func(assetPath string, d fs.DirEntry, walkErr error) error {
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
