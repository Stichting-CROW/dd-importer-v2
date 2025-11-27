CREATE MATERIALIZED VIEW park_event_on_date AS (
	SELECT on_date, ARRAY_AGG(park_event_id) AS park_event_ids 
	FROM (
		SELECT park_event_id, generate_series(start_time::date, COALESCE(end_time::date, NOW()::date), '1 day'::interval) 
		AS on_date 
		FROM park_events
	) as q1
GROUP BY on_date);

CREATE INDEX park_event_on_date_index ON park_event_on_date (on_date);
