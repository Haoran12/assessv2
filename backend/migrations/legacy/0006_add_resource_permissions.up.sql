-- Add resource-level permission control fields
-- Permission mode format: 0XXX (octal-like), where each digit represents owner/group/others permissions
-- Each digit is a 4-bit value: RWDX (Read/Write/Delete/Execute)
-- Examples:
--   0644 = Owner(RW), Group(R), Others(R)
--   0600 = Owner(RW), Group(none), Others(none)
--   0754 = Owner(RWDX), Group(RX), Others(R)

-- Assessment years: Owner can modify, all can read
ALTER TABLE assessment_years ADD COLUMN permission_mode SMALLINT NOT NULL DEFAULT 420;  -- 0644 in decimal

-- Assessment rules: Owner can modify, all can read
ALTER TABLE assessment_rules ADD COLUMN permission_mode SMALLINT NOT NULL DEFAULT 420;  -- 0644

-- Rule templates: Owner can modify, all can read
ALTER TABLE rule_templates ADD COLUMN permission_mode SMALLINT NOT NULL DEFAULT 420;  -- 0644

-- Direct scores: Only owner can read/write (sensitive data)
ALTER TABLE direct_scores ADD COLUMN permission_mode SMALLINT NOT NULL DEFAULT 384;  -- 0600

-- Extra points: Owner can modify, group can read
ALTER TABLE extra_points ADD COLUMN permission_mode SMALLINT NOT NULL DEFAULT 416;  -- 0640

-- Score modules: Owner can modify, all can read
ALTER TABLE score_modules ADD COLUMN permission_mode SMALLINT NOT NULL DEFAULT 420;  -- 0644

-- Vote groups: Owner can modify, all can read
ALTER TABLE vote_groups ADD COLUMN permission_mode SMALLINT NOT NULL DEFAULT 420;  -- 0644

-- Organizations: Owner can modify, all can read
ALTER TABLE organizations ADD COLUMN permission_mode SMALLINT NOT NULL DEFAULT 420;  -- 0644

-- Departments: Owner can modify, all can read
ALTER TABLE departments ADD COLUMN permission_mode SMALLINT NOT NULL DEFAULT 420;  -- 0644

-- Employees: Owner can modify, group can read
ALTER TABLE employees ADD COLUMN permission_mode SMALLINT NOT NULL DEFAULT 416;  -- 0640
