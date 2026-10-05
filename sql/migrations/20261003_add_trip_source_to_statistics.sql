-- Add a trip_source flag to the aggregated statistics.
--
-- trip_source is NULL for statistics that do not depend on trip data
-- (park-event based indicators). For trip-based indicators it holds
-- 'vehicles' (trips derived from vehicle/park events) or 'trips' (trips
-- imported from an MDS trips feed). Source-specific statistics are only
-- produced for operators that actually have trips of that source.
--
-- A plain VARCHAR + CHECK is used instead of the existing `trip_source`
-- enum because DuckDB's postgres extension has unreliable support for
-- Postgres enum columns (duckdb-postgres#379). The value set is identical.
--
-- A primary key cannot contain NULL, so the existing primary keys are
-- replaced by two partial unique indexes: one for rows without a source
-- and one for rows with a source.

ALTER TABLE moment_statistics ADD COLUMN trip_source VARCHAR;
ALTER TABLE moment_statistics
    ADD CONSTRAINT moment_statistics_trip_source_check
    CHECK (trip_source IN ('vehicles', 'trips'));

ALTER TABLE moment_statistics DROP CONSTRAINT moment_statistics_pkey;
CREATE UNIQUE INDEX moment_statistics_pkey_null
    ON moment_statistics (date, measurement_moment, indicator, geometry_ref, system_id, vehicle_type)
    WHERE trip_source IS NULL;
CREATE UNIQUE INDEX moment_statistics_pkey_source
    ON moment_statistics (date, measurement_moment, indicator, geometry_ref, system_id, vehicle_type, trip_source)
    WHERE trip_source IS NOT NULL;

ALTER TABLE day_statistics ADD COLUMN trip_source VARCHAR;
ALTER TABLE day_statistics
    ADD CONSTRAINT day_statistics_trip_source_check
    CHECK (trip_source IN ('vehicles', 'trips'));

ALTER TABLE day_statistics DROP CONSTRAINT day_statistics_pkey;
CREATE UNIQUE INDEX day_statistics_pkey_null
    ON day_statistics (date, indicator, geometry_ref, system_id, vehicle_type)
    WHERE trip_source IS NULL;
CREATE UNIQUE INDEX day_statistics_pkey_source
    ON day_statistics (date, indicator, geometry_ref, system_id, vehicle_type, trip_source)
    WHERE trip_source IS NOT NULL;

-- Add trip_source to the covering indexes so consumers can filter on it.
DROP INDEX IF EXISTS idx_moment_statistics_covering;
CREATE INDEX idx_moment_statistics_covering
    ON moment_statistics (geometry_ref, indicator, system_id, vehicle_type, trip_source, measurement_moment, date)
    INCLUDE (value);

DROP INDEX IF EXISTS idx_day_statistics_geometry_date;
DROP INDEX IF EXISTS idx_day_statistics_system_date;
CREATE INDEX idx_day_statistics_geometry_date
    ON day_statistics (geometry_ref, date, indicator, system_id, vehicle_type, trip_source)
    INCLUDE (value);
CREATE INDEX idx_day_statistics_system_date
    ON day_statistics (system_id, date, indicator, geometry_ref, vehicle_type, trip_source)
    INCLUDE (value);
