CREATE EXTENSION postgis;

CREATE TYPE trip_source AS ENUM ('vehicles', 'trips');

CREATE table trips (
    trip_id                 INT GENERATED BY DEFAULT AS IDENTITY,
    system_id               VARCHAR(255),
    bike_id                 VARCHAR(255),
    start_location          GEOMETRY,
    end_location            GEOMETRY, 
    start_time              TIMESTAMP,
    end_time                TIMESTAMP,
    vehicle_type_id         INT,
    source_feed_id          INT,
    trip_source             trip_source DEFAULT 'vehicles',
    distance_over_road      INT,
    PRIMARY KEY(trip_id)
);

CREATE INDEX start_time_trips
	ON trips (start_time);

CREATE INDEX end_time_trips
	ON trips (end_time);

CREATE INDEX trips_vehicle_type_id
	ON trips (vehicle_type_id);

CREATE INDEX start_time_end_time_trip
    ON trips (start_time, end_time);

CREATE TABLE park_events (
    park_event_id  			INT GENERATED BY DEFAULT AS IDENTITY,
	system_id               VARCHAR(255),
    bike_id                 VARCHAR(255),
    location				GEOMETRY,
    start_time              TIMESTAMP,
    end_time                TIMESTAMP,
    check_in_sample_id      INT,
    check_out_sample_id     INT,
    vehicle_type_id         INT,
    PRIMARY KEY(park_event_id)
);

CREATE INDEX  
    ON park_events(bike_id);

CREATE INDEX
    ON park_events(end_time);

CREATE INDEX park_event_vehicle_type_id
    ON park_events (vehicle_type_id); 

CREATE INDEX
    ON park_events (start_time, end_time);

CREATE INDEX
    ON park_events (start_time);

CREATE INDEX system_id ON public.park_events USING btree (system_id);
CREATE INDEX trip_not_completed ON public.park_events USING btree (end_time) WHERE (end_time IS NULL);

CREATE TABLE zones (
    zone_id INT GENERATED BY DEFAULT AS IDENTITY,
    area    GEOMETRY,
    name    VARCHAR(255),
    owner        VARCHAR(255),
    municipality VARCHAR(255),
    zone_type    VARCHAR(255),
    stats_ref    VARCHAR(40),
    PRIMARY KEY(zone_id)
);

CREATE TABLE geographies (
	geography_id UUID NOT NULL,
	zone_id INT REFERENCES zones(zone_id),
	name VARCHAR(255) NOT NULL,
	description VARCHAR(255),
	geography_type VARCHAR(255),
	effective_date timestamptz,
	published_date timestamptz NOT NULL,
	retire_date timestamptz,
	prev_geographies UUID ARRAY,
	publish BOOLEAN NOT NULL,
    PRIMARY KEY (geography_id)
);

CREATE TABLE stops (
	stop_id UUID NOT NULL,
	name VARCHAR(255) NOT NULL,
	location GEOMETRY NOT NULL,
	status JSONB NOT NULL,
	capacity JSONB NOT NULL,
	geography_id UUID REFERENCES geographies(geography_id)
);

CREATE TABLE no_parking_policy(
	geography_id UUID REFERENCES geographies(geography_id),
	start_date TIMESTAMP NOT NULL,
	end_date TIMESTAMP
);

CREATE TABLE policies (
	policy_id UUID NOT NULL,
	start_date TIMESTAMP NOT NULL,
	end_date TIMESTAMP,
	published_date TIMESTAMP NOT NULL,
	rules JSONB NOT NULL,
	gm_code VARCHAR(255) NOT NULL,
	geography_ref UUID NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL
);


CREATE INDEX municipality ON zones (zone_type, municipality);
CREATE INDEX zones_area_index ON public.zones USING gist (area);

CREATE TABLE acl (
    username VARCHAR(255),
    filter_municipality BOOLEAN,
    filter_operator BOOLEAN,
    is_admin BOOLEAN,
    is_contact_person_municipality BOOLEAN,
    PRIMARY KEY(username)
);

CREATE TABLE acl_municipalities (
    username VARCHAR(255) REFERENCES acl(username),
    municipality VARCHAR(255)
);

CREATE TABLE acl_operator (
    username VARCHAR(255) REFERENCES acl(username),
    operator VARCHAR(255)
);

CREATE TABLE municipalities_with_data (
    name VARCHAR(255),
    municipality VARCHAR(255) 
);

CREATE TABLE feeds (
     feed_id                            INT GENERATED BY DEFAULT AS IDENTITY, 
     system_id                          VARCHAR(255) NOT NULL,
     feed_url                           TEXT NOT NULL,
     feed_type                          VARCHAR(50) NOT NULL,
     import_strategy                    VARCHAR(50) NOT NULL,
     authentication                     JSON,
     last_time_updated                  TIMESTAMP,
     request_headers                    JSON,
     default_vehicle_type               INT,
     is_active                          BOOLEAN DEFAULT TRUE,
     import_vehicles                    BOOLEAN DEFAULT TRUE,
     import_service_area                BOOLEAN DEFAULT FALSE,
     last_time_succesfully_imported     TIMESTAMPTZ,
     ignore_disruptions_in_feed_until   TIMESTAMPTZ
);

-- Migratie:
-- ALTER TABLE feeds
-- ADD request_headers JSON; 

CREATE TABLE stats_pre_process (
     date             DATE NOT NULL,
     zone_ref         VARCHAR(255) NOT NULL, 
     stat_description VARCHAR(255) NOT NULL,
     system_id        VARCHAR(255),
     value            NUMERIC,
     UNIQUE(date, zone_ref, stat_description, system_id)
);

CREATE INDEX stats_index 
ON stats_pre_process (stat_description, zone_ref, date);

CREATE TABLE audit_log (
     username           VARCHAR(255),
     timestamp_accessed TIME,
     raw_api_call       TEXT,
     filter_active      TEXT 
);

CREATE TABLE vehicle_type (
    vehicle_type_id              INT GENERATED BY DEFAULT AS IDENTITY,
    external_vehicle_type_id     VARCHAR(255) NOT NULL,
    form_factor                  VARCHAR(50) NOT NULL,
    propulsion_type              VARCHAR(50),
    max_permitted_speed          SMALLINT,
    system_id                    VARCHAR(255),
    name                         VARCHAR(255),
    icon_url                     TEXT
);

-- insert default vehicle types
INSERT INTO public.vehicle_type (vehicle_type_id, external_vehicle_type_id, form_factor, propulsion_type, max_permitted_speed, system_id, name, icon_url) VALUES (1, 'DEFAULT_MOPED', 'moped', 'electric', NULL, NULL, 'default scooter', NULL);
INSERT INTO public.vehicle_type (vehicle_type_id, external_vehicle_type_id, form_factor, propulsion_type, max_permitted_speed, system_id, name, icon_url) VALUES (2, 'DEFAULT_CARGO_BICYCLE', 'cargo_bicycle', 'electric_assist', NULL, NULL, 'default cargo bicycle', NULL);
INSERT INTO public.vehicle_type (vehicle_type_id, external_vehicle_type_id, form_factor, propulsion_type, max_permitted_speed, system_id, name, icon_url) VALUES (3, 'DEFAULT_BIKE', 'bicycle', NULL, NULL, NULL, 'default bicycle (electric and non electric)', NULL);
INSERT INTO public.vehicle_type (vehicle_type_id, external_vehicle_type_id, form_factor, propulsion_type, max_permitted_speed, system_id, name, icon_url) VALUES (4, 'DEFAULT_ELECTRIC_BIKE', 'bicycle', 'electric_assist', NULL, NULL, 'default electric bicycle', NULL);
INSERT INTO public.vehicle_type (vehicle_type_id, external_vehicle_type_id, form_factor, propulsion_type, max_permitted_speed, system_id, name, icon_url) VALUES (5, 'DEFAULT_NORMAL_BIKE', 'bicycle', 'human', NULL, NULL, 'default normal bicycle', NULL);

CREATE TABLE feed_status_logs (
    feed_status_log_id  INT GENERATED BY DEFAULT AS IDENTITY,
    feed_id             INT NOT NULL,
    created_at          TIMESTAMP,
    log_type            VARCHAR(20) NOT NULL,
    log_message         VARCHAR(255),
    is_feed_broken      BOOLEAN NOT NULL
);

CREATE INDEX feed_status_logs_feed_id
    ON feed_status_logs (feed_id);

CREATE INDEX feed_status_logs_created_at
    ON feed_status_logs (created_at);

CREATE TABLE feed_status (
    feed_id             INT NOT NULL,
    last_time_updated   TIMESTAMP NOT NULL,
    number_of_vehicles  INT NOT NULL
);

CREATE TABLE active_user_stats (
    user_hash           VARCHAR(50),
    role                VARCHAR(50),
    active_on           DATE,
    CONSTRAINT active_user_on_date UNIQUE (user_hash, role, active_on)
);

CREATE INDEX active_on_user_stats
    ON active_user_stats (active_on);

CREATE TABLE service_area (
    service_area_version_id SERIAL PRIMARY KEY,
    municipality TEXT NOT NULL,
    operator VARCHAR(255) NOT NULL,
    valid_from TIMESTAMP NOT NULL,
    valid_until TIMESTAMP,
    service_area_geometries TEXT[]
);

CREATE INDEX service_area_municipality
    ON service_area (municipality, operator);

CREATE INDEX  service_area_municipality_active 
    ON service_area (valid_until) WHERE (valid_until IS NULL);

CREATE EXTENSION pgcrypto; 

CREATE TABLE service_area_geometry (
    geom_hash VARCHAR PRIMARY KEY,
    geom GEOMETRY,
    municipalities TEXT[]
);

-- CREATE INDEX service_area_geometry_index
--   ON service_area_geometry
--   USING GIST (geom);