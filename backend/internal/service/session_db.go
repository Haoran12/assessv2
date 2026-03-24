package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"assessv2/backend/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const sessionBusinessSQLiteFileName = "assess.db"

func openSessionBusinessDB(summary *AssessmentSessionSummary) (*gorm.DB, func(), error) {
	if summary == nil {
		return nil, nil, ErrInvalidParam
	}
	dataDir := resolveSessionDataDir(summary.DataDir, summary.AssessmentName)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create session data directory for sqlite: %w", err)
	}
	dbPath := filepath.Join(dataDir, sessionBusinessSQLiteFileName)
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, nil, fmt.Errorf("open session sqlite failed: %w", err)
	}
	if err := ensureSessionBusinessSchema(db); err != nil {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
		return nil, nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve session sqlite sql db failed: %w", err)
	}
	closeFn := func() {
		_ = sqlDB.Close()
	}
	return db, closeFn, nil
}

func withSessionBusinessDB(
	ctx context.Context,
	summary *AssessmentSessionSummary,
	fn func(sessionDB *gorm.DB) error,
) error {
	sessionDB, closeFn, err := openSessionBusinessDB(summary)
	if err != nil {
		return err
	}
	defer closeFn()
	return fn(sessionDB.WithContext(ctx))
}

func ensureSessionBusinessSchema(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.AssessmentSessionPeriod{},
		&model.AssessmentObjectGroup{},
		&model.AssessmentSessionObject{},
		&model.AssessmentObjectModuleScore{},
		&model.RuleFile{},
	); err != nil {
		return fmt.Errorf("automigrate session business schema failed: %w", err)
	}
	return nil
}
