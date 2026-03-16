package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path"
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

func bootstrapBackend() (config.Config, *sql.DB, *gin.Engine, error) {
	if err := prepareDesktopEnv(); err != nil {
		return config.Config{}, nil, nil, err
	}

	cfg := config.Load()
	db, err := database.NewSQLite(cfg.Database)
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("initialize sqlite: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("get sql db handle: %w", err)
	}

	migrationManager, err := migration.NewManager(db, cfg.MigrationsDir)
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("initialize migration manager: %w", err)
	}
	if _, err := migrationManager.Up(context.Background()); err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("apply migrations: %w", err)
	}

	if err := database.SeedBaselineData(db, cfg.DefaultPassword); err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("seed baseline data: %w", err)
	}

	engine := router.New(cfg, db)
	return cfg, sqlDB, engine, nil
}

func prepareDesktopEnv() error {
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

	if os.Getenv("ASSESS_MIGRATIONS_DIR") == "" {
		migrationsDir, err := ensureEmbeddedMigrationsDir()
		if err != nil {
			// Development fallback when embedded runtime assets are unavailable.
			migrationsDir, err = resolveMigrationsDir()
			if err != nil {
				return err
			}
		}
		if err := os.Setenv("ASSESS_MIGRATIONS_DIR", migrationsDir); err != nil {
			return err
		}
	}

	return nil
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
		dbPath := filepath.Join(dataRoot, entry.Name(), "assess.db")
		if _, err := os.Stat(dbPath); err != nil {
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

	entries, err := fs.ReadDir(embeddedRuntimeAssets, "runtime/migrations")
	if err != nil {
		return "", fmt.Errorf("read embedded migrations: %w", err)
	}

	sqlCount := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		content, err := fs.ReadFile(embeddedRuntimeAssets, path.Join("runtime/migrations", entry.Name()))
		if err != nil {
			return "", fmt.Errorf("read embedded migration %s: %w", entry.Name(), err)
		}

		targetFile := filepath.Join(targetDir, entry.Name())
		if err := os.WriteFile(targetFile, content, 0o644); err != nil {
			return "", fmt.Errorf("write runtime migration %s: %w", entry.Name(), err)
		}
		sqlCount++
	}

	if sqlCount == 0 {
		return "", fmt.Errorf("no embedded migration sql files found")
	}

	return targetDir, nil
}

func resolveMigrationsDir() (string, error) {
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
		if ok {
			return resolved, nil
		}
	}

	return "", fmt.Errorf("unable to locate migrations directory")
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
