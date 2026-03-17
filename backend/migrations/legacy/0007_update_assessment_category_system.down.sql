PRAGMA foreign_keys = ON;

-- Best-effort rollback (lossy for new team categories that had no legacy equivalent).
INSERT INTO position_levels (level_code, level_name, description, is_system, is_for_assessment, sort_order, status, created_at, updated_at)
SELECT 'group_leader', '集团高层领导', '集团领导班子成员', 1, 1, 1, 'active', strftime('%s', 'now'), strftime('%s', 'now')
WHERE NOT EXISTS (SELECT 1 FROM position_levels WHERE level_code = 'group_leader');

INSERT INTO position_levels (level_code, level_name, description, is_system, is_for_assessment, sort_order, status, created_at, updated_at)
SELECT 'company_leader', '权属企业高层领导', '企业领导班子成员', 1, 1, 2, 'active', strftime('%s', 'now'), strftime('%s', 'now')
WHERE NOT EXISTS (SELECT 1 FROM position_levels WHERE level_code = 'company_leader');

INSERT INTO position_levels (level_code, level_name, description, is_system, is_for_assessment, sort_order, status, created_at, updated_at)
SELECT 'manager_main', '正职管理人员', '部门正职管理人员', 1, 1, 3, 'active', strftime('%s', 'now'), strftime('%s', 'now')
WHERE NOT EXISTS (SELECT 1 FROM position_levels WHERE level_code = 'manager_main');

INSERT INTO position_levels (level_code, level_name, description, is_system, is_for_assessment, sort_order, status, created_at, updated_at)
SELECT 'manager_deputy', '副职管理人员', '部门副职管理人员', 1, 1, 4, 'active', strftime('%s', 'now'), strftime('%s', 'now')
WHERE NOT EXISTS (SELECT 1 FROM position_levels WHERE level_code = 'manager_deputy');

INSERT INTO position_levels (level_code, level_name, description, is_system, is_for_assessment, sort_order, status, created_at, updated_at)
SELECT 'staff', '一般人员', '普通员工', 1, 1, 5, 'active', strftime('%s', 'now'), strftime('%s', 'now')
WHERE NOT EXISTS (SELECT 1 FROM position_levels WHERE level_code = 'staff');

UPDATE employees
SET position_level_id = (SELECT id FROM position_levels WHERE level_code = 'group_leader')
WHERE position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'leadership_main');

UPDATE employees
SET position_level_id = (SELECT id FROM position_levels WHERE level_code = 'company_leader')
WHERE position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'leadership_deputy');

UPDATE employees
SET position_level_id = (SELECT id FROM position_levels WHERE level_code = 'manager_main')
WHERE position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'department_main');

UPDATE employees
SET position_level_id = (SELECT id FROM position_levels WHERE level_code = 'manager_deputy')
WHERE position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'department_deputy');

UPDATE employees
SET position_level_id = (SELECT id FROM position_levels WHERE level_code = 'staff')
WHERE position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'general_management_personnel');

UPDATE employee_history
SET old_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'group_leader')
WHERE old_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'leadership_main');

UPDATE employee_history
SET old_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'company_leader')
WHERE old_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'leadership_deputy');

UPDATE employee_history
SET old_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'manager_main')
WHERE old_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'department_main');

UPDATE employee_history
SET old_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'manager_deputy')
WHERE old_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'department_deputy');

UPDATE employee_history
SET old_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'staff')
WHERE old_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'general_management_personnel');

UPDATE employee_history
SET new_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'group_leader')
WHERE new_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'leadership_main');

UPDATE employee_history
SET new_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'company_leader')
WHERE new_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'leadership_deputy');

UPDATE employee_history
SET new_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'manager_main')
WHERE new_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'department_main');

UPDATE employee_history
SET new_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'manager_deputy')
WHERE new_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'department_deputy');

UPDATE employee_history
SET new_position_level_id = (SELECT id FROM position_levels WHERE level_code = 'staff')
WHERE new_position_level_id IN (SELECT id FROM position_levels WHERE level_code = 'general_management_personnel');

DELETE FROM position_levels
WHERE level_code IN ('leadership_main', 'leadership_deputy', 'department_main', 'department_deputy', 'general_management_personnel');

UPDATE assessment_objects SET object_category = 'group_dept' WHERE object_category = 'group_department';
UPDATE assessment_objects SET object_category = 'company' WHERE object_category IN ('group', 'group_leadership_team', 'subsidiary_company', 'subsidiary_company_leadership_team');
UPDATE assessment_objects SET object_category = 'company_dept' WHERE object_category = 'subsidiary_company_department';
UPDATE assessment_objects SET object_category = 'group_leader' WHERE object_category = 'leadership_main';
UPDATE assessment_objects SET object_category = 'company_leader' WHERE object_category = 'leadership_deputy';
UPDATE assessment_objects SET object_category = 'manager_main' WHERE object_category = 'department_main';
UPDATE assessment_objects SET object_category = 'manager_deputy' WHERE object_category = 'department_deputy';
UPDATE assessment_objects SET object_category = 'staff' WHERE object_category = 'general_management_personnel';

UPDATE assessment_rules SET object_category = 'group_dept' WHERE object_category = 'group_department';
UPDATE assessment_rules SET object_category = 'company' WHERE object_category IN ('group', 'group_leadership_team', 'subsidiary_company', 'subsidiary_company_leadership_team');
UPDATE assessment_rules SET object_category = 'company_dept' WHERE object_category = 'subsidiary_company_department';
UPDATE assessment_rules SET object_category = 'group_leader' WHERE object_category = 'leadership_main';
UPDATE assessment_rules SET object_category = 'company_leader' WHERE object_category = 'leadership_deputy';
UPDATE assessment_rules SET object_category = 'manager_main' WHERE object_category = 'department_main';
UPDATE assessment_rules SET object_category = 'manager_deputy' WHERE object_category = 'department_deputy';
UPDATE assessment_rules SET object_category = 'staff' WHERE object_category = 'general_management_personnel';

UPDATE rule_templates SET object_category = 'group_dept' WHERE object_category = 'group_department';
UPDATE rule_templates SET object_category = 'company' WHERE object_category IN ('group', 'group_leadership_team', 'subsidiary_company', 'subsidiary_company_leadership_team');
UPDATE rule_templates SET object_category = 'company_dept' WHERE object_category = 'subsidiary_company_department';
UPDATE rule_templates SET object_category = 'group_leader' WHERE object_category = 'leadership_main';
UPDATE rule_templates SET object_category = 'company_leader' WHERE object_category = 'leadership_deputy';
UPDATE rule_templates SET object_category = 'manager_main' WHERE object_category = 'department_main';
UPDATE rule_templates SET object_category = 'manager_deputy' WHERE object_category = 'department_deputy';
UPDATE rule_templates SET object_category = 'staff' WHERE object_category = 'general_management_personnel';
