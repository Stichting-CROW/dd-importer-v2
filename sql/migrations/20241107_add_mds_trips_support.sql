-- migration script to add support for MDS /trips importing

CREATE TYPE trip_source AS ENUM ('vehicles', 'trips');

ALTER TABLE trips ADD COLUMN source_feed_id INT;
ALTER TABLE trips ADD COLUMN trip_source trip_source DEFAULT 'vehicles';
ALTER TABLE trips ADD COLUMN distance_over_road INT;