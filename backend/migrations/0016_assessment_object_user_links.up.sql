PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS assessment_object_user_links (
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

CREATE INDEX IF NOT EXISTS idx_object_user_links_user_id
    ON assessment_object_user_links(user_id);
CREATE INDEX IF NOT EXISTS idx_object_user_links_object_id
    ON assessment_object_user_links(assessment_object_id);
CREATE INDEX IF NOT EXISTS idx_object_user_links_user_active
    ON assessment_object_user_links(user_id, is_active);
CREATE INDEX IF NOT EXISTS idx_object_user_links_object_access
    ON assessment_object_user_links(assessment_object_id, access_level);
