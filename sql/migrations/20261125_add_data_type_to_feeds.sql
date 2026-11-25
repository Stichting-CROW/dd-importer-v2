-- Migration: add data_type to feeds (step 1 of 2)
-- Date: 2026-11-25
-- Description: Every feed row now delivers exactly one kind of data, stored in
-- feeds.data_type ('vehicles', 'trips' or 'service_areas'). This replaces the
-- import_vehicles / import_service_area flags and the convention that trips
-- feeds are recognised by their feed_type.
--
-- Existing rows are converted as follows:
--   feed_type mds-trips-v1 / mds-trips-v2            -> 'trips'
--   import_vehicles AND import_service_area          -> split into two rows: the
--                                                       existing row (same feed_id)
--                                                       becomes 'vehicles', a new
--                                                       row becomes 'service_areas'
--   only import_vehicles                             -> 'vehicles'
--   only import_service_area                         -> 'service_areas'
--   neither flag (and not a trips feed)              -> 'vehicles' with
--                                                       is_active = false, because
--                                                       nothing imported it before
--
-- Safe to run while services that still read the old flags are deployed:
-- import_vehicles and import_service_area stay available as read-only columns
-- derived from data_type. Remove them with
-- 20261125_drop_feed_import_flags.sql once every service uses data_type.
--
-- Re-running this migration fails on CREATE TYPE and rolls back, changing nothing.

BEGIN;

CREATE TYPE feed_data_type AS ENUM ('vehicles', 'trips', 'service_areas');

ALTER TABLE feeds ADD COLUMN data_type feed_data_type;

UPDATE feeds
SET data_type = CASE
        WHEN feed_type IN ('mds-trips-v1', 'mds-trips-v2') THEN 'trips'
        WHEN import_vehicles IS TRUE THEN 'vehicles'
        WHEN import_service_area IS TRUE THEN 'service_areas'
        ELSE 'vehicles'
    END::feed_data_type;

-- Feeds that no importer picked up keep that behaviour by being deactivated.
UPDATE feeds
SET is_active = FALSE,
    remarks = CONCAT_WS(' | ', NULLIF(remarks, ''),
        'Deactivated by data_type migration: import_vehicles and import_service_area were both off')
WHERE data_type = 'vehicles'
  AND import_vehicles IS NOT TRUE
  AND is_active IS TRUE;

-- Split feeds that were used for both vehicles and service areas. The existing
-- row keeps its feed_id (referenced by feed_downtime) and stays the vehicles feed.
INSERT INTO feeds (
    system_id, feed_url, feed_type, import_strategy, authentication,
    last_time_updated, request_headers, default_vehicle_type, is_active,
    remarks, data_type
)
SELECT
    system_id, feed_url, feed_type, import_strategy, authentication,
    last_time_updated, request_headers, default_vehicle_type, is_active,
    CONCAT_WS(' | ', NULLIF(remarks, ''),
        'Split off from feed ' || feed_id || ' for service area import'),
    'service_areas'
FROM feeds
WHERE data_type = 'vehicles'
  AND import_vehicles IS TRUE
  AND import_service_area IS TRUE
ORDER BY feed_id;

ALTER TABLE feeds
    ALTER COLUMN data_type SET DEFAULT 'vehicles',
    ALTER COLUMN data_type SET NOT NULL;

-- Keep the old flags readable for services that are not updated yet, but make
-- data_type the single source of truth.
ALTER TABLE feeds
    DROP COLUMN import_vehicles,
    DROP COLUMN import_service_area;

ALTER TABLE feeds
    ADD COLUMN import_vehicles BOOLEAN
        GENERATED ALWAYS AS (data_type = 'vehicles') STORED,
    ADD COLUMN import_service_area BOOLEAN
        GENERATED ALWAYS AS (data_type = 'service_areas') STORED;

COMMENT ON COLUMN feeds.data_type IS
    'Kind of data this feed delivers: vehicles, trips or service_areas';
COMMENT ON COLUMN feeds.import_vehicles IS
    'Deprecated: derived from data_type, use data_type instead';
COMMENT ON COLUMN feeds.import_service_area IS
    'Deprecated: derived from data_type, use data_type instead';

COMMENT ON TABLE feed_downtime IS
    'Tracks downtime periods per feed; feeds.data_type tells which kind of data the feed delivers';

COMMIT;
