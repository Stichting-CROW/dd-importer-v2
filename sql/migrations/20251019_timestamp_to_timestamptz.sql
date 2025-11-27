BEGIN;

DROP MATERIALIZED VIEW IF EXISTS park_event_on_date;
DROP INDEX IF EXISTS park_events_ended_less_than_three_days_ago;

ALTER TABLE park_events
ALTER COLUMN start_time TYPE timestamptz,
ALTER COLUMN end_time TYPE timestamptz;

ALTER TABLE trips
ALTER COLUMN start_time TYPE timestamptz,
ALTER COLUMN end_time TYPE timestamptz;

-- Only relevant when you deploy this within the Netherlands
-- If you deploy this something else some custom work should be done;
SET TIME ZONE 'Europe/Amsterdam';
ALTER DATABASE dashboarddeelmobiliteit SET timezone TO 'Europe/Amsterdam';

CREATE MATERIALIZED VIEW park_event_on_date AS (
	SELECT on_date, ARRAY_AGG(park_event_id) AS park_event_ids 
	FROM (
		SELECT park_event_id, generate_series(start_time::date, COALESCE(end_time::date, NOW()::date), '1 day'::interval) 
		AS on_date 
		FROM park_events
	) as q1
GROUP BY on_date);

CREATE INDEX park_event_on_date_index ON park_event_on_date (on_date);

COMMIT;