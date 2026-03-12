PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS direct_scores (
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
    FOREIGN KEY (year_id) REFERENCES assessment_years(id) ON DELETE CASCADE,
    FOREIGN KEY (module_id) REFERENCES score_modules(id) ON DELETE CASCADE,
    FOREIGN KEY (object_id) REFERENCES assessment_objects(id) ON DELETE CASCADE,
    FOREIGN KEY (input_by) REFERENCES users(id),
    FOREIGN KEY (updated_by) REFERENCES users(id),
    UNIQUE (year_id, period_code, module_id, object_id)
);
CREATE INDEX IF NOT EXISTS idx_direct_scores_year_id ON direct_scores(year_id);
CREATE INDEX IF NOT EXISTS idx_direct_scores_period_code ON direct_scores(period_code);
CREATE INDEX IF NOT EXISTS idx_direct_scores_module_id ON direct_scores(module_id);
CREATE INDEX IF NOT EXISTS idx_direct_scores_object_id ON direct_scores(object_id);
CREATE INDEX IF NOT EXISTS idx_direct_scores_input_by ON direct_scores(input_by);
CREATE INDEX IF NOT EXISTS idx_direct_scores_year_period ON direct_scores(year_id, period_code);

CREATE TABLE IF NOT EXISTS vote_tasks (
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
    FOREIGN KEY (voter_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES users(id),
    UNIQUE (year_id, period_code, vote_group_id, object_id, voter_id)
);
CREATE INDEX IF NOT EXISTS idx_vote_tasks_year_id ON vote_tasks(year_id);
CREATE INDEX IF NOT EXISTS idx_vote_tasks_period_code ON vote_tasks(period_code);
CREATE INDEX IF NOT EXISTS idx_vote_tasks_vote_group_id ON vote_tasks(vote_group_id);
CREATE INDEX IF NOT EXISTS idx_vote_tasks_object_id ON vote_tasks(object_id);
CREATE INDEX IF NOT EXISTS idx_vote_tasks_voter_id ON vote_tasks(voter_id);
CREATE INDEX IF NOT EXISTS idx_vote_tasks_status ON vote_tasks(status);
CREATE INDEX IF NOT EXISTS idx_vote_tasks_year_period ON vote_tasks(year_id, period_code);

CREATE TABLE IF NOT EXISTS vote_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL UNIQUE,
    grade_option VARCHAR(20) NOT NULL,
    remark TEXT,
    voted_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (task_id) REFERENCES vote_tasks(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_vote_records_grade_option ON vote_records(grade_option);
CREATE INDEX IF NOT EXISTS idx_vote_records_voted_at ON vote_records(voted_at);

CREATE TABLE IF NOT EXISTS extra_points (
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
    FOREIGN KEY (year_id) REFERENCES assessment_years(id) ON DELETE CASCADE,
    FOREIGN KEY (object_id) REFERENCES assessment_objects(id) ON DELETE CASCADE,
    FOREIGN KEY (approved_by) REFERENCES users(id),
    FOREIGN KEY (input_by) REFERENCES users(id),
    FOREIGN KEY (updated_by) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_extra_points_year_id ON extra_points(year_id);
CREATE INDEX IF NOT EXISTS idx_extra_points_period_code ON extra_points(period_code);
CREATE INDEX IF NOT EXISTS idx_extra_points_object_id ON extra_points(object_id);
CREATE INDEX IF NOT EXISTS idx_extra_points_point_type ON extra_points(point_type);
CREATE INDEX IF NOT EXISTS idx_extra_points_year_period ON extra_points(year_id, period_code);
