PRAGMA foreign_keys = ON;

CREATE TABLE system_settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    setting_key VARCHAR(100) NOT NULL UNIQUE,
    setting_value TEXT,
    setting_type VARCHAR(20) NOT NULL,
    description TEXT,
    is_system BOOLEAN NOT NULL DEFAULT 0,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL
);

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

CREATE TABLE organizations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    org_name VARCHAR(200) NOT NULL,
    org_type VARCHAR(20) NOT NULL,
    parent_id INTEGER,
    leader_id INTEGER,
    sort_order INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    deleted_at INTEGER, permission_mode SMALLINT NOT NULL DEFAULT 420,
    FOREIGN KEY (parent_id) REFERENCES organizations(id)
);

CREATE TABLE departments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    dept_name VARCHAR(200) NOT NULL,
    organization_id INTEGER NOT NULL,
    parent_dept_id INTEGER,
    leader_id INTEGER,
    sort_order INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    deleted_at INTEGER, permission_mode SMALLINT NOT NULL DEFAULT 420,
    FOREIGN KEY (organization_id) REFERENCES organizations(id),
    FOREIGN KEY (parent_dept_id) REFERENCES departments(id)
);

CREATE TABLE position_levels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    level_code VARCHAR(50) NOT NULL UNIQUE,
    level_name VARCHAR(100) NOT NULL,
    description TEXT,
    is_system BOOLEAN NOT NULL DEFAULT 0,
    is_for_assessment BOOLEAN NOT NULL DEFAULT 1,
    sort_order INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL
);

CREATE TABLE employees (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    emp_name VARCHAR(100) NOT NULL,
    organization_id INTEGER NOT NULL,
    department_id INTEGER,
    position_level_id INTEGER NOT NULL,
    position_title VARCHAR(100),
    hire_date DATE,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    deleted_at INTEGER, permission_mode SMALLINT NOT NULL DEFAULT 416,
    FOREIGN KEY (organization_id) REFERENCES organizations(id),
    FOREIGN KEY (department_id) REFERENCES departments(id),
    FOREIGN KEY (position_level_id) REFERENCES position_levels(id)
);

CREATE TABLE employee_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    employee_id INTEGER NOT NULL,
    change_type VARCHAR(50) NOT NULL,
    old_organization_id INTEGER,
    new_organization_id INTEGER,
    old_department_id INTEGER,
    new_department_id INTEGER,
    old_position_level_id INTEGER,
    new_position_level_id INTEGER,
    old_position_title VARCHAR(100),
    new_position_title VARCHAR(100),
    change_reason TEXT,
    effective_date DATE NOT NULL,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (employee_id) REFERENCES employees(id),
    FOREIGN KEY (old_organization_id) REFERENCES organizations(id),
    FOREIGN KEY (new_organization_id) REFERENCES organizations(id),
    FOREIGN KEY (old_department_id) REFERENCES departments(id),
    FOREIGN KEY (new_department_id) REFERENCES departments(id),
    FOREIGN KEY (old_position_level_id) REFERENCES position_levels(id),
    FOREIGN KEY (new_position_level_id) REFERENCES position_levels(id)
);

CREATE TABLE assessment_years (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    year INTEGER NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'preparing',
    description TEXT,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL, permission_mode SMALLINT NOT NULL DEFAULT 420
);

CREATE TABLE assessment_periods (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    year_id INTEGER NOT NULL,
    period_code VARCHAR(20) NOT NULL,
    period_name VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'preparing',
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (year_id) REFERENCES assessment_years(id) ON DELETE CASCADE,
    UNIQUE (year_id, period_code)
);

CREATE TABLE assessment_objects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    year_id INTEGER NOT NULL,
    object_type VARCHAR(20) NOT NULL,
    object_category VARCHAR(50) NOT NULL,
    target_id INTEGER NOT NULL,
    target_type VARCHAR(20) NOT NULL,
    object_name VARCHAR(200) NOT NULL,
    parent_object_id INTEGER,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (year_id) REFERENCES assessment_years(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_object_id) REFERENCES assessment_objects(id),
    UNIQUE (year_id, target_type, target_id)
);

CREATE TABLE assessment_categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    category_code VARCHAR(50) NOT NULL UNIQUE,
    category_name VARCHAR(100) NOT NULL,
    object_type VARCHAR(20) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_system INTEGER NOT NULL DEFAULT 1,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
);

CREATE TABLE assessment_rules (
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
    updated_at INTEGER NOT NULL, permission_mode SMALLINT NOT NULL DEFAULT 420,
    FOREIGN KEY (year_id) REFERENCES assessment_years(id) ON DELETE CASCADE,
    UNIQUE (year_id, period_code, object_type, object_category)
);

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

CREATE TABLE score_modules (
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
    updated_at INTEGER NOT NULL, permission_mode SMALLINT NOT NULL DEFAULT 420,
    FOREIGN KEY (rule_id) REFERENCES assessment_rules(id) ON DELETE CASCADE,
    UNIQUE (rule_id, module_key)
);

CREATE TABLE vote_groups (
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
    updated_at INTEGER NOT NULL, permission_mode SMALLINT NOT NULL DEFAULT 420,
    FOREIGN KEY (module_id) REFERENCES score_modules(id) ON DELETE CASCADE,
    UNIQUE (module_id, group_code)
);

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

CREATE TABLE calculated_scores (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    year_id INTEGER NOT NULL,
    period_code VARCHAR(20) NOT NULL,
    object_id INTEGER NOT NULL,
    rule_id INTEGER NOT NULL,
    weighted_score DECIMAL(10, 6) NOT NULL DEFAULT 0,
    extra_points DECIMAL(10, 6) NOT NULL DEFAULT 0,
    final_score DECIMAL(10, 6) NOT NULL DEFAULT 0,
    rank_basis TEXT,
    detail_json TEXT,
    trigger_mode VARCHAR(20) NOT NULL DEFAULT 'auto',
    triggered_by INTEGER,
    calculated_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (year_id) REFERENCES assessment_years(id) ON DELETE CASCADE,
    FOREIGN KEY (object_id) REFERENCES assessment_objects(id) ON DELETE CASCADE,
    FOREIGN KEY (rule_id) REFERENCES assessment_rules(id) ON DELETE CASCADE,
    UNIQUE (year_id, period_code, object_id)
);

CREATE TABLE calculated_module_scores (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    calculated_score_id INTEGER NOT NULL,
    module_id INTEGER NOT NULL,
    module_code VARCHAR(50) NOT NULL,
    module_key VARCHAR(100) NOT NULL,
    module_name VARCHAR(100) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    raw_score DECIMAL(10, 6) NOT NULL DEFAULT 0,
    weighted_score DECIMAL(10, 6) NOT NULL DEFAULT 0,
    score_detail TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (calculated_score_id) REFERENCES calculated_scores(id) ON DELETE CASCADE,
    FOREIGN KEY (module_id) REFERENCES score_modules(id) ON DELETE CASCADE,
    UNIQUE (calculated_score_id, module_id)
);

CREATE TABLE rankings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    year_id INTEGER NOT NULL,
    period_code VARCHAR(20) NOT NULL,
    object_id INTEGER NOT NULL,
    object_type VARCHAR(20) NOT NULL,
    object_category VARCHAR(50) NOT NULL,
    ranking_scope VARCHAR(30) NOT NULL,
    scope_key VARCHAR(100) NOT NULL,
    rank_no INTEGER NOT NULL,
    score DECIMAL(10, 6) NOT NULL,
    tie_break_key TEXT,
    calculated_score_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (year_id) REFERENCES assessment_years(id) ON DELETE CASCADE,
    FOREIGN KEY (object_id) REFERENCES assessment_objects(id) ON DELETE CASCADE,
    FOREIGN KEY (calculated_score_id) REFERENCES calculated_scores(id) ON DELETE CASCADE,
    UNIQUE (year_id, period_code, object_id, ranking_scope, scope_key)
);

CREATE TABLE assessment_object_user_links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    assessment_object_id INTEGER NOT NULL,
    link_type VARCHAR(30) NOT NULL DEFAULT 'member',
    access_level VARCHAR(20) NOT NULL DEFAULT 'detail' CHECK (access_level IN ('read', 'detail')),
    is_primary BOOLEAN NOT NULL DEFAULT 0,
    effective_from INTEGER,
    effective_to INTEGER,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (assessment_object_id) REFERENCES assessment_objects(id) ON DELETE CASCADE,
    CHECK (effective_to IS NULL OR effective_from IS NULL OR effective_to >= effective_from),
    UNIQUE (user_id, assessment_object_id, link_type)
);

CREATE INDEX idx_audit_logs_action_type ON audit_logs(action_type);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX idx_audit_logs_target_id ON audit_logs(target_id);
CREATE INDEX idx_audit_logs_target_type ON audit_logs(target_type);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_backups_backup_type ON backups(backup_type);
CREATE INDEX idx_backups_created_at ON backups(created_at DESC);
CREATE INDEX idx_backups_created_by ON backups(created_by);
CREATE INDEX idx_position_levels_status ON position_levels(status);
CREATE INDEX idx_employee_history_change_type ON employee_history(change_type);
CREATE INDEX idx_employee_history_effective_date ON employee_history(effective_date);
CREATE INDEX idx_employee_history_employee_id ON employee_history(employee_id);
CREATE INDEX idx_assessment_years_status ON assessment_years(status);
CREATE INDEX idx_assessment_periods_status ON assessment_periods(status);
CREATE INDEX idx_assessment_periods_year_id ON assessment_periods(year_id);
CREATE INDEX idx_assessment_objects_is_active ON assessment_objects(is_active);
CREATE INDEX idx_assessment_objects_object_type ON assessment_objects(object_type);
CREATE INDEX idx_assessment_objects_parent_object_id ON assessment_objects(parent_object_id);
CREATE INDEX idx_assessment_objects_target_type ON assessment_objects(target_type);
CREATE INDEX idx_assessment_objects_year_id ON assessment_objects(year_id);
CREATE INDEX idx_assessment_categories_object_type ON assessment_categories(object_type);
CREATE INDEX idx_assessment_categories_status ON assessment_categories(status);
CREATE INDEX idx_assessment_rules_is_active ON assessment_rules(is_active);
CREATE INDEX idx_assessment_rules_object_category ON assessment_rules(object_category);
CREATE INDEX idx_assessment_rules_object_type ON assessment_rules(object_type);
CREATE INDEX idx_assessment_rules_period_code ON assessment_rules(period_code);
CREATE INDEX idx_assessment_rules_year_id ON assessment_rules(year_id);
CREATE INDEX idx_rule_bindings_lookup
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
CREATE INDEX idx_rule_bindings_rule_id ON rule_bindings(rule_id);
CREATE INDEX idx_rule_templates_is_system ON rule_templates(is_system);
CREATE INDEX idx_rule_templates_object_category ON rule_templates(object_category);
CREATE INDEX idx_rule_templates_object_type ON rule_templates(object_type);
CREATE INDEX idx_score_modules_is_active ON score_modules(is_active);
CREATE INDEX idx_score_modules_module_code ON score_modules(module_code);
CREATE INDEX idx_score_modules_module_key ON score_modules(module_key);
CREATE INDEX idx_score_modules_rule_id ON score_modules(rule_id);
CREATE INDEX idx_score_modules_sort_order ON score_modules(sort_order);
CREATE INDEX idx_vote_groups_is_active ON vote_groups(is_active);
CREATE INDEX idx_vote_groups_module_id ON vote_groups(module_id);
CREATE INDEX idx_vote_groups_sort_order ON vote_groups(sort_order);
CREATE INDEX idx_vote_groups_voter_type ON vote_groups(voter_type);
CREATE INDEX idx_direct_scores_input_by ON direct_scores(input_by);
CREATE INDEX idx_direct_scores_module_id ON direct_scores(module_id);
CREATE INDEX idx_direct_scores_object_id ON direct_scores(object_id);
CREATE INDEX idx_direct_scores_period_code ON direct_scores(period_code);
CREATE INDEX idx_direct_scores_year_id ON direct_scores(year_id);
CREATE INDEX idx_direct_scores_year_period ON direct_scores(year_id, period_code);
CREATE INDEX idx_vote_tasks_object_id ON vote_tasks(object_id);
CREATE INDEX idx_vote_tasks_period_code ON vote_tasks(period_code);
CREATE INDEX idx_vote_tasks_status ON vote_tasks(status);
CREATE INDEX idx_vote_tasks_vote_group_id ON vote_tasks(vote_group_id);
CREATE INDEX idx_vote_tasks_voter_id ON vote_tasks(voter_id);
CREATE INDEX idx_vote_tasks_year_id ON vote_tasks(year_id);
CREATE INDEX idx_vote_tasks_year_period ON vote_tasks(year_id, period_code);
CREATE INDEX idx_vote_records_grade_option ON vote_records(grade_option);
CREATE INDEX idx_vote_records_voted_at ON vote_records(voted_at);
CREATE INDEX idx_extra_points_object_id ON extra_points(object_id);
CREATE INDEX idx_extra_points_period_code ON extra_points(period_code);
CREATE INDEX idx_extra_points_point_type ON extra_points(point_type);
CREATE INDEX idx_extra_points_year_id ON extra_points(year_id);
CREATE INDEX idx_extra_points_year_period ON extra_points(year_id, period_code);
CREATE INDEX idx_calculated_scores_final_score ON calculated_scores(final_score);
CREATE INDEX idx_calculated_scores_object_id ON calculated_scores(object_id);
CREATE INDEX idx_calculated_scores_rule_id ON calculated_scores(rule_id);
CREATE INDEX idx_calculated_scores_year_period ON calculated_scores(year_id, period_code);
CREATE INDEX idx_calculated_module_scores_calc_id ON calculated_module_scores(calculated_score_id);
CREATE INDEX idx_calculated_module_scores_module_key ON calculated_module_scores(module_key);
CREATE INDEX idx_calculated_module_scores_sort_order ON calculated_module_scores(sort_order);
CREATE INDEX idx_rankings_object_id ON rankings(object_id);
CREATE INDEX idx_rankings_rank_no ON rankings(rank_no);
CREATE INDEX idx_rankings_year_period_scope ON rankings(year_id, period_code, ranking_scope, scope_key);
CREATE INDEX idx_object_user_links_object_access
    ON assessment_object_user_links(assessment_object_id, access_level);
CREATE INDEX idx_object_user_links_object_id
    ON assessment_object_user_links(assessment_object_id);
CREATE INDEX idx_object_user_links_user_active
    ON assessment_object_user_links(user_id, is_active);
CREATE INDEX idx_object_user_links_user_id
    ON assessment_object_user_links(user_id);

