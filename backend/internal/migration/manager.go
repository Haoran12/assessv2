package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

const schemaMigrationsTable = "schema_migrations"

type Manager struct {
	sqlDB         *sql.DB
	migrationsDir string
}

type Status struct {
	Version   int
	Name      string
	Applied   bool
	AppliedAt int64
}

type migrationFile struct {
	Version  int
	Name     string
	UpPath   string
	DownPath string
	UpSQL    string
	DownSQL  string
	Checksum string
}

type appliedMigration struct {
	Version   int
	Name      string
	Checksum  string
	AppliedAt int64
}

func NewManager(db *gorm.DB, migrationsDir string) (*Manager, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	resolvedDir, err := resolveMigrationDir(migrationsDir)
	if err != nil {
		return nil, err
	}

	manager := &Manager{sqlDB: sqlDB, migrationsDir: resolvedDir}
	if err := manager.ensureSchemaMigrationsTable(context.Background()); err != nil {
		return nil, err
	}
	return manager, nil
}

func (m *Manager) Up(ctx context.Context) (int, error) {
	if err := m.ensureSchemaMigrationsTable(ctx); err != nil {
		return 0, err
	}

	migrations, err := m.listMigrationFiles()
	if err != nil {
		return 0, err
	}

	applied, err := m.loadAppliedMigrations(ctx)
	if err != nil {
		return 0, err
	}

	appliedCount := 0
	for _, item := range migrations {
		recorded, exists := applied[item.Version]
		if exists {
			if recorded.Checksum != item.Checksum {
				return 0, fmt.Errorf("migration checksum mismatch version=%d file=%s", item.Version, filepath.Base(item.UpPath))
			}
			continue
		}

		if err := m.applyMigration(ctx, item); err != nil {
			return 0, err
		}
		appliedCount++
	}

	return appliedCount, nil
}

func (m *Manager) Down(ctx context.Context, steps int) (int, error) {
	if steps <= 0 {
		return 0, fmt.Errorf("steps must be greater than 0")
	}

	if err := m.ensureSchemaMigrationsTable(ctx); err != nil {
		return 0, err
	}

	migrations, err := m.listMigrationFiles()
	if err != nil {
		return 0, err
	}
	migrationByVersion := make(map[int]migrationFile, len(migrations))
	for _, item := range migrations {
		migrationByVersion[item.Version] = item
	}

	rows, err := m.sqlDB.QueryContext(
		ctx,
		"SELECT version FROM schema_migrations ORDER BY version DESC LIMIT ?",
		steps,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	versions := make([]int, 0, steps)
	for rows.Next() {
		var version int
		if scanErr := rows.Scan(&version); scanErr != nil {
			return 0, fmt.Errorf("failed to scan migration version: %w", scanErr)
		}
		versions = append(versions, version)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return 0, fmt.Errorf("failed to iterate applied migrations: %w", rowsErr)
	}

	revertedCount := 0
	for _, version := range versions {
		item, exists := migrationByVersion[version]
		if !exists {
			return revertedCount, fmt.Errorf("missing migration file for version=%d", version)
		}
		if strings.TrimSpace(item.DownSQL) == "" {
			return revertedCount, fmt.Errorf("missing down migration SQL for version=%d", version)
		}

		tx, err := m.sqlDB.BeginTx(ctx, nil)
		if err != nil {
			return revertedCount, fmt.Errorf("failed to begin migration transaction version=%d: %w", version, err)
		}

		if err := execSQLScript(ctx, tx, item.DownSQL); err != nil {
			_ = tx.Rollback()
			return revertedCount, fmt.Errorf("failed to execute down migration version=%d: %w", version, err)
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version = ?", version); err != nil {
			_ = tx.Rollback()
			return revertedCount, fmt.Errorf("failed to delete migration version=%d: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return revertedCount, fmt.Errorf("failed to commit down migration version=%d: %w", version, err)
		}
		revertedCount++
	}

	return revertedCount, nil
}

func (m *Manager) Status(ctx context.Context) ([]Status, error) {
	if err := m.ensureSchemaMigrationsTable(ctx); err != nil {
		return nil, err
	}

	migrations, err := m.listMigrationFiles()
	if err != nil {
		return nil, err
	}

	applied, err := m.loadAppliedMigrations(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]Status, 0, len(migrations))
	for _, item := range migrations {
		status := Status{Version: item.Version, Name: item.Name}
		if recorded, exists := applied[item.Version]; exists {
			status.Applied = true
			status.AppliedAt = recorded.AppliedAt
		}
		result = append(result, status)
	}

	return result, nil
}

func (m *Manager) applyMigration(ctx context.Context, migration migrationFile) error {
	tx, err := m.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin migration transaction version=%d: %w", migration.Version, err)
	}

	if err := execSQLScript(ctx, tx, migration.UpSQL); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to execute up migration version=%d: %w", migration.Version, err)
	}

	if _, err := tx.ExecContext(
		ctx,
		"INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, ?)",
		migration.Version,
		migration.Name,
		migration.Checksum,
		time.Now().Unix(),
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to record migration version=%d: %w", migration.Version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration version=%d: %w", migration.Version, err)
	}
	return nil
}

func (m *Manager) ensureSchemaMigrationsTable(ctx context.Context) error {
	statement := `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    checksum TEXT NOT NULL,
    applied_at INTEGER NOT NULL
);
`
	if _, err := m.sqlDB.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("failed to ensure schema_migrations table: %w", err)
	}
	return nil
}

func (m *Manager) loadAppliedMigrations(ctx context.Context) (map[int]appliedMigration, error) {
	rows, err := m.sqlDB.QueryContext(
		ctx,
		"SELECT version, name, checksum, applied_at FROM schema_migrations ORDER BY version ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query schema_migrations: %w", err)
	}
	defer rows.Close()

	result := make(map[int]appliedMigration)
	for rows.Next() {
		var item appliedMigration
		if scanErr := rows.Scan(&item.Version, &item.Name, &item.Checksum, &item.AppliedAt); scanErr != nil {
			return nil, fmt.Errorf("failed to scan schema_migrations row: %w", scanErr)
		}
		result[item.Version] = item
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("failed to iterate schema_migrations rows: %w", rowsErr)
	}
	return result, nil
}

func (m *Manager) listMigrationFiles() ([]migrationFile, error) {
	entries, err := os.ReadDir(m.migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations dir %s: %w", m.migrationsDir, err)
	}

	type draft struct {
		Version  int
		Name     string
		UpPath   string
		DownPath string
	}
	buffer := map[int]*draft{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		var base string
		isUp := false
		switch {
		case strings.HasSuffix(fileName, ".up.sql"):
			base = strings.TrimSuffix(fileName, ".up.sql")
			isUp = true
		case strings.HasSuffix(fileName, ".down.sql"):
			base = strings.TrimSuffix(fileName, ".down.sql")
		default:
			continue
		}

		version, name, err := parseMigrationBaseName(base)
		if err != nil {
			return nil, fmt.Errorf("invalid migration file name %s: %w", fileName, err)
		}

		item, exists := buffer[version]
		if !exists {
			item = &draft{Version: version, Name: name}
			buffer[version] = item
		}
		if item.Name != name {
			return nil, fmt.Errorf("migration version %d has inconsistent names", version)
		}

		fullPath := filepath.Join(m.migrationsDir, fileName)
		if isUp {
			item.UpPath = fullPath
		} else {
			item.DownPath = fullPath
		}
	}

	versions := make([]int, 0, len(buffer))
	for version := range buffer {
		versions = append(versions, version)
	}
	sort.Ints(versions)

	result := make([]migrationFile, 0, len(versions))
	for _, version := range versions {
		item := buffer[version]
		if item.UpPath == "" {
			return nil, fmt.Errorf("missing up migration file for version=%d", version)
		}

		upSQLBytes, err := os.ReadFile(item.UpPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read up migration file %s: %w", item.UpPath, err)
		}
		upSQL := string(upSQLBytes)

		downSQL := ""
		if item.DownPath != "" {
			downSQLBytes, err := os.ReadFile(item.DownPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read down migration file %s: %w", item.DownPath, err)
			}
			downSQL = string(downSQLBytes)
		}

		hash := sha256.Sum256(upSQLBytes)
		result = append(result, migrationFile{
			Version:  item.Version,
			Name:     item.Name,
			UpPath:   item.UpPath,
			DownPath: item.DownPath,
			UpSQL:    upSQL,
			DownSQL:  downSQL,
			Checksum: hex.EncodeToString(hash[:]),
		})
	}

	return result, nil
}

func parseMigrationBaseName(base string) (int, string, error) {
	index := strings.IndexByte(base, '_')
	if index <= 0 || index >= len(base)-1 {
		return 0, "", fmt.Errorf("expected pattern <version>_<name>")
	}

	version, err := strconv.Atoi(base[:index])
	if err != nil || version <= 0 {
		return 0, "", fmt.Errorf("invalid version")
	}

	name := strings.TrimSpace(base[index+1:])
	if name == "" {
		return 0, "", fmt.Errorf("missing migration name")
	}
	return version, name, nil
}

func execSQLScript(ctx context.Context, tx *sql.Tx, script string) error {
	statements := strings.Split(script, ";")
	for _, statement := range statements {
		trimmed := strings.TrimSpace(statement)
		if trimmed == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, trimmed); err != nil {
			return err
		}
	}
	return nil
}

func resolveMigrationDir(input string) (string, error) {
	dir := strings.TrimSpace(input)
	if dir == "" {
		dir = "migrations"
	}

	if filepath.IsAbs(dir) {
		info, err := os.Stat(dir)
		if err != nil {
			return "", fmt.Errorf("failed to stat migrations dir %s: %w", dir, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("migrations path is not a directory: %s", dir)
		}
		return dir, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working dir: %w", err)
	}

	current := cwd
	for {
		candidate := filepath.Join(current, dir)
		info, statErr := os.Stat(candidate)
		if statErr == nil && info.IsDir() {
			return candidate, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", fmt.Errorf("migration directory %q not found from %s upward", dir, cwd)
}
