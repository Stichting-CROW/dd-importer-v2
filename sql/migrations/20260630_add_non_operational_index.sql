CREATE INDEX IF NOT EXISTS idx_non_operational_event_start_end
ON non_operational_event (start_time, end_time);
