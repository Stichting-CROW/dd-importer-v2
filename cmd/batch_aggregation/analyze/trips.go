package analyze

import (
	"database/sql"
	"deelfietsdashboard-importer/cmd/batch_aggregation/indicators"
	"log"
	"time"
)

func CountTripsPerDay(db *sql.DB, date time.Time, selected []indicators.Indicator) {
	if !indicators.IsSelectedOnDate(selected, "count_trips_per_day", date) {
		return
	}

	indicatorID, err := indicators.GetNumericIndicatorID("count_trips_per_day")
	if err != nil {
		log.Fatal(err)
	}

	stmt := `
		INSERT INTO day_statistics
		SELECT
			$1::DATE AS date,
			$2 AS indicator,
			stat_ref AS geometry_ref,
			system_id,
			vehicle_type,
			COUNT(*) AS value
		FROM trips_in_zone
		WHERE end_time >= $1
			AND end_time < $1 + INTERVAL '1 day'
		GROUP BY stat_ref, system_id, vehicle_type;
	`

	_, err = db.Exec(stmt, date, indicatorID)
	if err != nil {
		log.Fatal(err)
	}
}

func ComputeTripsPerVehiclePerDay(db *sql.DB, selected []indicators.Indicator) {
	if !indicators.HasIndicator(selected, "trips_per_vehicle_per_day") {
		return
	}

	if !indicators.HasIndicator(selected, "count_trips_per_day") || !indicators.HasIndicator(selected, "count_vehicles_in_public_space") {
		log.Print("trips_per_vehicle_per_day requires count_trips_per_day and count_vehicles_in_public_space to be selected")
		return
	}

	tripsIndicatorID, err := indicators.GetNumericIndicatorID("count_trips_per_day")
	if err != nil {
		log.Fatal(err)
	}

	vehiclesIndicatorID, err := indicators.GetNumericIndicatorID("count_vehicles_in_public_space")
	if err != nil {
		log.Fatal(err)
	}

	resultIndicatorID, err := indicators.GetNumericIndicatorID("trips_per_vehicle_per_day")
	if err != nil {
		log.Fatal(err)
	}

	stmt := `
		INSERT INTO day_statistics
		SELECT
			t.date,
			$1 AS indicator,
			t.geometry_ref,
			t.system_id,
			t.vehicle_type,
			t.value / NULLIF(v.value, 0) AS value
		FROM day_statistics t
		JOIN day_statistics v
			USING (date, geometry_ref, system_id, vehicle_type)
		WHERE t.indicator = $2
			AND v.indicator = $3;
	`

	_, err = db.Exec(stmt, resultIndicatorID, tripsIndicatorID, vehiclesIndicatorID)
	if err != nil {
		log.Fatal(err)
	}
}
