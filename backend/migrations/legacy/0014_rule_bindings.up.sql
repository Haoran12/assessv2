PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS rule_bindings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    year_id INTEGER NOT NULL,
    period_code VARCHAR(20) NOT NULL,
    object_type VARCHAR(20) NOT NULL,
    segment_code VARCHAR(80) NOT NULL,
    owner_scope VARCHAR(30) NOT NULL DEFAULT 'global',
    owner_org_type VARCHAR(20),
    owner_org_id INTEGER,
    rule_id INTEGER NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (year_id) REFERENCES assessment_years(id) ON DELETE CASCADE,
    FOREIGN KEY (owner_org_id) REFERENCES organizations(id) ON DELETE SET NULL,
    FOREIGN KEY (rule_id) REFERENCES assessment_rules(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES users(id),
    FOREIGN KEY (updated_by) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_rule_bindings_lookup
    ON rule_bindings(
        year_id,
        period_code,
        object_type,
        segment_code,
        owner_scope,
        owner_org_type,
        owner_org_id,
        priority,
        is_active
    );
CREATE INDEX IF NOT EXISTS idx_rule_bindings_rule_id ON rule_bindings(rule_id);
