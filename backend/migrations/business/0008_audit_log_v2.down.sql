DROP INDEX IF EXISTS idx_audit_logs_has_diff;
DROP INDEX IF EXISTS idx_audit_logs_event_code;

ALTER TABLE audit_logs DROP COLUMN has_diff;
ALTER TABLE audit_logs DROP COLUMN change_count;
ALTER TABLE audit_logs DROP COLUMN summary;
ALTER TABLE audit_logs DROP COLUMN event_code;
