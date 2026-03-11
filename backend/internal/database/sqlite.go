package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"assessv2/backend/internal/config"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func NewSQLite(cfg config.DatabaseConfig) (*gorm.DB, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}

	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create sqlite directory: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(cfg.Path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sqlite sql.DB: %w", err)
	}

	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns >= 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetimeSeconds > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeSeconds) * time.Second)
	}

	if err := applySQLitePragmas(sqlDB, cfg); err != nil {
		return nil, err
	}

	return db, nil
}

func applySQLitePragmas(sqlDB *sql.DB, cfg config.DatabaseConfig) error {
	foreignKeys := 0
	if cfg.ForeignKeys {
		foreignKeys = 1
	}
	if err := execPragma(sqlDB, "foreign_keys", fmt.Sprintf("%d", foreignKeys)); err != nil {
		return err
	}

	if cfg.JournalMode != "" {
		if err := execPragma(sqlDB, "journal_mode", strings.ToUpper(cfg.JournalMode)); err != nil {
			return err
		}
	}
	if cfg.Synchronous != "" {
		if err := execPragma(sqlDB, "synchronous", strings.ToUpper(cfg.Synchronous)); err != nil {
			return err
		}
	}
	if cfg.BusyTimeoutMS > 0 {
		if err := execPragma(sqlDB, "busy_timeout", fmt.Sprintf("%d", cfg.BusyTimeoutMS)); err != nil {
			return err
		}
	}
	if cfg.CacheSize != 0 {
		if err := execPragma(sqlDB, "cache_size", fmt.Sprintf("%d", cfg.CacheSize)); err != nil {
			return err
		}
	}
	if cfg.TempStore != "" {
		if err := execPragma(sqlDB, "temp_store", strings.ToUpper(cfg.TempStore)); err != nil {
			return err
		}
	}
	return nil
}

func execPragma(sqlDB *sql.DB, key, value string) error {
	statement := fmt.Sprintf("PRAGMA %s = %s;", key, value)
	if _, err := sqlDB.Exec(statement); err != nil {
		return fmt.Errorf("failed to execute sqlite pragma %s=%s: %w", key, value, err)
	}
	return nil
}
