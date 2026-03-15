CREATE TABLE non_operational_event (
    non_operational_event_id SERIAL PRIMARY KEY,
    park_event_id INTEGER NOT NULL,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ,
    FOREIGN KEY (park_event_id) REFERENCES park_events (park_event_id)
);

CREATE INDEX idx_non_operational_event_park_event_id
    ON non_operational_event (park_event_id);