PRAGMA foreign_keys = ON;

ALTER TABLE audit_logs RENAME TO audit_logs_old;
CREATE TABLE audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    action_type VARCHAR(50) NOT NULL,
    target_type VARCHAR(50),
    target_id INTEGER,
    action_detail TEXT,
    ip_address VARCHAR(50),
    user_agent TEXT,
    created_at INTEGER NOT NULL
);
INSERT INTO audit_logs (id, user_id, action_type, target_type, target_id, action_detail, ip_address, user_agent, created_at)
SELECT id, user_id, action_type, target_type, target_id, action_detail, ip_address, user_agent, created_at
FROM audit_logs_old;
DROP TABLE audit_logs_old;
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action_type ON audit_logs(action_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_target_type ON audit_logs(target_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_target_id ON audit_logs(target_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);

ALTER TABLE backups RENAME TO backups_old;
CREATE TABLE backups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    backup_name VARCHAR(200) NOT NULL,
    backup_path VARCHAR(500) NOT NULL,
    backup_type VARCHAR(20) NOT NULL,
    file_size INTEGER,
    description TEXT,
    created_by INTEGER,
    created_at INTEGER NOT NULL
);
INSERT INTO backups (id, backup_name, backup_path, backup_type, file_size, description, created_by, created_at)
SELECT id, backup_name, backup_path, backup_type, file_size, description, created_by, created_at
FROM backups_old;
DROP TABLE backups_old;
CREATE INDEX IF NOT EXISTS idx_backups_backup_type ON backups(backup_type);
CREATE INDEX IF NOT EXISTS idx_backups_created_at ON backups(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_backups_created_by ON backups(created_by);

ALTER TABLE rule_templates RENAME TO rule_templates_old;
CREATE TABLE rule_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    template_name VARCHAR(200) NOT NULL,
    object_type VARCHAR(20) NOT NULL,
    object_category VARCHAR(50) NOT NULL,
    template_config TEXT NOT NULL,
    description TEXT,
    is_system BOOLEAN NOT NULL DEFAULT 0,
    permission_mode SMALLINT NOT NULL DEFAULT 420,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL
);
INSERT INTO rule_templates (id, template_name, object_type, object_category, template_config, description, is_system, permission_mode, created_by, created_at, updated_by, updated_at)
SELECT id, template_name, object_type, object_category, template_config, description, is_system, permission_mode, created_by, created_at, updated_by, updated_at
FROM rule_templates_old;
DROP TABLE rule_templates_old;
CREATE INDEX IF NOT EXISTS idx_rule_templates_object_type ON rule_templates(object_type);
CREATE INDEX IF NOT EXISTS idx_rule_templates_object_category ON rule_templates(object_category);
CREATE INDEX IF NOT EXISTS idx_rule_templates_is_system ON rule_templates(is_system);

ALTER TABLE rule_bindings RENAME TO rule_bindings_old;
CREATE TABLE rule_bindings (
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
    FOREIGN KEY (rule_id) REFERENCES assessment_rules(id) ON DELETE CASCADE
);
INSERT INTO rule_bindings (id, year_id, period_code, object_type, segment_code, owner_scope, owner_org_type, owner_org_id, rule_id, priority, description, is_active, created_by, created_at, updated_by, updated_at)
SELECT id, year_id, period_code, object_type, segment_code, owner_scope, owner_org_type, owner_org_id, rule_id, priority, description, is_active, created_by, created_at, updated_by, updated_at
FROM rule_bindings_old;
DROP TABLE rule_bindings_old;
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

ALTER TABLE direct_scores RENAME TO direct_scores_old;
CREATE TABLE direct_scores (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    year_id INTEGER NOT NULL,
    period_code VARCHAR(20) NOT NULL,
    module_id INTEGER NOT NULL,
    object_id INTEGER NOT NULL,
    score DECIMAL(10, 6) NOT NULL,
    remark TEXT,
    input_by INTEGER NOT NULL,
    input_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER,
    permission_mode SMALLINT NOT NULL DEFAULT 384,
    FOREIGN KEY (year_id) REFERENCES assessment_years(id) ON DELETE CASCADE,
    FOREIGN KEY (module_id) REFERENCES score_modules(id) ON DELETE CASCADE,
    FOREIGN KEY (object_id) REFERENCES assessment_objects(id) ON DELETE CASCADE,
    UNIQUE (year_id, period_code, module_id, object_id)
);
INSERT INTO direct_scores (id, year_id, period_code, module_id, object_id, score, remark, input_by, input_at, updated_by, updated_at, permission_mode)
SELECT id, year_id, period_code, module_id, object_id, score, remark, input_by, input_at, updated_by, updated_at, permission_mode
FROM direct_scores_old;
DROP TABLE direct_scores_old;
CREATE INDEX IF NOT EXISTS idx_direct_scores_year_id ON direct_scores(year_id);
CREATE INDEX IF NOT EXISTS idx_direct_scores_period_code ON direct_scores(period_code);
CREATE INDEX IF NOT EXISTS idx_direct_scores_module_id ON direct_scores(module_id);
CREATE INDEX IF NOT EXISTS idx_direct_scores_object_id ON direct_scores(object_id);
CREATE INDEX IF NOT EXISTS idx_direct_scores_input_by ON direct_scores(input_by);
CREATE INDEX IF NOT EXISTS idx_direct_scores_year_period ON direct_scores(year_id, period_code);

ALTER TABLE extra_points RENAME TO extra_points_old;
CREATE TABLE extra_points (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    year_id INTEGER NOT NULL,
    period_code VARCHAR(20) NOT NULL,
    object_id INTEGER NOT NULL,
    point_type VARCHAR(20) NOT NULL,
    points DECIMAL(10, 6) NOT NULL,
    reason TEXT NOT NULL,
    evidence TEXT,
    approved_by INTEGER,
    approved_at INTEGER,
    input_by INTEGER NOT NULL,
    input_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER,
    permission_mode SMALLINT NOT NULL DEFAULT 416,
    FOREIGN KEY (year_id) REFERENCES assessment_years(id) ON DELETE CASCADE,
    FOREIGN KEY (object_id) REFERENCES assessment_objects(id) ON DELETE CASCADE
);
INSERT INTO extra_points (id, year_id, period_code, object_id, point_type, points, reason, evidence, approved_by, approved_at, input_by, input_at, updated_by, updated_at, permission_mode)
SELECT id, year_id, period_code, object_id, point_type, points, reason, evidence, approved_by, approved_at, input_by, input_at, updated_by, updated_at, permission_mode
FROM extra_points_old;
DROP TABLE extra_points_old;
CREATE INDEX IF NOT EXISTS idx_extra_points_year_id ON extra_points(year_id);
CREATE INDEX IF NOT EXISTS idx_extra_points_period_code ON extra_points(period_code);
CREATE INDEX IF NOT EXISTS idx_extra_points_object_id ON extra_points(object_id);
CREATE INDEX IF NOT EXISTS idx_extra_points_point_type ON extra_points(point_type);
CREATE INDEX IF NOT EXISTS idx_extra_points_year_period ON extra_points(year_id, period_code);

ALTER TABLE vote_records RENAME TO vote_records_old;
ALTER TABLE vote_tasks RENAME TO vote_tasks_old;
CREATE TABLE vote_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    year_id INTEGER NOT NULL,
    period_code VARCHAR(20) NOT NULL,
    vote_group_id INTEGER NOT NULL,
    object_id INTEGER NOT NULL,
    voter_id INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    completed_at INTEGER,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (year_id) REFERENCES assessment_years(id) ON DELETE CASCADE,
    FOREIGN KEY (vote_group_id) REFERENCES vote_groups(id) ON DELETE CASCADE,
    FOREIGN KEY (object_id) REFERENCES assessment_objects(id) ON DELETE CASCADE,
    UNIQUE (year_id, period_code, vote_group_id, object_id, voter_id)
);
INSERT INTO vote_tasks (id, year_id, period_code, vote_group_id, object_id, voter_id, status, completed_at, created_by, created_at, updated_at)
SELECT id, year_id, period_code, vote_group_id, object_id, voter_id, status, completed_at, created_by, created_at, updated_at
FROM vote_tasks_old;

CREATE TABLE vote_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL UNIQUE,
    grade_option VARCHAR(20) NOT NULL,
    remark TEXT,
    voted_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (task_id) REFERENCES vote_tasks(id) ON DELETE CASCADE
);
INSERT INTO vote_records (id, task_id, grade_option, remark, voted_at, created_at, updated_at)
SELECT id, task_id, grade_option, remark, voted_at, created_at, updated_at
FROM vote_records_old;
DROP TABLE vote_records_old;
DROP TABLE vote_tasks_old;
CREATE INDEX IF NOT EXISTS idx_vote_tasks_year_id ON vote_tasks(year_id);
CREATE INDEX IF NOT EXISTS idx_vote_tasks_period_code ON vote_tasks(period_code);
CREATE INDEX IF NOT EXISTS idx_vote_tasks_vote_group_id ON vote_tasks(vote_group_id);
CREATE INDEX IF NOT EXISTS idx_vote_tasks_object_id ON vote_tasks(object_id);
CREATE INDEX IF NOT EXISTS idx_vote_tasks_voter_id ON vote_tasks(voter_id);
CREATE INDEX IF NOT EXISTS idx_vote_tasks_status ON vote_tasks(status);
CREATE INDEX IF NOT EXISTS idx_vote_tasks_year_period ON vote_tasks(year_id, period_code);
CREATE INDEX IF NOT EXISTS idx_vote_records_grade_option ON vote_records(grade_option);
CREATE INDEX IF NOT EXISTS idx_vote_records_voted_at ON vote_records(voted_at);
