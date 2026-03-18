ALTER TABLE users ADD COLUMN real_name VARCHAR(100) NOT NULL DEFAULT '';
UPDATE users
SET real_name = username
WHERE TRIM(real_name) = '';
