PRAGMA foreign_keys = OFF;

DROP TABLE IF EXISTS assessment_object_user_links;
DROP TABLE IF EXISTS rankings;
DROP TABLE IF EXISTS calculated_module_scores;
DROP TABLE IF EXISTS calculated_scores;
DROP TABLE IF EXISTS extra_points;
DROP TABLE IF EXISTS vote_records;
DROP TABLE IF EXISTS vote_tasks;
DROP TABLE IF EXISTS direct_scores;
DROP TABLE IF EXISTS vote_groups;
DROP TABLE IF EXISTS score_modules;
DROP TABLE IF EXISTS rule_templates;
DROP TABLE IF EXISTS rule_bindings;
DROP TABLE IF EXISTS assessment_rules;
DROP TABLE IF EXISTS assessment_categories;
DROP TABLE IF EXISTS assessment_objects;
DROP TABLE IF EXISTS assessment_periods;
DROP TABLE IF EXISTS assessment_years;

CREATE TABLE assessment_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    assessment_name VARCHAR(200) NOT NULL UNIQUE,
    display_name VARCHAR(200) NOT NULL,
    year INTEGER NOT NULL,
    organization_id INTEGER NOT NULL,
    description TEXT,
    data_dir VARCHAR(500) NOT NULL,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (organization_id) REFERENCES organizations(id)
);

CREATE TABLE assessment_session_periods (
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

CREATE TABLE assessment_object_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    assessment_id INTEGER NOT NULL,
    object_type VARCHAR(20) NOT NULL,
    group_code VARCHAR(80) NOT NULL,
    group_name VARCHAR(120) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_system BOOLEAN NOT NULL DEFAULT 0,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (assessment_id) REFERENCES assessment_sessions(id) ON DELETE CASCADE,
    UNIQUE (assessment_id, group_code)
);

CREATE TABLE assessment_session_objects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    assessment_id INTEGER NOT NULL,
    object_type VARCHAR(20) NOT NULL,
    group_code VARCHAR(80) NOT NULL,
    target_id INTEGER NOT NULL,
    target_type VARCHAR(20) NOT NULL,
    object_name VARCHAR(200) NOT NULL,
    parent_object_id INTEGER,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (assessment_id) REFERENCES assessment_sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_object_id) REFERENCES assessment_session_objects(id) ON DELETE SET NULL,
    UNIQUE (assessment_id, target_type, target_id)
);

CREATE TABLE rule_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    assessment_id INTEGER NOT NULL,
    rule_name VARCHAR(200) NOT NULL,
    description TEXT,
    content_json TEXT NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    is_copy BOOLEAN NOT NULL DEFAULT 0,
    source_rule_id INTEGER,
    owner_org_id INTEGER,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (assessment_id) REFERENCES assessment_sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (source_rule_id) REFERENCES rule_files(id) ON DELETE SET NULL,
    FOREIGN KEY (owner_org_id) REFERENCES organizations(id) ON DELETE SET NULL
);

CREATE TABLE rule_file_hides (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_file_id INTEGER NOT NULL,
    organization_id INTEGER NOT NULL,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (rule_file_id) REFERENCES rule_files(id) ON DELETE CASCADE,
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    UNIQUE (rule_file_id, organization_id)
);

CREATE TABLE assessment_rule_bindings (
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

CREATE INDEX idx_assessment_sessions_org ON assessment_sessions(organization_id);
CREATE INDEX idx_assessment_session_periods_assessment_id ON assessment_session_periods(assessment_id);
CREATE INDEX idx_assessment_object_groups_assessment_id ON assessment_object_groups(assessment_id);
CREATE INDEX idx_assessment_session_objects_assessment_id ON assessment_session_objects(assessment_id);
CREATE INDEX idx_assessment_session_objects_group_code ON assessment_session_objects(group_code);
CREATE INDEX idx_rule_files_is_copy ON rule_files(is_copy);
CREATE INDEX idx_rule_files_assessment_id ON rule_files(assessment_id);
CREATE INDEX idx_rule_files_owner_org_id ON rule_files(owner_org_id);
CREATE INDEX idx_rule_files_source_rule_id ON rule_files(source_rule_id);
CREATE INDEX idx_assessment_rule_bindings_lookup
    ON assessment_rule_bindings(assessment_id, period_code, object_group_code, organization_id);

PRAGMA foreign_keys = ON;
