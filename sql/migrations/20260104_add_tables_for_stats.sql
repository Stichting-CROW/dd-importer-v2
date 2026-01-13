CREATE TABLE IF NOT EXISTS moment_statistics (
    date 	           DATE NOT NULL,
    measurement_moment SMALLINT NOT NULL,
    indicator 	       SMALLINT NOT NULL,
    geometry_ref       VARCHAR NOT NULL,
    system_id          VARCHAR NOT NULL,
    vehicle_type       VARCHAR NOT NULL,
    value              NUMERIC NOT NULL,
    PRIMARY KEY (date, measurement_moment, indicator, geometry_ref, system_id, vehicle_type)
);

CREATE INDEX idx_moment_statistics_covering
ON moment_statistics (geometry_ref, indicator, system_id, vehicle_type, measurement_moment, date)
INCLUDE (value);


CREATE TABLE IF NOT EXISTS day_statistics (
    date            DATE NOT NULL,
    indicator       SMALLINT NOT NULL,
    geometry_ref    VARCHAR NOT NULL,
    system_id       VARCHAR NOT NULL,
    vehicle_type    VARCHAR NOT NULL,
    value           NUMERIC NOT NULL,
    PRIMARY KEY (date, indicator, geometry_ref, system_id, vehicle_type)
);

CREATE INDEX idx_day_statistics_covering
ON day_statistics (geometry_ref, indicator, system_id, vehicle_type, date)
INCLUDE (value);