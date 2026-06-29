package analyze

import (
	"database/sql"
	"deelfietsdashboard-importer/cmd/batch_aggregation/indicators"
	"fmt"
	"log"
	"time"
)

func CountNonOperationalVehiclesLongerThen24Hours(db *sql.DB, measurementMoment time.Time, selected []indicators.Indicator) {
	countNonOperationalVehiclesLongerThan(db, measurementMoment, 24, "hour", "count_vehicles_non_operational_longer_then_24_hours", selected)
}

func CountNonOperationalVehiclesLongerThen7Days(db *sql.DB, measurementMoment time.Time, selected []indicators.Indicator) {
	countNonOperationalVehiclesLongerThan(db, measurementMoment, 7, "day", "count_vehicles_non_operational_longer_then_7_days", selected)
}

func countNonOperationalVehiclesLongerThan(db *sql.DB, measurementMoment time.Time, duration int, unit string, textID string, selected []indicators.Indicator) {
	if !indicators.IsSelectedOnDate(selected, textID, measurementMoment) {
		return
	}

	indicatorID, err := indicators.GetNumericIndicatorID(textID)
	if err != nil {
		log.Fatal(err)
	}

	stmt := fmt.Sprintf(`
		INSERT INTO moment_statistics
		SELECT
			$1::DATE AS date,
			0 AS measurement_moment,
			$2 AS indicator,
			stat_ref AS geometry_ref,
			system_id,
			vehicle_type,
			COUNT(*) AS value
		FROM park_events_in_zone pez
		JOIN non_operational_events noe
			ON noe.park_event_id = pez.park_event_id
		WHERE noe.start_time <= $3
			AND noe.start_time <= $3 - ($4 * INTERVAL '1 %s')
			AND (noe.end_time >= $3 OR noe.end_time IS NULL)
			AND zone_type = 'municipality'
		GROUP BY stat_ref, system_id, vehicle_type;
	`, unit)

	_, err = db.Exec(stmt, measurementMoment, indicatorID, measurementMoment, duration)
	if err != nil {
		log.Fatal(err)
	}
}
