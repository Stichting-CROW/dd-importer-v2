package analyze

import (
	"database/sql"
	"log"
)

func FindIntersectionsWithZones(db *sql.DB) {
	stmt := `
		DROP TABLE IF EXISTS park_events_in_zone;
	`
	_, err := db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}

	stmt2 := `
		CREATE TABLE IF NOT EXISTS park_events_in_zone AS (
        SELECT park_event_id, location, start_time, end_time, system_id, vehicle_type, 
		zones.stat_ref, zones.zone_type, zones.geography_type, 
		zm.stat_ref AS municipality
		FROM park_events
		JOIN zones
		ON ST_Dwithin(location, zones.buffered_area, 0.0)
		LEFT JOIN zones as zm
           ON ST_DWithin(park_events.location, zm.buffered_area, 0.0)
           AND zm.zone_type = 'municipality'
		);
	`
	_, err = db.Exec(stmt2)
	if err != nil {
		log.Fatal(err)
	}
}
