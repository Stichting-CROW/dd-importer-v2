package main

import (
	"database/sql"
	"fmt"
	"log"
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
	_, err = db.Exec("PRAGMA enable_profiling = 'query_tree';")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("ATTACH 'dbname=dashboarddeelmobiliteit user=postgres host=127.0.0.1 password=mysecretpassword' AS postgres_db (TYPE postgres);")
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
		SELECT name, stat_ref, geography_type, ST_GeomFromWKB(buffered_area) AS buffered_area
		FROM postgres_query('postgres_db', 
			'SELECT z.name,
  			COALESCE(g.geography_id::text, z.stats_ref) AS stat_ref,
  			g.geography_type,
  			CASE
    			WHEN g.geography_type = ''stop'' THEN ST_asBinary(ST_Buffer(z.area::geography, 30)::geometry)
    			WHEN g.geography_type = ''no_parking'' THEN ST_asBinary(ST_Buffer(z.area::geography, -30)::geometry)
    			ELSE ST_asBinary(z.area)	
  			END AS buffered_area
		FROM zones z
		LEFT JOIN geographies g USING(zone_id)
		WHERE (retire_date IS null or retire_date > NOW())
		AND zone_type IN (''custom'', ''residential_area'', ''municipality'');
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

func findAllIntersections(db *sql.DB) {
	row := db.QueryRow(`SELECT count(*) as count
		FROM park_events
		JOIN zones
		ON ST_Dwithin(location, zones.buffered_area, 0.0);`)
	var count int
	err := row.Scan(&count)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Found intersections:", count)

	rows, err := db.Query(`
		SELECT * FROM(
		SELECT zones.stat_ref, count(*) as count
		FROM park_events
		JOIN zones
		ON ST_Dwithin(location, zones.buffered_area, 0.0)
		GROUP BY zones.stat_ref)
		ORDER BY count DESC
		LIMIT 10;`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var statRef string
		var count int
		if err := rows.Scan(&statRef, &count); err != nil {
			log.Fatal(err)
		}
		log.Println("Found intersections for", statRef, ":", count)
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
	// rows.Next()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer rows.Close()
	// for rows.Next() {
	// 	var zoneID string
	// 	var area []byte
	// 	if err := rows.Scan(&zoneID, &area); err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	log.Println(zoneID, len(area))
	// }
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

func loadParkEventOnDate(db *sql.DB, date time.Time) {
	stmt := `
	CREATE TABLE IF NOT EXISTS park_events AS
	SELECT park_event_id, ST_GeomFromWKB(location) AS location
	FROM postgres_query('postgres_db', 
		'SELECT park_event_id, ST_AsBinary(location) AS location, start_time, end_time, 
		CONCAT(form_factor, '':'', propulsion_type) AS vehicle_type
		FROM park_events
		JOIN vehicle_type
		USING(vehicle_type_id)
		WHERE start_time >= ''%s''::date AND start_time < ''%s''::date + INTERVAL ''1 day'';'
	);
	`
	stmt = fmt.Sprintf(stmt, date.Format("2006-01-02"), date.Format("2006-01-02"))

	_, err := db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}
}
