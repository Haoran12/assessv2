ALTER TABLE assessment_sessions
    ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'preparing';

ALTER TABLE assessment_sessions
    ADD COLUMN completed_snapshot_path VARCHAR(500);

ALTER TABLE assessment_sessions
    ADD COLUMN completed_snapshot_created_at INTEGER;

UPDATE assessment_sessions
SET status = 'active'
WHERE status IS NULL OR TRIM(status) = '';

CREATE INDEX IF NOT EXISTS idx_assessment_sessions_status
    ON assessment_sessions(status);
