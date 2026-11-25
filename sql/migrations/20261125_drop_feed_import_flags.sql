-- Migration: drop the import flags from feeds (step 2 of 2)
-- Date: 2026-11-25
-- Description: Removes import_vehicles and import_service_area, which were
-- replaced by feeds.data_type in 20261125_add_data_type_to_feeds.sql.
--
-- Run this only after every service that reads the feeds table uses data_type
-- (importer, service area importer, feed monitor and the dashboard API
-- endpoint /public/active_feeds). Older versions fail on the missing columns.

BEGIN;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'feeds'
          AND column_name = 'data_type'
    ) THEN
        RAISE EXCEPTION
            'feeds.data_type does not exist, run 20261125_add_data_type_to_feeds.sql first';
    END IF;
END
$$;

ALTER TABLE feeds
    DROP COLUMN IF EXISTS import_vehicles,
    DROP COLUMN IF EXISTS import_service_area;

COMMIT;
