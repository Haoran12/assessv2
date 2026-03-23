ALTER TABLE audit_logs
ADD COLUMN event_code VARCHAR(100);

ALTER TABLE audit_logs
ADD COLUMN summary VARCHAR(300);

ALTER TABLE audit_logs
ADD COLUMN change_count INTEGER NOT NULL DEFAULT 0;

ALTER TABLE audit_logs
ADD COLUMN has_diff BOOLEAN NOT NULL DEFAULT 0;

UPDATE audit_logs
SET event_code = LOWER(COALESCE(action_type, '')) || '.' || LOWER(COALESCE(target_type, ''))
WHERE TRIM(COALESCE(event_code, '')) = '';

UPDATE audit_logs
SET summary = event_code
WHERE TRIM(COALESCE(summary, '')) = '';

CREATE INDEX IF NOT EXISTS idx_audit_logs_event_code ON audit_logs(event_code);
CREATE INDEX IF NOT EXISTS idx_audit_logs_has_diff ON audit_logs(has_diff);
