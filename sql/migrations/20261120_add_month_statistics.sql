-- Monthly statistics for trip-derived KPIs that need extra dimensions.
--
-- The indicator determines what `category` means:
--   indicator 13 (count_ended_rentals_by_distance_and_time_period):
--     straight-line CBS distance category 0-9
--   indicator 14 (count_ended_rentals_by_duration_and_time_period):
--     rental duration category 0-11
--   indicator 15 (count_ended_rentals_by_real_distance_and_time_period):
--     distance_over_road CBS distance category 0-9
--
-- Distance categories (km):
--   0 = 0.0 <= d < 0.1
--   1 = 0.1 <= d < 1
--   2 = 1 <= d < 3.7
--   3 = 3.7 <= d < 7.5
--   4 = 7.5 <= d < 15
--   5 = 15 <= d < 30
--   6 = 30 <= d < 50
--   7 = 50 <= d < 75
--   8 = 75 <= d < 500
--   9 = d >= 500
--
-- Duration categories:
--   0  = d < 1 minute
--   1  = 1 minute <= d < 5 minutes
--   2  = 5 minutes <= d < 10 minutes
--   3  = 10 minutes <= d < 15 minutes
--   4  = 15 minutes <= d < 20 minutes
--   5  = 20 minutes <= d < 30 minutes
--   6  = 30 minutes <= d < 60 minutes
--   7  = 1 hour <= d < 2 hours
--   8  = 2 hours <= d < 1 day
--   9  = 1 day <= d < 1 week
--   10 = 1 week <= d < 4 weeks
--   11 = d >= 4 weeks
--
-- Time periods are based on the rental end time in Europe/Amsterdam:
--   0 = 00:00 <= t < 07:00
--   1 = 07:00 <= t < 09:00
--   2 = 09:00 <= t < 12:00
--   3 = 12:00 <= t < 16:00
--   4 = 16:00 <= t < 18:00
--   5 = 18:00 <= t < 24:00
CREATE TABLE IF NOT EXISTS month_statistics (
    month        DATE NOT NULL,
    indicator    SMALLINT NOT NULL,
    geometry_ref VARCHAR NOT NULL,
    system_id    VARCHAR NOT NULL,
    vehicle_type VARCHAR NOT NULL,
    trip_source  VARCHAR NOT NULL CHECK (trip_source IN ('vehicles', 'trips')),
    category     SMALLINT NOT NULL CHECK (category BETWEEN 0 AND 11),
    time_period  SMALLINT NOT NULL CHECK (time_period BETWEEN 0 AND 5),
    value        NUMERIC NOT NULL
);

CREATE UNIQUE INDEX idx_month_statistics_pkey
ON month_statistics (month, indicator, geometry_ref, system_id, vehicle_type, trip_source, category, time_period);

CREATE INDEX idx_month_statistics_geometry_month
ON month_statistics (geometry_ref, month, indicator, system_id, vehicle_type, trip_source)
INCLUDE (category, time_period, value);

CREATE INDEX idx_month_statistics_system_month
ON month_statistics (system_id, month, indicator, geometry_ref, vehicle_type, trip_source)
INCLUDE (category, time_period, value);
