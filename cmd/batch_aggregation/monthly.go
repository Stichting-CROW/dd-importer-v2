package main

import (
	"database/sql"
	"deelfietsdashboard-importer/cmd/batch_aggregation/indicators"
	"log"
	"time"
)

func computeMonthlyStatisticsForMonth(db *sql.DB, month time.Time, selected []indicators.Indicator) {
	if indicators.HasIndicator(selected, "count_ended_rentals_by_distance_and_time_period") {
		indicatorID, err := indicators.GetNumericIndicatorID("count_ended_rentals_by_distance_and_time_period")
		if err != nil {
			log.Fatal(err)
		}
		insertMonthlyDistanceStatistics(db, month, indicatorID)
	}

	if indicators.HasIndicator(selected, "count_ended_rentals_by_duration_and_time_period") {
		indicatorID, err := indicators.GetNumericIndicatorID("count_ended_rentals_by_duration_and_time_period")
		if err != nil {
			log.Fatal(err)
		}
		insertMonthlyDurationStatistics(db, month, indicatorID)
	}

	if indicators.HasIndicator(selected, "count_ended_rentals_by_real_distance_and_time_period") {
		indicatorID, err := indicators.GetNumericIndicatorID("count_ended_rentals_by_real_distance_and_time_period")
		if err != nil {
			log.Fatal(err)
		}
		insertMonthlyRealDistanceStatistics(db, month, indicatorID)
	}
}

func insertMonthlyDistanceStatistics(db *sql.DB, month time.Time, indicatorID int) {
	stmt := `
		INSERT INTO month_statistics
			(month, indicator, geometry_ref, system_id, vehicle_type, trip_source, category, time_period, value)
		SELECT
			month,
			indicator,
			geometry_ref,
			system_id,
			vehicle_type,
			trip_source,
			category,
			time_period,
			COUNT(*) AS value
		FROM (
			SELECT
				$1::DATE AS month,
				$2 AS indicator,
				d.geometry_ref,
				d.system_id,
				d.vehicle_type,
				'vehicles' AS trip_source,
				CASE
					WHEN d.distance_km < 0.1 THEN 0
					WHEN d.distance_km < 1 THEN 1
					WHEN d.distance_km < 3.7 THEN 2
					WHEN d.distance_km < 7.5 THEN 3
					WHEN d.distance_km < 15 THEN 4
					WHEN d.distance_km < 30 THEN 5
					WHEN d.distance_km < 50 THEN 6
					WHEN d.distance_km < 75 THEN 7
					WHEN d.distance_km < 500 THEN 8
					ELSE 9
				END AS category,
				CASE
					WHEN d.end_hour < 7 THEN 0
					WHEN d.end_hour < 9 THEN 1
					WHEN d.end_hour < 12 THEN 2
					WHEN d.end_hour < 16 THEN 3
					WHEN d.end_hour < 18 THEN 4
					ELSE 5
				END AS time_period
			FROM (
				SELECT
					tz.stat_ref AS geometry_ref,
					tz.system_id,
					tz.vehicle_type,
					ST_Distance_Sphere(t.start_location, t.end_location) / 1000.0 AS distance_km,
					EXTRACT(HOUR FROM timezone('Europe/Amsterdam', tz.end_time)) AS end_hour
				FROM trips_in_zone tz
				JOIN trips t ON t.trip_id = tz.trip_id
				WHERE tz.trip_source = 'vehicles'
					AND t.start_location IS NOT NULL
					AND t.end_location IS NOT NULL
			) d
			WHERE d.distance_km IS NOT NULL
		) q
		GROUP BY month, indicator, geometry_ref, system_id, vehicle_type, trip_source, category, time_period;
	`

	_, err := db.Exec(stmt, month.Format("2006-01-02"), indicatorID)
	if err != nil {
		log.Fatal(err)
	}
}

func insertMonthlyDurationStatistics(db *sql.DB, month time.Time, indicatorID int) {
	stmt := `
		INSERT INTO month_statistics
			(month, indicator, geometry_ref, system_id, vehicle_type, trip_source, category, time_period, value)
		SELECT
			month,
			indicator,
			geometry_ref,
			system_id,
			vehicle_type,
			trip_source,
			category,
			time_period,
			COUNT(*) AS value
		FROM (
			SELECT
				$1::DATE AS month,
				$2 AS indicator,
				tz.stat_ref AS geometry_ref,
				tz.system_id,
				tz.vehicle_type,
				'vehicles' AS trip_source,
				CASE
					WHEN date_diff('second', tz.start_time, tz.end_time) < 60 THEN 0
					WHEN date_diff('second', tz.start_time, tz.end_time) < 300 THEN 1
					WHEN date_diff('second', tz.start_time, tz.end_time) < 600 THEN 2
					WHEN date_diff('second', tz.start_time, tz.end_time) < 900 THEN 3
					WHEN date_diff('second', tz.start_time, tz.end_time) < 1200 THEN 4
					WHEN date_diff('second', tz.start_time, tz.end_time) < 1800 THEN 5
					WHEN date_diff('second', tz.start_time, tz.end_time) < 3600 THEN 6
					WHEN date_diff('second', tz.start_time, tz.end_time) < 7200 THEN 7
					WHEN date_diff('second', tz.start_time, tz.end_time) < 86400 THEN 8
					WHEN date_diff('second', tz.start_time, tz.end_time) < 604800 THEN 9
					WHEN date_diff('second', tz.start_time, tz.end_time) < 2419200 THEN 10
					ELSE 11
				END AS category,
				CASE
					WHEN EXTRACT(HOUR FROM timezone('Europe/Amsterdam', tz.end_time)) < 7 THEN 0
					WHEN EXTRACT(HOUR FROM timezone('Europe/Amsterdam', tz.end_time)) < 9 THEN 1
					WHEN EXTRACT(HOUR FROM timezone('Europe/Amsterdam', tz.end_time)) < 12 THEN 2
					WHEN EXTRACT(HOUR FROM timezone('Europe/Amsterdam', tz.end_time)) < 16 THEN 3
					WHEN EXTRACT(HOUR FROM timezone('Europe/Amsterdam', tz.end_time)) < 18 THEN 4
					ELSE 5
				END AS time_period
			FROM trips_in_zone tz
			WHERE tz.trip_source = 'vehicles'
				AND tz.end_time >= tz.start_time
		) q
		GROUP BY month, indicator, geometry_ref, system_id, vehicle_type, trip_source, category, time_period;
	`

	_, err := db.Exec(stmt, month.Format("2006-01-02"), indicatorID)
	if err != nil {
		log.Fatal(err)
	}
}

func insertMonthlyRealDistanceStatistics(db *sql.DB, month time.Time, indicatorID int) {
	stmt := `
		INSERT INTO month_statistics
			(month, indicator, geometry_ref, system_id, vehicle_type, trip_source, category, time_period, value)
		SELECT
			month,
			indicator,
			geometry_ref,
			system_id,
			vehicle_type,
			trip_source,
			category,
			time_period,
			COUNT(*) AS value
		FROM (
			SELECT
				$1::DATE AS month,
				$2 AS indicator,
				tz.stat_ref AS geometry_ref,
				tz.system_id,
				tz.vehicle_type,
				'trips' AS trip_source,
				CASE
					WHEN t.distance_over_road / 1000.0 < 0.1 THEN 0
					WHEN t.distance_over_road / 1000.0 < 1 THEN 1
					WHEN t.distance_over_road / 1000.0 < 3.7 THEN 2
					WHEN t.distance_over_road / 1000.0 < 7.5 THEN 3
					WHEN t.distance_over_road / 1000.0 < 15 THEN 4
					WHEN t.distance_over_road / 1000.0 < 30 THEN 5
					WHEN t.distance_over_road / 1000.0 < 50 THEN 6
					WHEN t.distance_over_road / 1000.0 < 75 THEN 7
					WHEN t.distance_over_road / 1000.0 < 500 THEN 8
					ELSE 9
				END AS category,
				CASE
					WHEN EXTRACT(HOUR FROM timezone('Europe/Amsterdam', tz.end_time)) < 7 THEN 0
					WHEN EXTRACT(HOUR FROM timezone('Europe/Amsterdam', tz.end_time)) < 9 THEN 1
					WHEN EXTRACT(HOUR FROM timezone('Europe/Amsterdam', tz.end_time)) < 12 THEN 2
					WHEN EXTRACT(HOUR FROM timezone('Europe/Amsterdam', tz.end_time)) < 16 THEN 3
					WHEN EXTRACT(HOUR FROM timezone('Europe/Amsterdam', tz.end_time)) < 18 THEN 4
					ELSE 5
				END AS time_period
			FROM trips_in_zone tz
			JOIN trips t ON t.trip_id = tz.trip_id
			WHERE tz.trip_source = 'trips'
				AND t.distance_over_road IS NOT NULL
				AND t.distance_over_road >= 0
		) q
		GROUP BY month, indicator, geometry_ref, system_id, vehicle_type, trip_source, category, time_period;
	`

	_, err := db.Exec(stmt, month.Format("2006-01-02"), indicatorID)
	if err != nil {
		log.Fatal(err)
	}
}
