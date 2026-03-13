PRAGMA foreign_keys = ON;

-- Normalize position_levels to new personal category system.
INSERT INTO position_levels (level_code, level_name, description, is_system, is_for_assessment, sort_order, status, created_at, updated_at)
SELECT 'leadership_main', '领导班子正职', '用于个人考核分类：领导班子正职', 1, 1, 1, 'active', strftime('%s', 'now'), strftime('%s', 'now')
WHERE NOT EXISTS (SELECT 1 FROM position_levels WHERE level_code = 'leadership_main');

INSERT INTO position_levels (level_code, level_name, description, is_system, is_for_assessment, sort_order, status, created_at, updated_at)
SELECT 'leadership_deputy', '领导班子副职', '用于个人考核分类：领导班子副职', 1, 1, 2, 'active', strftime('%s', 'now'), strftime('%s', 'now')
WHERE NOT EXISTS (SELECT 1 FROM position_levels WHERE level_code = 'leadership_deputy');

INSERT INTO position_levels (level_code, level_name, description, is_system, is_for_assessment, sort_order, status, created_at, updated_at)
SELECT 'department_main', '部门正职', '用于个人考核分类：部门正职', 1, 1, 3, 'active', strftime('%s', 'now'), strftime('%s', 'now')
WHERE NOT EXISTS (SELECT 1 FROM position_levels WHERE level_code = 'department_main');

INSERT INTO position_levels (level_code, level_name, description, is_system, is_for_assessment, sort_order, status, created_at, updated_at)
SELECT 'department_deputy', '部门副职', '用于个人考核分类：部门副职', 1, 1, 4, 'active', strftime('%s', 'now'), strftime('%s', 'now')
WHERE NOT EXISTS (SELECT 1 FROM position_levels WHERE level_code = 'department_deputy');

INSERT INTO position_levels (level_code, level_name, description, is_system, is_for_assessment, sort_order, status, created_at, updated_at)
SELECT 'general_management_personnel', '一般管理人员', '用于个人考核分类：一般管理人员', 1, 1, 5, 'active', strftime('%s', 'now'), strftime('%s', 'now')
WHERE NOT EXISTS (SELECT 1 FROM position_levels WHERE level_code = 'general_management_personnel');

UPDATE employees
SET position_level_id = (SELECT id FROM position_levels WHERE level_code = 'leadership_main')
WHERE position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'group_leader');

UPDATE employees
SET position_level_id = (SELECT id FROM position_levels WHERE level_code = 'leadership_deputy')
WHERE position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'company_leader');

UPDATE employees
SET position_level_id = (SELECT id FROM position_levels WHERE level_code = 'department_main')
WHERE position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'manager_main');

UPDATE employees
SET position_level_id = (SELECT id FROM position_levels WHERE level_code = 'department_deputy')
WHERE position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'manager_deputy');

UPDATE employees
SET position_level_id = (SELECT id FROM position_levels WHERE level_code = 'general_management_personnel')
WHERE position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'staff');

UPDATE employee_history
SET old_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'leadership_main')
WHERE old_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'group_leader');

UPDATE employee_history
SET old_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'leadership_deputy')
WHERE old_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'company_leader');

UPDATE employee_history
SET old_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'department_main')
WHERE old_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'manager_main');

UPDATE employee_history
SET old_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'department_deputy')
WHERE old_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'manager_deputy');

UPDATE employee_history
SET old_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'general_management_personnel')
WHERE old_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'staff');

UPDATE employee_history
SET new_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'leadership_main')
WHERE new_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'group_leader');

UPDATE employee_history
SET new_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'leadership_deputy')
WHERE new_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'company_leader');

UPDATE employee_history
SET new_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'department_main')
WHERE new_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'manager_main');

UPDATE employee_history
SET new_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'department_deputy')
WHERE new_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'manager_deputy');

UPDATE employee_history
SET new_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'general_management_personnel')
WHERE new_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'staff');

DELETE FROM position_levels WHERE level_code IN ('group_leader', 'company_leader', 'manager_main', 'manager_deputy', 'staff');

UPDATE position_levels
SET level_name = '领导班子正职',
    description = '用于个人考核分类：领导班子正职',
    is_system = 1,
    is_for_assessment = 1,
    sort_order = 1,
    status = 'active',
    updated_at = strftime('%s', 'now')
WHERE level_code = 'leadership_main';

UPDATE position_levels
SET level_name = '领导班子副职',
    description = '用于个人考核分类：领导班子副职',
    is_system = 1,
    is_for_assessment = 1,
    sort_order = 2,
    status = 'active',
    updated_at = strftime('%s', 'now')
WHERE level_code = 'leadership_deputy';

UPDATE position_levels
SET level_name = '部门正职',
    description = '用于个人考核分类：部门正职',
    is_system = 1,
    is_for_assessment = 1,
    sort_order = 3,
    status = 'active',
    updated_at = strftime('%s', 'now')
WHERE level_code = 'department_main';

UPDATE position_levels
SET level_name = '部门副职',
    description = '用于个人考核分类：部门副职',
    is_system = 1,
    is_for_assessment = 1,
    sort_order = 4,
    status = 'active',
    updated_at = strftime('%s', 'now')
WHERE level_code = 'department_deputy';

UPDATE position_levels
SET level_name = '一般管理人员',
    description = '用于个人考核分类：一般管理人员',
    is_system = 1,
    is_for_assessment = 1,
    sort_order = 5,
    status = 'active',
    updated_at = strftime('%s', 'now')
WHERE level_code = 'general_management_personnel';

-- Normalize assessment object categories.
UPDATE assessment_objects SET object_category = 'group_department' WHERE object_category = 'group_dept';
UPDATE assessment_objects SET object_category = 'subsidiary_company' WHERE object_category = 'company';
UPDATE assessment_objects SET object_category = 'subsidiary_company_department' WHERE object_category = 'company_dept';
UPDATE assessment_objects SET object_category = 'leadership_main' WHERE object_category = 'group_leader';
UPDATE assessment_objects SET object_category = 'leadership_deputy' WHERE object_category = 'company_leader';
UPDATE assessment_objects SET object_category = 'department_main' WHERE object_category = 'manager_main';
UPDATE assessment_objects SET object_category = 'department_deputy' WHERE object_category = 'manager_deputy';
UPDATE assessment_objects SET object_category = 'general_management_personnel' WHERE object_category = 'staff';

UPDATE assessment_rules SET object_category = 'group_department' WHERE object_category = 'group_dept';
UPDATE assessment_rules SET object_category = 'subsidiary_company' WHERE object_category = 'company';
UPDATE assessment_rules SET object_category = 'subsidiary_company_department' WHERE object_category = 'company_dept';
UPDATE assessment_rules SET object_category = 'leadership_main' WHERE object_category = 'group_leader';
UPDATE assessment_rules SET object_category = 'leadership_deputy' WHERE object_category = 'company_leader';
UPDATE assessment_rules SET object_category = 'department_main' WHERE object_category = 'manager_main';
UPDATE assessment_rules SET object_category = 'department_deputy' WHERE object_category = 'manager_deputy';
UPDATE assessment_rules SET object_category = 'general_management_personnel' WHERE object_category = 'staff';

UPDATE rule_templates SET object_category = 'group_department' WHERE object_category = 'group_dept';
UPDATE rule_templates SET object_category = 'subsidiary_company' WHERE object_category = 'company';
UPDATE rule_templates SET object_category = 'subsidiary_company_department' WHERE object_category = 'company_dept';
UPDATE rule_templates SET object_category = 'leadership_main' WHERE object_category = 'group_leader';
UPDATE rule_templates SET object_category = 'leadership_deputy' WHERE object_category = 'company_leader';
UPDATE rule_templates SET object_category = 'department_main' WHERE object_category = 'manager_main';
UPDATE rule_templates SET object_category = 'department_deputy' WHERE object_category = 'manager_deputy';
UPDATE rule_templates SET object_category = 'general_management_personnel' WHERE object_category = 'staff';
