ALTER TABLE assessment_session_periods
ADD COLUMN rule_binding_key VARCHAR(20) NOT NULL DEFAULT '';

UPDATE assessment_session_periods
SET rule_binding_key = period_code
WHERE TRIM(COALESCE(rule_binding_key, '')) = '';

CREATE INDEX IF NOT EXISTS idx_assessment_session_periods_rule_binding_key
    ON assessment_session_periods(assessment_id, rule_binding_key);
