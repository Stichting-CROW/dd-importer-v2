-- Add a trip_on_date materialized view, analogous to park_event_on_date.
--
-- Trips are only ever queried by their end_time, so it maps each end date to
-- the trip_ids that ended that day. This lets the API narrow historical trip
-- queries using the view instead of scanning the whole trips table. The view
-- is refreshed and the recent partial indexes on trips are rebuilt by
-- batch_aggregation.

BEGIN;

DROP INDEX IF EXISTS idx_trips_bike_time_after_2026_01_12;

CREATE MATERIALIZED VIEW IF NOT EXISTS trip_on_date AS (
	SELECT end_time::date AS on_date, ARRAY_AGG(trip_id) AS trip_ids
	FROM trips
	GROUP BY end_time::date
);

CREATE INDEX IF NOT EXISTS trip_on_date_index ON trip_on_date (on_date);

COMMIT;
