package migration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"assessv2/backend/internal/config"
	"assessv2/backend/internal/database"
	"gorm.io/gorm"
)

func TestManagerUpDownStatus(t *testing.T) {
	ctx := context.Background()
	migrationsDir := t.TempDir()

	mustWriteFile(t, filepath.Join(migrationsDir, "0001_create_users.up.sql"), `
CREATE TABLE test_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL
);
`)
	mustWriteFile(t, filepath.Join(migrationsDir, "0001_create_users.down.sql"), `
DROP TABLE IF EXISTS test_users;
`)
	mustWriteFile(t, filepath.Join(migrationsDir, "0002_create_logs.up.sql"), `
CREATE TABLE test_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content TEXT NOT NULL
);
`)
	mustWriteFile(t, filepath.Join(migrationsDir, "0002_create_logs.down.sql"), `
DROP TABLE IF EXISTS test_logs;
`)

	db, cleanup := openTestDB(t)
	defer cleanup()

	manager, err := NewManager(db, migrationsDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	applied, err := manager.Up(ctx)
	if err != nil {
		t.Fatalf("failed to apply migrations: %v", err)
	}
	if applied != 2 {
		t.Fatalf("expected applied=2, got=%d", applied)
	}

	appliedAgain, err := manager.Up(ctx)
	if err != nil {
		t.Fatalf("failed to reapply migrations: %v", err)
	}
	if appliedAgain != 0 {
		t.Fatalf("expected applied again=0, got=%d", appliedAgain)
	}

	statusRows, err := manager.Status(ctx)
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if len(statusRows) != 2 {
		t.Fatalf("expected 2 status rows, got=%d", len(statusRows))
	}
	if !statusRows[0].Applied || !statusRows[1].Applied {
		t.Fatalf("expected all migrations applied, got=%+v", statusRows)
	}

	reverted, err := manager.Down(ctx, 1)
	if err != nil {
		t.Fatalf("failed to rollback migration: %v", err)
	}
	if reverted != 1 {
		t.Fatalf("expected reverted=1, got=%d", reverted)
	}

	statusRows, err = manager.Status(ctx)
	if err != nil {
		t.Fatalf("failed to get status after rollback: %v", err)
	}
	if !statusRows[0].Applied {
		t.Fatalf("expected first migration still applied")
	}
	if statusRows[1].Applied {
		t.Fatalf("expected second migration rolled back")
	}
}

func TestManagerReconcileChecksums(t *testing.T) {
	ctx := context.Background()
	migrationsDir := t.TempDir()

	upPath := filepath.Join(migrationsDir, "0001_create_users.up.sql")
	downPath := filepath.Join(migrationsDir, "0001_create_users.down.sql")

	mustWriteFile(t, upPath, `
CREATE TABLE test_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL
);
`)
	mustWriteFile(t, downPath, `
DROP TABLE IF EXISTS test_users;
`)

	db, cleanup := openTestDB(t)
	defer cleanup()

	manager, err := NewManager(db, migrationsDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	applied, err := manager.Up(ctx)
	if err != nil {
		t.Fatalf("failed to apply initial migrations: %v", err)
	}
	if applied != 1 {
		t.Fatalf("expected applied=1, got=%d", applied)
	}

	mustWriteFile(t, upPath, `
CREATE TABLE test_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    email TEXT
);
`)

	_, err = manager.Up(ctx)
	if err == nil {
		t.Fatalf("expected checksum mismatch error")
	}
	var checksumErr *ChecksumMismatchError
	if !errors.As(err, &checksumErr) {
		t.Fatalf("expected ChecksumMismatchError, got: %v", err)
	}

	reconciled, err := manager.ReconcileChecksums(ctx)
	if err != nil {
		t.Fatalf("failed to reconcile checksums: %v", err)
	}
	if reconciled != 1 {
		t.Fatalf("expected reconciled=1, got=%d", reconciled)
	}

	applied, err = manager.Up(ctx)
	if err != nil {
		t.Fatalf("expected up to pass after reconcile, got: %v", err)
	}
	if applied != 0 {
		t.Fatalf("expected applied=0 after reconcile, got=%d", applied)
	}
}

func openTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	cfg := config.DatabaseConfig{Path: filepath.Join(t.TempDir(), "migration_test.db")}
	db, err := database.NewSQLite(cfg)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql db: %v", err)
	}

	cleanup := func() {
		_ = sqlDB.Close()
	}
	return db, cleanup
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}
