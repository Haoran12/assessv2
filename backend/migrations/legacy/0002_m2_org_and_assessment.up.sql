PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS organizations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    org_code VARCHAR(50) NOT NULL UNIQUE,
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
    deleted_at INTEGER,
    FOREIGN KEY (parent_id) REFERENCES organizations(id),
    FOREIGN KEY (created_by) REFERENCES users(id),
    FOREIGN KEY (updated_by) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_organizations_org_type ON organizations(org_type);
CREATE INDEX IF NOT EXISTS idx_organizations_parent_id ON organizations(parent_id);
CREATE INDEX IF NOT EXISTS idx_organizations_status ON organizations(status);
CREATE INDEX IF NOT EXISTS idx_organizations_deleted_at ON organizations(deleted_at);

CREATE TABLE IF NOT EXISTS departments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    dept_code VARCHAR(50) NOT NULL,
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
    deleted_at INTEGER,
    FOREIGN KEY (organization_id) REFERENCES organizations(id),
    FOREIGN KEY (parent_dept_id) REFERENCES departments(id),
    FOREIGN KEY (created_by) REFERENCES users(id),
    FOREIGN KEY (updated_by) REFERENCES users(id),
    UNIQUE (organization_id, dept_code)
);
CREATE INDEX IF NOT EXISTS idx_departments_organization_id ON departments(organization_id);
CREATE INDEX IF NOT EXISTS idx_departments_parent_dept_id ON departments(parent_dept_id);
CREATE INDEX IF NOT EXISTS idx_departments_status ON departments(status);
CREATE INDEX IF NOT EXISTS idx_departments_deleted_at ON departments(deleted_at);

CREATE TABLE IF NOT EXISTS position_levels (
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
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (created_by) REFERENCES users(id),
    FOREIGN KEY (updated_by) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_position_levels_status ON position_levels(status);

CREATE TABLE IF NOT EXISTS employees (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    emp_code VARCHAR(50) NOT NULL UNIQUE,
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
    deleted_at INTEGER,
    FOREIGN KEY (organization_id) REFERENCES organizations(id),
    FOREIGN KEY (department_id) REFERENCES departments(id),
    FOREIGN KEY (position_level_id) REFERENCES position_levels(id),
    FOREIGN KEY (created_by) REFERENCES users(id),
    FOREIGN KEY (updated_by) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_employees_organization_id ON employees(organization_id);
CREATE INDEX IF NOT EXISTS idx_employees_department_id ON employees(department_id);
CREATE INDEX IF NOT EXISTS idx_employees_position_level_id ON employees(position_level_id);
CREATE INDEX IF NOT EXISTS idx_employees_status ON employees(status);
CREATE INDEX IF NOT EXISTS idx_employees_deleted_at ON employees(deleted_at);

CREATE TABLE IF NOT EXISTS employee_history (
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
    FOREIGN KEY (new_position_level_id) REFERENCES position_levels(id),
    FOREIGN KEY (created_by) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_employee_history_employee_id ON employee_history(employee_id);
CREATE INDEX IF NOT EXISTS idx_employee_history_change_type ON employee_history(change_type);
CREATE INDEX IF NOT EXISTS idx_employee_history_effective_date ON employee_history(effective_date);

CREATE TABLE IF NOT EXISTS assessment_years (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    year INTEGER NOT NULL UNIQUE,
    year_name VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'preparing',
    description TEXT,
    created_by INTEGER,
    created_at INTEGER NOT NULL,
    updated_by INTEGER,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (created_by) REFERENCES users(id),
    FOREIGN KEY (updated_by) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_assessment_years_status ON assessment_years(status);

CREATE TABLE IF NOT EXISTS assessment_periods (
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
    FOREIGN KEY (created_by) REFERENCES users(id),
    FOREIGN KEY (updated_by) REFERENCES users(id),
    UNIQUE (year_id, period_code)
);
CREATE INDEX IF NOT EXISTS idx_assessment_periods_year_id ON assessment_periods(year_id);
CREATE INDEX IF NOT EXISTS idx_assessment_periods_status ON assessment_periods(status);

CREATE TABLE IF NOT EXISTS assessment_objects (
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
    FOREIGN KEY (created_by) REFERENCES users(id),
    FOREIGN KEY (updated_by) REFERENCES users(id),
    UNIQUE (year_id, target_type, target_id)
);
CREATE INDEX IF NOT EXISTS idx_assessment_objects_year_id ON assessment_objects(year_id);
CREATE INDEX IF NOT EXISTS idx_assessment_objects_object_type ON assessment_objects(object_type);
CREATE INDEX IF NOT EXISTS idx_assessment_objects_target_type ON assessment_objects(target_type);
CREATE INDEX IF NOT EXISTS idx_assessment_objects_parent_object_id ON assessment_objects(parent_object_id);
CREATE INDEX IF NOT EXISTS idx_assessment_objects_is_active ON assessment_objects(is_active);
