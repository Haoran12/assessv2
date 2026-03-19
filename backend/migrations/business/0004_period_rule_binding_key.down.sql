PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS idx_assessment_session_periods_rule_binding_key;

CREATE TABLE assessment_session_periods_rollback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    assessment_id INTEGER NOT NULL,
    period_code VARCHAR(20) NOT NULL,
    period_name VARCHAR(100) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (assessment_id) REFERENCES assessment_sessions(id) ON DELETE CASCADE,
    UNIQUE (assessment_id, period_code)
);

INSERT INTO assessment_session_periods_rollback (
    id,
    assessment_id,
    period_code,
    period_name,
    sort_order,
    created_by,
    created_at,
    updated_by,
    updated_at
)
SELECT
    id,
    assessment_id,
    period_code,
    period_name,
    sort_order,
    created_by,
    created_at,
    updated_by,
    updated_at
FROM assessment_session_periods;

DROP TABLE assessment_session_periods;
ALTER TABLE assessment_session_periods_rollback RENAME TO assessment_session_periods;

CREATE INDEX idx_assessment_session_periods_assessment_id
    ON assessment_session_periods(assessment_id);

PRAGMA foreign_keys = ON;
