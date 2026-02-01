package main

import (
	"context"
	"database/sql"
	"deelfietsdashboard-importer/cmd/batch_aggregation/analyze"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	log.Print("Starting batch processing...")

	pgConn := initPostgresDB()
	dConn := initDuckDB()
	startTime := time.Now()

	loadZones(dConn)

	newestDate := getNewestDateInMomentStatistics(pgConn).Local()
	fmt.Printf("Newest date in moment_statistics: %s\n", newestDate.Format("2006-01-02"))
	aggregateData(dConn, newestDate.AddDate(0, 0, 1))

	analyze.AggregateVehiclesInPublicSpacePerDay(dConn)
	writeToPostgres(dConn)

	log.Printf("Done analyzing data, took %s", time.Since(startTime))
}

func aggregateData(dConn *sql.DB, startDate time.Time) {
	const chunkSize = 30
	yesterday := time.Now().Local().AddDate(0, 0, -1)

	for start := startDate; start.Before(yesterday); {
		end := start.AddDate(0, 0, chunkSize)

		if end.After(yesterday) {
			end = yesterday
		}

		fmt.Printf("Chunk: %s -> %s\n",
			start.Format("2006-01-02"),
			end.Format("2006-01-02"),
		)

		analyzeChunk(dConn, start, end)
		start = end.AddDate(0, 0, 1)
	}
}

func analyzeChunk(dConn *sql.DB, startDate time.Time, endDate time.Time) {
	loadParkEventInBetween(dConn, startDate, endDate)
	//loadParkEventOnDate(dConn, date)
	analyze.FindIntersectionsWithZones(dConn)

	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		log.Printf("Analyzing date %s", d.Format("2006-01-02"))
		analyzeDay(dConn, d)
	}
	analyze.CountWronglyParkedVehicles(dConn)
	analyze.AggregateWronglyParkedVehiclesPerDay(dConn, startDate, endDate)
}

func analyzeDay(dConn *sql.DB, date time.Time) {
	// Set measurement moment to 03:30 on the given date
	measurementMoment := time.Date(
		date.Year(), date.Month(), date.Day(),
		03, 30, 0, 0,
		time.Local,
	)
	analyze.CountVehiclesInPublicSpaceForLongerThenXDays(dConn, measurementMoment, 1)
	analyze.CountVehiclesInPublicSpaceForLongerThenXDays(dConn, measurementMoment, 3)
	analyze.CountVehiclesInPublicSpaceForLongerThenXDays(dConn, measurementMoment, 7)
	analyze.CountVehiclesInPublicSpaceForLongerThenXDays(dConn, measurementMoment, 14)
	analyze.CountVehiclesInPublicSpaceOnDate(dConn, measurementMoment)
}

func writeToPostgres(db *sql.DB) {
	log.Print("Writing results to Postgres...")
	stmt := `
	INSERT INTO postgres_db.moment_statistics
	SELECT
		date,
		measurement_moment,
		indicator,
		geometry_ref,
		system_id,
		vehicle_type,
		value
	FROM moment_statistics;
	`

	_, err := db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}

	stmt = `
	INSERT INTO postgres_db.day_statistics
	SELECT
		date,
		indicator,
		geometry_ref,
		system_id,
		vehicle_type,
		value
	FROM day_statistics;
	`

	_, err = db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}
}

func getNewestDateInMomentStatistics(db *pgx.Conn) time.Time {
	var newestDate time.Time
	log.Print("Getting newest date in moment_statistics...")
	err := db.QueryRow(context.Background(), `
		SELECT COALESCE(MAX(date), '2019-12-31'::DATE)
		FROM moment_statistics;
	`).Scan(&newestDate)
	if err != nil {
		log.Fatal(err)
	}
	return newestDate
}

func writeTmpTableToCSV(db *sql.DB) {
	_, err := db.Exec(`
		COPY moment_statistics TO 'moment_stats.csv'
		(HEADER, DELIMITER ',');
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		COPY day_statistics TO 'day_stats.csv'
		(HEADER, DELIMITER ',');
	`)
	if err != nil {
		log.Fatal(err)
	}
}
