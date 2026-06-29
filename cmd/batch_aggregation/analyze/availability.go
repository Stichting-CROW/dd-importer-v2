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
		countAvailableVehiclesAtMoment(db, date, moment, momentIndex, indicatorID)
	}
}

func countAvailableVehiclesAtMoment(db *sql.DB, date time.Time, moment time.Time, measurementMomentIndex int, indicatorID int) {
	stmt := `
		INSERT INTO moment_statistics
		SELECT
			$1::DATE AS date,
			$2 AS measurement_moment,
			$3 AS indicator,
			geometry_ref,
			system_id,
			vehicle_type,
			SUM(value) AS value
		FROM (
			SELECT
				stat_ref AS geometry_ref,
				system_id,
				vehicle_type,
				COUNT(*) AS value
			FROM park_events_in_zone pez
			WHERE pez.start_time <= $4
				AND (pez.end_time >= $4 OR pez.end_time IS NULL)
				AND zone_type = 'municipality'
				AND NOT EXISTS (
					SELECT 1
					FROM non_operational_events noe
					WHERE noe.park_event_id = pez.park_event_id
						AND noe.start_time <= $4
						AND (noe.end_time >= $4 OR noe.end_time IS NULL)
				)
			GROUP BY stat_ref, system_id, vehicle_type

			UNION ALL

			SELECT
				stat_ref AS geometry_ref,
				system_id,
				vehicle_type,
				COUNT(*) AS value
			FROM trips_in_zone
			WHERE start_time <= $4
				AND end_time > $4
				AND end_time < $1::DATE + INTERVAL '1 day'
			GROUP BY stat_ref, system_id, vehicle_type
		) q
		GROUP BY geometry_ref, system_id, vehicle_type;
	`

	_, err := db.Exec(stmt, date, measurementMomentIndex, indicatorID, moment)
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
		SELECT
			date,
			indicator,
			geometry_ref,
			system_id,
			vehicle_type,
			MAX(value) AS value
		FROM moment_statistics
		WHERE indicator = $1
		GROUP BY date, indicator, geometry_ref, system_id, vehicle_type;
	`

	_, err = db.Exec(stmt, indicatorID)
	if err != nil {
		log.Fatal(err)
	}
}
