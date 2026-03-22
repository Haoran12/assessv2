CREATE TABLE assessment_object_module_scores (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    assessment_id INTEGER NOT NULL,
    period_code VARCHAR(20) NOT NULL,
    object_id INTEGER NOT NULL,
    module_key VARCHAR(120) NOT NULL,
    score DECIMAL(10, 6) NOT NULL DEFAULT 0,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (assessment_id) REFERENCES assessment_sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (object_id) REFERENCES assessment_session_objects(id) ON DELETE CASCADE,
    UNIQUE (assessment_id, period_code, object_id, module_key)
);

CREATE INDEX idx_assessment_object_module_scores_assessment_period
    ON assessment_object_module_scores(assessment_id, period_code);
CREATE INDEX idx_assessment_object_module_scores_object_id
    ON assessment_object_module_scores(object_id);
CREATE INDEX idx_assessment_object_module_scores_module_key
    ON assessment_object_module_scores(module_key);
