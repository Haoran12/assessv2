PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS backups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    backup_name VARCHAR(200) NOT NULL,
    backup_path VARCHAR(500) NOT NULL,
    backup_type VARCHAR(20) NOT NULL,
    file_size INTEGER,
    description TEXT,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_backups_backup_type ON backups(backup_type);
CREATE INDEX IF NOT EXISTS idx_backups_created_at ON backups(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_backups_created_by ON backups(created_by);
