-- Restore assessment year name field.
ALTER TABLE assessment_years ADD COLUMN year_name VARCHAR(100) NOT NULL DEFAULT '';
UPDATE assessment_years
SET year_name = CAST(year AS TEXT)
WHERE TRIM(year_name) = '';
