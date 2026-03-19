CREATE TABLE IF NOT EXISTS rule_file_hides (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_file_id INTEGER NOT NULL,
    organization_id INTEGER NOT NULL,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (rule_file_id) REFERENCES rule_files(id) ON DELETE CASCADE,
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    UNIQUE (rule_file_id, organization_id)
);

CREATE TABLE IF NOT EXISTS assessment_rule_bindings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    assessment_id INTEGER NOT NULL,
    period_code VARCHAR(20) NOT NULL,
    object_group_code VARCHAR(80) NOT NULL,
    organization_id INTEGER NOT NULL,
    rule_file_id INTEGER NOT NULL,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (assessment_id) REFERENCES assessment_sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    FOREIGN KEY (rule_file_id) REFERENCES rule_files(id) ON DELETE CASCADE,
    UNIQUE (assessment_id, period_code, object_group_code, organization_id)
);

CREATE INDEX IF NOT EXISTS idx_assessment_rule_bindings_lookup
    ON assessment_rule_bindings(assessment_id, period_code, object_group_code, organization_id);
