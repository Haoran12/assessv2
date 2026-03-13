PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS assessment_categories (
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

CREATE INDEX IF NOT EXISTS idx_assessment_categories_object_type ON assessment_categories(object_type);
CREATE INDEX IF NOT EXISTS idx_assessment_categories_status ON assessment_categories(status);
