PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS assessment_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    year_id INTEGER NOT NULL,
    period_code VARCHAR(20) NOT NULL,
    object_type VARCHAR(20) NOT NULL,
    object_category VARCHAR(50) NOT NULL,
    rule_name VARCHAR(200) NOT NULL,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (year_id) REFERENCES assessment_years(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES users(id),
    FOREIGN KEY (updated_by) REFERENCES users(id),
    UNIQUE (year_id, period_code, object_type, object_category)
);
CREATE INDEX IF NOT EXISTS idx_assessment_rules_year_id ON assessment_rules(year_id);
CREATE INDEX IF NOT EXISTS idx_assessment_rules_period_code ON assessment_rules(period_code);
CREATE INDEX IF NOT EXISTS idx_assessment_rules_object_type ON assessment_rules(object_type);
CREATE INDEX IF NOT EXISTS idx_assessment_rules_object_category ON assessment_rules(object_category);
CREATE INDEX IF NOT EXISTS idx_assessment_rules_is_active ON assessment_rules(is_active);

CREATE TABLE IF NOT EXISTS score_modules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER NOT NULL,
    module_code VARCHAR(50) NOT NULL,
    module_key VARCHAR(100) NOT NULL,
    module_name VARCHAR(100) NOT NULL,
    weight DECIMAL(5, 4),
    max_score DECIMAL(10, 6),
    calculation_method VARCHAR(50),
    expression TEXT,
    context_scope TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (rule_id) REFERENCES assessment_rules(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES users(id),
    FOREIGN KEY (updated_by) REFERENCES users(id),
    UNIQUE (rule_id, module_key)
);
CREATE INDEX IF NOT EXISTS idx_score_modules_rule_id ON score_modules(rule_id);
CREATE INDEX IF NOT EXISTS idx_score_modules_module_code ON score_modules(module_code);
CREATE INDEX IF NOT EXISTS idx_score_modules_module_key ON score_modules(module_key);
CREATE INDEX IF NOT EXISTS idx_score_modules_sort_order ON score_modules(sort_order);
CREATE INDEX IF NOT EXISTS idx_score_modules_is_active ON score_modules(is_active);

CREATE TABLE IF NOT EXISTS vote_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    module_id INTEGER NOT NULL,
    group_code VARCHAR(50) NOT NULL,
    group_name VARCHAR(100) NOT NULL,
    weight DECIMAL(5, 4) NOT NULL,
    voter_type VARCHAR(50) NOT NULL,
    voter_scope TEXT,
    max_score DECIMAL(10, 6) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (module_id) REFERENCES score_modules(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES users(id),
    FOREIGN KEY (updated_by) REFERENCES users(id),
    UNIQUE (module_id, group_code)
);
CREATE INDEX IF NOT EXISTS idx_vote_groups_module_id ON vote_groups(module_id);
CREATE INDEX IF NOT EXISTS idx_vote_groups_voter_type ON vote_groups(voter_type);
CREATE INDEX IF NOT EXISTS idx_vote_groups_sort_order ON vote_groups(sort_order);
CREATE INDEX IF NOT EXISTS idx_vote_groups_is_active ON vote_groups(is_active);

CREATE TABLE IF NOT EXISTS rule_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    template_name VARCHAR(200) NOT NULL,
    object_type VARCHAR(20) NOT NULL,
    object_category VARCHAR(50) NOT NULL,
    template_config TEXT NOT NULL,
    description TEXT,
    is_system BOOLEAN NOT NULL DEFAULT 0,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (created_by) REFERENCES users(id),
    FOREIGN KEY (updated_by) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_rule_templates_object_type ON rule_templates(object_type);
CREATE INDEX IF NOT EXISTS idx_rule_templates_object_category ON rule_templates(object_category);
CREATE INDEX IF NOT EXISTS idx_rule_templates_is_system ON rule_templates(is_system);
