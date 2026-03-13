package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"assessv2/backend/internal/api/router"
	"assessv2/backend/internal/config"
	"assessv2/backend/internal/database"
	"assessv2/backend/internal/migration"
	"github.com/gin-gonic/gin"
)

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
		configRoot, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("resolve user config dir: %w", err)
		}

		dataDir := filepath.Join(configRoot, "AssessV2", "data")
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			return fmt.Errorf("create data dir: %w", err)
		}

		if err := os.Setenv("ASSESS_SQLITE_PATH", filepath.Join(dataDir, "assess.db")); err != nil {
			return err
		}
	}

	if os.Getenv("ASSESS_MIGRATIONS_DIR") == "" {
		migrationsDir, err := resolveMigrationsDir()
		if err != nil {
			return err
		}
		if err := os.Setenv("ASSESS_MIGRATIONS_DIR", migrationsDir); err != nil {
			return err
		}
	}

	return nil
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
