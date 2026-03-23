PRAGMA foreign_keys = OFF;

CREATE TABLE audit_logs_legacy (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    action_type VARCHAR(50) NOT NULL,
    target_type VARCHAR(50),
    target_id INTEGER,
    action_detail TEXT,
    ip_address VARCHAR(50),
    user_agent TEXT,
    created_at INTEGER NOT NULL
);

INSERT INTO audit_logs_legacy (
    id,
    user_id,
    action_type,
    target_type,
    target_id,
    action_detail,
    ip_address,
    user_agent,
    created_at
)
SELECT
    id,
    user_id,
    action_type,
    target_type,
    target_id,
    action_detail,
    ip_address,
    user_agent,
    created_at
FROM audit_logs;

DROP TABLE audit_logs;
ALTER TABLE audit_logs_legacy RENAME TO audit_logs;

CREATE INDEX IF NOT EXISTS idx_audit_logs_action_type ON audit_logs(action_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_target_id ON audit_logs(target_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_target_type ON audit_logs(target_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);

PRAGMA foreign_keys = ON;
