package database

import (
	"fmt"
	"os"
	"path/filepath"

	"assessv2/backend/internal/config"
	"gorm.io/driver/sqlite"
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

	return db, nil
}
