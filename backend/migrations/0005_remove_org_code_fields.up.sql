ALTER TABLE organizations RENAME TO organizations_old;
ALTER TABLE departments RENAME TO departments_old;
ALTER TABLE employees RENAME TO employees_old;

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
    deleted_at INTEGER,
    FOREIGN KEY (parent_id) REFERENCES organizations(id),
    FOREIGN KEY (created_by) REFERENCES users(id),
    FOREIGN KEY (updated_by) REFERENCES users(id)
);

INSERT INTO organizations (
    id, org_name, org_type, parent_id, leader_id, sort_order, status,
    created_by, created_at, updated_by, updated_at, deleted_at
)
SELECT
    id, org_name, org_type, parent_id, leader_id, sort_order, status,
    created_by, created_at, updated_by, updated_at, deleted_at
FROM organizations_old;

CREATE INDEX IF NOT EXISTS idx_organizations_org_type ON organizations(org_type);
CREATE INDEX IF NOT EXISTS idx_organizations_parent_id ON organizations(parent_id);
CREATE INDEX IF NOT EXISTS idx_organizations_status ON organizations(status);
CREATE INDEX IF NOT EXISTS idx_organizations_deleted_at ON organizations(deleted_at);

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
    deleted_at INTEGER,
    FOREIGN KEY (organization_id) REFERENCES organizations(id),
    FOREIGN KEY (parent_dept_id) REFERENCES departments(id),
    FOREIGN KEY (created_by) REFERENCES users(id),
    FOREIGN KEY (updated_by) REFERENCES users(id)
);

INSERT INTO departments (
    id, dept_name, organization_id, parent_dept_id, leader_id, sort_order, status,
    created_by, created_at, updated_by, updated_at, deleted_at
)
SELECT
    id, dept_name, organization_id, parent_dept_id, leader_id, sort_order, status,
    created_by, created_at, updated_by, updated_at, deleted_at
FROM departments_old;

CREATE INDEX IF NOT EXISTS idx_departments_organization_id ON departments(organization_id);
CREATE INDEX IF NOT EXISTS idx_departments_parent_dept_id ON departments(parent_dept_id);
CREATE INDEX IF NOT EXISTS idx_departments_status ON departments(status);
CREATE INDEX IF NOT EXISTS idx_departments_deleted_at ON departments(deleted_at);

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
    deleted_at INTEGER,
    FOREIGN KEY (organization_id) REFERENCES organizations(id),
    FOREIGN KEY (department_id) REFERENCES departments(id),
    FOREIGN KEY (position_level_id) REFERENCES position_levels(id),
    FOREIGN KEY (created_by) REFERENCES users(id),
    FOREIGN KEY (updated_by) REFERENCES users(id)
);

INSERT INTO employees (
    id, emp_name, organization_id, department_id, position_level_id, position_title,
    hire_date, status, created_by, created_at, updated_by, updated_at, deleted_at
)
SELECT
    id, emp_name, organization_id, department_id, position_level_id, position_title,
    hire_date, status, created_by, created_at, updated_by, updated_at, deleted_at
FROM employees_old;

CREATE INDEX IF NOT EXISTS idx_employees_organization_id ON employees(organization_id);
CREATE INDEX IF NOT EXISTS idx_employees_department_id ON employees(department_id);
CREATE INDEX IF NOT EXISTS idx_employees_position_level_id ON employees(position_level_id);
CREATE INDEX IF NOT EXISTS idx_employees_status ON employees(status);
CREATE INDEX IF NOT EXISTS idx_employees_deleted_at ON employees(deleted_at);

DROP TABLE employees_old;
DROP TABLE departments_old;
DROP TABLE organizations_old;
