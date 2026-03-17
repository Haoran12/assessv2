PRAGMA foreign_keys = ON;

UPDATE assessment_years
SET status = 'completed'
WHERE status = 'ended';

UPDATE assessment_periods
SET status = 'preparing'
WHERE status = 'not_started';

UPDATE assessment_periods
SET status = 'completed'
WHERE status IN ('ended', 'locked');
