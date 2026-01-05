package analyze

import (
	"database/sql"
	"log"
	"time"
)

// This could be improved by also checking at what times stops were opened / not opened.
func CountWronglyParkedVehicles(db *sql.DB) {
	log.Print("Counting wrongly parked vehicles...")
	stmt := `
		CREATE OR REPLACE TABLE wrongly_parked_vehicles_output AS
		WITH wrongly_parked_vehicles as (SELECT DISTINCT(park_event_id)
		FROM park_events_in_zone 
		JOIN zones ON park_events_in_zone.stat_ref = zones.stat_ref
		WHERE zones.geography_type = 'no_parking'
		AND zones.effective_date <= park_events_in_zone.start_time and (zones.retire_date IS NULL OR zones.retire_date > park_events_in_zone.start_time)
		AND split_part(vehicle_type, ':', 1) IN affected_modalities
		EXCEPT
		SELECT park_event_id
		FROM park_events_in_zone
		JOIN zones ON park_events_in_zone.stat_ref = zones.stat_ref
		WHERE zones.geography_type = 'stop' 
		AND zones.effective_date <= park_events_in_zone.start_time and (zones.retire_date IS NULL OR zones.retire_date > park_events_in_zone.start_time)
		AND split_part(vehicle_type, ':', 1) IN affected_modalities
		)
		SELECT st_y(location), st_x(location), system_id, vehicle_type, start_time, end_time, municipality
		FROM park_events_in_zone
		WHERE park_event_id IN (SELECT park_event_id FROM wrongly_parked_vehicles);
	`

	_, err := db.Query(stmt)
	if err != nil {
		log.Fatal(err)
	}

}

func AggregateWronglyParkedVehiclesPerDay(db *sql.DB, startDate time.Time, endDate time.Time) {
	log.Print("Aggregating wrongly parked vehicles per day...")
	_, err := db.Exec(`
		INSERT INTO day_statistics
		SELECT
			d.day::DATE AS date,
			6 as indicator,
			w.municipality AS geometry_ref,
			w.system_id as system_id,
			w.vehicle_type as vehicle_type,
			COUNT(*) AS value
		FROM wrongly_parked_vehicles_output w
		CROSS JOIN generate_series(
			date_trunc('day', w.start_time),
			date_trunc('day', w.end_time),
			INTERVAL 1 DAY
		) AS d(day)
		WHERE d.day >= $1 AND d.day <= $2
		GROUP BY
			w.municipality,
			w.system_id,
			w.vehicle_type,
			d.day;
	`, startDate, endDate)
	if err != nil {
		log.Fatal(err)
	}
}
