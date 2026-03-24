DROP INDEX IF EXISTS idx_assessment_sessions_status;

ALTER TABLE assessment_sessions
    DROP COLUMN completed_snapshot_created_at;

ALTER TABLE assessment_sessions
    DROP COLUMN completed_snapshot_path;

ALTER TABLE assessment_sessions
    DROP COLUMN status;
