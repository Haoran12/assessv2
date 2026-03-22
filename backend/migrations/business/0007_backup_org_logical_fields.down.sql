PRAGMA foreign_keys = OFF;

CREATE TABLE backups_legacy (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    backup_name VARCHAR(200) NOT NULL,
    backup_path VARCHAR(500) NOT NULL,
    backup_type VARCHAR(20) NOT NULL,
    file_size INTEGER,
    description TEXT,
    created_by INTEGER,
    created_at INTEGER NOT NULL
);

INSERT INTO backups_legacy (
    id,
    backup_name,
    backup_path,
    backup_type,
    file_size,
    description,
    created_by,
    created_at
)
SELECT
    id,
    backup_name,
    backup_path,
    backup_type,
    file_size,
    description,
    created_by,
    created_at
FROM backups;

DROP TABLE backups;
ALTER TABLE backups_legacy RENAME TO backups;

CREATE INDEX idx_backups_backup_type ON backups(backup_type);
CREATE INDEX idx_backups_created_at ON backups(created_at DESC);
CREATE INDEX idx_backups_created_by ON backups(created_by);

PRAGMA foreign_keys = ON;
