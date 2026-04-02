-- Migration: Create feed_downtime table for tracking feed uptime
-- Date: 2026-04-02
-- Description: Table to track when feeds go down and come back up

BEGIN;

-- Create feed_downtime table
CREATE TABLE IF NOT EXISTS feed_downtime (
    downtime_id SERIAL PRIMARY KEY,
    feed_id INTEGER NOT NULL REFERENCES feeds(feed_id) ON DELETE CASCADE,
    downtime_start TIMESTAMP WITH TIME ZONE NOT NULL,
    downtime_end TIMESTAMP WITH TIME ZONE,
    reason TEXT,
    notification_sent BOOLEAN DEFAULT FALSE,
    recovery_notification_sent BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create index on feed_id for faster lookups
CREATE INDEX IF NOT EXISTS idx_feed_downtime_feed_id ON feed_downtime(feed_id);

-- Create index on downtime_end to quickly find ongoing downtime
CREATE INDEX IF NOT EXISTS idx_feed_downtime_end_null ON feed_downtime(downtime_end) 
    WHERE downtime_end IS NULL;

-- Create index on start time for time-based queries
CREATE INDEX IF NOT EXISTS idx_feed_downtime_start ON feed_downtime(downtime_start);

-- Add comment to table
COMMENT ON TABLE feed_downtime IS 'Tracks feed downtime periods for vehicle import feeds';

-- Grant permissions (assuming same pattern as feeds table)
GRANT SELECT, INSERT, UPDATE ON TABLE feed_downtime TO dashboarddeelmobiliteit;
GRANT SELECT, USAGE ON SEQUENCE feed_downtime_downtime_id_seq TO dashboarddeelmobiliteit;

COMMIT;
