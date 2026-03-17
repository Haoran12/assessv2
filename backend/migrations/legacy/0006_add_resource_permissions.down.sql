-- Rollback resource-level permission control

ALTER TABLE assessment_years DROP COLUMN permission_mode;
ALTER TABLE assessment_rules DROP COLUMN permission_mode;
ALTER TABLE rule_templates DROP COLUMN permission_mode;
ALTER TABLE direct_scores DROP COLUMN permission_mode;
ALTER TABLE extra_points DROP COLUMN permission_mode;
ALTER TABLE score_modules DROP COLUMN permission_mode;
ALTER TABLE vote_groups DROP COLUMN permission_mode;
ALTER TABLE organizations DROP COLUMN permission_mode;
ALTER TABLE departments DROP COLUMN permission_mode;
ALTER TABLE employees DROP COLUMN permission_mode;
