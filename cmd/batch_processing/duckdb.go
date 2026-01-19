package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/marcboeker/go-duckdb/v2"
)

func initDuckDB() *sql.DB {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec("INSTALL spatial;")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("LOAD SPATIAL;")
	if err != nil {
		log.Fatal(err)
	}

	// _, err = db.Exec("PRAGMA enable_profiling = 'query_tree';")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	log.Print("Connect postgresql database")
	stmt := fmt.Sprintf(
		"ATTACH '%s' AS postgres_db (TYPE postgres);", os.Getenv("PGURL"))
	_, err = db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}

	log.Print("Creating moment_statistics table...")
	stmt = `
	CREATE TABLE IF NOT EXISTS moment_statistics (
	    date 	           DATE NOT NULL,
		measurement_moment TINYINT NOT NULL,
		indicator 	       TINYINT NOT NULL,
		geometry_ref       VARCHAR NOT NULL,
		system_id          VARCHAR NOT NULL,
		vehicle_type       VARCHAR NOT NULL,
		value              NUMERIC NOT NULL,
		PRIMARY KEY (date, measurement_moment, indicator, geometry_ref, system_id, vehicle_type)
	);
   `
	_, err = db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}

	log.Print("Creating day_statistics table...")
	stmt2 := `
	CREATE TABLE IF NOT EXISTS day_statistics (
		date DATE NOT NULL,
		indicator TINYINT NOT NULL,
		geometry_ref VARCHAR NOT NULL,
		system_id VARCHAR NOT NULL,
		vehicle_type VARCHAR NOT NULL,
		value NUMERIC NOT NULL,
		PRIMARY KEY (date, indicator, geometry_ref, system_id, vehicle_type)
	);
	`

	_, err = db.Exec(stmt2)
	if err != nil {
		log.Fatal(err)
	}

	return db
}

func loadZones(db *sql.DB) {
	stmt := `
		DROP TABLE IF EXISTS buffered_zones;
	`
	_, err := db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}

	stmt = `
		CREATE TABLE IF NOT EXISTS zones AS
		SELECT name, stat_ref, zone_type, geography_type, effective_date, retire_date,
		ST_GeomFromWKB(buffered_area) AS buffered_area, affected_modalities
		FROM postgres_query('postgres_db', 
			'SELECT z.name,
  			COALESCE(g.geography_id::text, z.stats_ref) AS stat_ref,
  			z.zone_type,
  			g.geography_type,
			g.affected_modalities,
			g.effective_date,
			g.retire_date,
  			CASE
    			WHEN g.geography_type = ''stop'' THEN ST_asBinary(ST_Buffer(z.area::geography, 30)::geometry)
    			WHEN g.geography_type = ''no_parking'' THEN ST_asBinary(ST_Buffer(z.area::geography, -30)::geometry)
    			ELSE ST_asBinary(z.area)	
  			END AS buffered_area
		FROM zones z
		LEFT JOIN geographies g USING(zone_id)
		WHERE zone_type IN (''custom'', ''residential_area'', ''municipality'');
		');
	`

	_, err = db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}
}

func loadZonesBasedOnID(db *sql.DB, zonesPath string, idField string) {
	stmt := `
	CREATE TABLE IF NOT EXISTS zones AS SELECT zone_id, ST_GeomFromWKB(area) as area FROM postgres_scan('host=127.0.0.1 port=5432 dbname=dashboarddeelmobiliteit user=postgres password=3324a7ee8bba383effacd57ec5c680ef',
	'public',
	'zones'
	) WHERE zone_id = $1;`
	db.Exec(stmt, idField)
}

func loadAllParkEventData(db *sql.DB) {
	stmt := `
	CREATE TABLE IF NOT EXISTS park_events AS
	SELECT park_event_id, ST_GeomFromWKB(location) AS location
	FROM postgres_query('postgres_db', 
		'SELECT park_event_id, ST_AsBinary(location) AS location
		FROM park_events;'
	);
	`
	_, err := db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}
}

func loadParkEventInBetween(db *sql.DB, startDate time.Time, endDate time.Time) {
	stmt := `
	DROP TABLE IF EXISTS park_events;
	`
	_, err := db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}

	stmt = `
	CREATE TABLE IF NOT EXISTS park_events AS
	SELECT park_event_id, ST_GeomFromWKB(location) AS location, start_time, end_time, 
	system_id, vehicle_type
	FROM postgres_query('postgres_db', 
		'SELECT park_event_id, ST_AsBinary(location) AS location, start_time, end_time, 
		park_events.system_id, CONCAT(form_factor, '':'', propulsion_type) AS vehicle_type
		FROM park_events
		JOIN vehicle_type
		USING(vehicle_type_id)
		WHERE (end_time >= ''%s''::date or end_time IS NULL) AND start_time < ''%s''::date + INTERVAL ''1 day'';'
	);
	`
	stmt = fmt.Sprintf(stmt, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	_, err = db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}
}
