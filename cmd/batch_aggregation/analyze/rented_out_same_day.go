package analyze

import (
	"database/sql"
	"deelfietsdashboard-importer/cmd/batch_aggregation/indicators"
	"deelfietsdashboard-importer/cmd/batch_aggregation/util"
	"log"
	"time"
)

func CountRentedOutVehiclesAvailableSameDay(db *sql.DB, date time.Time, selected []indicators.Indicator) {
	if !indicators.IsSelectedOnDate(selected, "rented_out_vehicles_available_same_day", date) {
		return
	}

	indicatorID, err := indicators.GetNumericIndicatorID("rented_out_vehicles_available_same_day")
	if err != nil {
		log.Fatal(err)
	}

	measurementMoments := util.GetDefaultMeasrurementMoments(date)
	for momentIndex, moment := range measurementMoments {
		log.Print(date.Format("2006-01-02") + ": Counting rented out vehicles that become available the same day at " + moment.Format("2006-01-02 15:04:05") + "...")
		countRentedOutVehiclesAvailableSameDayAtMoment(db, date, moment, momentIndex, indicatorID)
	}
}

func countRentedOutVehiclesAvailableSameDayAtMoment(db *sql.DB, date time.Time, moment time.Time, measurementMomentIndex int, indicatorID int) {
	stmt := `
		INSERT INTO moment_statistics
			(date, measurement_moment, indicator, geometry_ref, system_id, vehicle_type, trip_source, value)
		SELECT
			$1::DATE AS date,
			$2 AS measurement_moment,
			$3 AS indicator,
			stat_ref AS geometry_ref,
			system_id,
			vehicle_type,
			'vehicles' AS trip_source,
			COUNT(*) AS value
		FROM trips_in_zone
		WHERE trip_source = 'vehicles'
			AND start_time <= $4
			AND end_time > $4
			AND end_time < $1::DATE + INTERVAL '1 day'
		GROUP BY stat_ref, system_id, vehicle_type;
	`

	_, err := db.Exec(stmt, date.Format("2006-01-02"), measurementMomentIndex, indicatorID, moment)
	if err != nil {
		log.Fatal(err)
	}
}
