package analyze

import (
	"database/sql"
	"deelfietsdashboard-importer/cmd/batch_processing/util"
	"log"
	"time"
)

func CountVehiclesInPublicSpaceOnDate(db *sql.DB, date time.Time) {
	measurementMoments := util.GetDefaultMeasrurementMoments(date)
	for moment_index, moment := range measurementMoments {
		countVehiclesInPublicSpace(db, moment, moment_index)
	}
}

func countVehiclesInPublicSpace(db *sql.DB, timestamp time.Time, measurementMomentIndex int) {
	stmt := `
		INSERT INTO moment_statistics
		SELECT $1::DATE AS date,
			$2 AS measurement_moment,
			$3 AS indicator,
			stat_ref,
			system_id,
			vehicle_type,
			COUNT(*) AS value
		FROM park_events_in_zone
		WHERE start_time <= $1 AND (end_time >= $1 OR end_time IS NULL)
		GROUP BY stat_ref, system_id, vehicle_type;
	`
	_, err := db.Exec(stmt, timestamp, measurementMomentIndex, util.GetNumericIndicatorID("count_vehicles_in_public_space"))
	if err != nil {
		log.Fatal(err)
	}
}

func AggregateVehiclesInPublicSpacePerDay(db *sql.DB) {
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
	_, err := db.Exec(stmt, util.GetNumericIndicatorID("count_vehicles_in_public_space"))
	if err != nil {
		log.Fatal(err)
	}
}
