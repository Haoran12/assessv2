PRAGMA foreign_keys = ON;

UPDATE assessment_years
SET status = 'ended'
WHERE status = 'completed';

UPDATE assessment_periods
SET status = 'not_started'
WHERE status = 'preparing';

UPDATE assessment_periods
SET status = 'locked'
WHERE status = 'completed';
