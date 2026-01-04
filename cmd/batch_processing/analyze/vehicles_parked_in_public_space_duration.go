package analyze

import (
	"database/sql"
	"deelfietsdashboard-importer/cmd/batch_processing/util"
	"log"
	"strconv"
	"time"
)

func CountVehiclesInPublicSpaceForLongerThenXDays(db *sql.DB, measurementMoment time.Time, durationDays int) {
	stmt := `
	INSERT INTO moment_statistics
	SELECT
	$1::DATE AS date,
    0 AS measurement_moment,
	$3 AS indicator,
    stat_ref AS geometry_ref,
    system_id,
    vehicle_type,
    COUNT(*) AS value
	FROM park_events_in_zone
	WHERE start_time <= $1
	AND (end_time >= $1 OR end_time IS NULL)
	AND start_time <= $1 - ($2 * INTERVAL '1 day')
	GROUP BY stat_ref, system_id, vehicle_type;
	`
	_, err := db.Exec(stmt, measurementMoment, durationDays, util.GetNumericIndicatorID("count_vehicles_in_public_space_longer_then_"+strconv.Itoa(durationDays)+"_days"))
	if err != nil {
		log.Fatal(err)
	}

}
