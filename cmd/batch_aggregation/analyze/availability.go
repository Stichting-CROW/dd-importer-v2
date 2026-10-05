package analyze

import (
	"database/sql"
	"deelfietsdashboard-importer/cmd/batch_aggregation/indicators"
	"deelfietsdashboard-importer/cmd/batch_aggregation/util"
	"log"
	"time"
)

func CountAvailableVehiclesInPublicSpace(db *sql.DB, date time.Time, selected []indicators.Indicator) {
	if !indicators.IsSelectedOnDate(selected, "available_vehicles_in_public_space", date) {
		return
	}

	indicatorID, err := indicators.GetNumericIndicatorID("available_vehicles_in_public_space")
	if err != nil {
		log.Fatal(err)
	}

	measurementMoments := util.GetDefaultMeasrurementMoments(date)
	for momentIndex, moment := range measurementMoments {
		log.Print(date.Format("2006-01-02") + ": Counting available vehicles in public space at " + moment.Format("2006-01-02 15:04:05") + "...")
		countAvailableVehiclesAtMoment(db, date, moment, momentIndex, indicatorID)
	}
}

func countAvailableVehiclesAtMoment(db *sql.DB, date time.Time, moment time.Time, measurementMomentIndex int, indicatorID int) {
	stmt := `
		INSERT INTO moment_statistics
			(date, measurement_moment, indicator, geometry_ref, system_id, vehicle_type, trip_source, value)
		SELECT
			$1::DATE AS date,
			$2 AS measurement_moment,
			$3 AS indicator,
			geometry_ref,
			system_id,
			vehicle_type,
			trip_source,
			COUNT(*) AS value
		FROM (
			SELECT
				pez.stat_ref AS geometry_ref,
				pez.system_id,
				pez.vehicle_type,
				s.trip_source
			FROM park_events_in_zone pez
			JOIN (
				SELECT DISTINCT system_id, trip_source
				FROM trips_in_zone
				WHERE trip_source IS NOT NULL
			) s ON s.system_id = pez.system_id
			WHERE pez.start_time <= $4
				AND (pez.end_time >= $4 OR pez.end_time IS NULL)
				AND pez.zone_type = 'municipality'
				AND NOT EXISTS (
					SELECT 1
					FROM non_operational_events noe
					WHERE noe.park_event_id = pez.park_event_id
						AND noe.start_time <= $4
						AND (noe.end_time >= $4 OR noe.end_time IS NULL)
				)

			UNION ALL

			SELECT
				stat_ref AS geometry_ref,
				system_id,
				vehicle_type,
				trip_source
			FROM trips_in_zone
			WHERE start_time <= $4
				AND end_time > $4
				AND end_time < $1::DATE + INTERVAL '1 day'
		) q
		GROUP BY geometry_ref, system_id, vehicle_type, trip_source;
	`

	_, err := db.Exec(stmt, date.Format("2006-01-02"), measurementMomentIndex, indicatorID, moment)
	if err != nil {
		log.Fatal(err)
	}
}

func AggregateAvailableVehiclesPerDay(db *sql.DB, selected []indicators.Indicator) {
	if !indicators.HasIndicator(selected, "available_vehicles_in_public_space") {
		return
	}

	indicatorID, err := indicators.GetNumericIndicatorID("available_vehicles_in_public_space")
	if err != nil {
		log.Fatal(err)
	}

	stmt := `
		INSERT INTO day_statistics
			(date, indicator, geometry_ref, system_id, vehicle_type, trip_source, value)
		SELECT
			date,
			indicator,
			geometry_ref,
			system_id,
			vehicle_type,
			trip_source,
			MAX(value) AS value
		FROM moment_statistics
		WHERE indicator = $1
		GROUP BY date, indicator, geometry_ref, system_id, vehicle_type, trip_source;
	`

	_, err = db.Exec(stmt, indicatorID)
	if err != nil {
		log.Fatal(err)
	}
}
