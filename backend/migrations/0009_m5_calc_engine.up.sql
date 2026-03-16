PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS calculated_scores (
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
    FOREIGN KEY (triggered_by) REFERENCES users(id),
    UNIQUE (year_id, period_code, object_id)
);
CREATE INDEX IF NOT EXISTS idx_calculated_scores_year_period ON calculated_scores(year_id, period_code);
CREATE INDEX IF NOT EXISTS idx_calculated_scores_object_id ON calculated_scores(object_id);
CREATE INDEX IF NOT EXISTS idx_calculated_scores_rule_id ON calculated_scores(rule_id);
CREATE INDEX IF NOT EXISTS idx_calculated_scores_final_score ON calculated_scores(final_score);

CREATE TABLE IF NOT EXISTS calculated_module_scores (
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
CREATE INDEX IF NOT EXISTS idx_calculated_module_scores_calc_id ON calculated_module_scores(calculated_score_id);
CREATE INDEX IF NOT EXISTS idx_calculated_module_scores_module_key ON calculated_module_scores(module_key);
CREATE INDEX IF NOT EXISTS idx_calculated_module_scores_sort_order ON calculated_module_scores(sort_order);

CREATE TABLE IF NOT EXISTS rankings (
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
CREATE INDEX IF NOT EXISTS idx_rankings_year_period_scope ON rankings(year_id, period_code, ranking_scope, scope_key);
CREATE INDEX IF NOT EXISTS idx_rankings_object_id ON rankings(object_id);
CREATE INDEX IF NOT EXISTS idx_rankings_rank_no ON rankings(rank_no);
