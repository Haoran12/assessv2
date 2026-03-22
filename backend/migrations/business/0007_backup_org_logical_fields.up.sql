ALTER TABLE backups
ADD COLUMN content_type VARCHAR(20) NOT NULL DEFAULT 'full_snapshot';

ALTER TABLE backups
ADD COLUMN scope_type VARCHAR(20) NOT NULL DEFAULT 'global';

ALTER TABLE backups
ADD COLUMN scope_org_id INTEGER;

ALTER TABLE backups
ADD COLUMN format_version VARCHAR(20);

ALTER TABLE backups
ADD COLUMN checksum_sha256 VARCHAR(64);

ALTER TABLE backups
ADD COLUMN manifest_json TEXT;

UPDATE backups
SET content_type = 'full_snapshot'
WHERE TRIM(COALESCE(content_type, '')) = '';

UPDATE backups
SET scope_type = 'global'
WHERE TRIM(COALESCE(scope_type, '')) = '';

CREATE INDEX IF NOT EXISTS idx_backups_content_type ON backups(content_type);
CREATE INDEX IF NOT EXISTS idx_backups_scope_type ON backups(scope_type);
CREATE INDEX IF NOT EXISTS idx_backups_scope_org_id ON backups(scope_org_id);
