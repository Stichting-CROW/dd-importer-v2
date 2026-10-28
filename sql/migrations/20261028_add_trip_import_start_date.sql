-- Migration: add a per-feed start date for the trip importer
-- Date: 2026-10-28
-- Description: Optional earliest hour to backfill for a trip feed. When NULL,
-- the global TRIP_IMPORT_START_DATE (or its default) is used.

ALTER TABLE feeds
ADD COLUMN IF NOT EXISTS trip_import_start_date TIMESTAMPTZ;

COMMENT ON COLUMN feeds.trip_import_start_date IS
    'Earliest trip import timestamp for this feed; NULL uses the global default';
