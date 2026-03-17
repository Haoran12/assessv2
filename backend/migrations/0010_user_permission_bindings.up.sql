CREATE TABLE IF NOT EXISTS user_permission_bindings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    role_code VARCHAR(50),
    scope_org_type VARCHAR(20),
    scope_org_id INTEGER,
    person_object_id INTEGER,
    team_object_id INTEGER,
    is_primary BOOLEAN NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_permission_bindings_user_id
    ON user_permission_bindings(user_id);
CREATE INDEX IF NOT EXISTS idx_user_permission_bindings_role_code
    ON user_permission_bindings(role_code);
CREATE INDEX IF NOT EXISTS idx_user_permission_bindings_scope
    ON user_permission_bindings(scope_org_type, scope_org_id);
CREATE INDEX IF NOT EXISTS idx_user_permission_bindings_person
    ON user_permission_bindings(person_object_id);
CREATE INDEX IF NOT EXISTS idx_user_permission_bindings_team
    ON user_permission_bindings(team_object_id);
