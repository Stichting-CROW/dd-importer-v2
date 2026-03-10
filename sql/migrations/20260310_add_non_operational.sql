CREATE TABLE non_operational_event (
    non_operational_event_id SERIAL PRIMARY KEY,
    parking_event_id INTEGER NOT NULL,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP,
    FOREIGN KEY (parking_event_id) REFERENCES parking_event (parking_event_id)
);

CREATE INDEX idx_non_operational_event_parking_event_id
    ON non_operational_event (parking_event_id);