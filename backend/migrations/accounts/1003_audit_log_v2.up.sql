PRAGMA foreign_keys = OFF;

CREATE TABLE audit_logs_v2 (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    action_type VARCHAR(50) NOT NULL,
    target_type VARCHAR(50),
    target_id INTEGER,
    event_code VARCHAR(100),
    summary VARCHAR(300),
    change_count INTEGER NOT NULL DEFAULT 0,
    has_diff BOOLEAN NOT NULL DEFAULT 0,
    action_detail TEXT,
    ip_address VARCHAR(50),
    user_agent TEXT,
    created_at INTEGER NOT NULL
);

INSERT INTO audit_logs_v2 (
    id,
    user_id,
    action_type,
    target_type,
    target_id,
    event_code,
    summary,
    change_count,
    has_diff,
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
    LOWER(COALESCE(action_type, '')) || '.' || LOWER(COALESCE(target_type, '')) AS event_code,
    LOWER(COALESCE(action_type, '')) || '.' || LOWER(COALESCE(target_type, '')) AS summary,
    0 AS change_count,
    0 AS has_diff,
    action_detail,
    ip_address,
    user_agent,
    created_at
FROM audit_logs;

DROP TABLE audit_logs;
ALTER TABLE audit_logs_v2 RENAME TO audit_logs;

CREATE INDEX IF NOT EXISTS idx_audit_logs_action_type ON audit_logs(action_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_target_id ON audit_logs(target_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_target_type ON audit_logs(target_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_event_code ON audit_logs(event_code);
CREATE INDEX IF NOT EXISTS idx_audit_logs_has_diff ON audit_logs(has_diff);

PRAGMA foreign_keys = ON;
