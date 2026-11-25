-- Migration: track import attempts per feed
-- Date: 2026-11-25
-- Description: Lets the feed monitor record downtime for trips and service
-- area feeds, which are imported by an hourly job instead of continuously.
-- The trip importer and the service area importer write both columns; the
-- vehicle importer does not use them.
--
-- Run after 20261125_add_data_type_to_feeds.sql and before deploying the feed
-- monitor that reads these columns.

ALTER TABLE feeds
    ADD COLUMN IF NOT EXISTS last_time_import_attempted TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS import_failing_since TIMESTAMPTZ;

COMMENT ON COLUMN feeds.last_time_import_attempted IS
    'Last time an importer requested this feed, whether or not that succeeded';
COMMENT ON COLUMN feeds.import_failing_since IS
    'First failed import attempt since the last successful one; NULL while the last attempt succeeded';
