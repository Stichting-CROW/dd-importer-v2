package main

import (
	"database/sql"
	"deelfietsdashboard-importer/cmd/batch_processing/analyze"
	"log"
	"time"
)

func main() {
	log.Print("Starting batch processing...")
	// dbURL := os.Getenv("DB_URL")
	dConn := initDuckDB()
	startTime := time.Now()

	// for every month in 2025
	year := 2025
	for m := time.January; m <= time.January; m++ {
		startLoadingMonth := time.Now()
		log.Printf("Loading park events for month %s", m.String())
		lastDayOfMonth := time.Date(year, m+1, 0, 0, 0, 0, 0, time.Now().Location())
		log.Printf("Analyzing month %s", m.String())
		// how to get last day of month?

		// load park events between first and last day of month because that is more efficient then loading a full year
		loadParkEventInBetween(dConn, time.Date(year, m, 1, 0, 0, 0, 0, time.Now().Location()), lastDayOfMonth)

		for d := time.Date(year, m, 1, 0, 0, 0, 0, time.Now().Location()); !d.After(lastDayOfMonth); d = d.AddDate(0, 0, 1) {
			log.Printf("Last day of month: %s, %s", d, lastDayOfMonth)

			log.Printf("Analyzing date %s", d.Format("2006-01-02"))
			analyzeDay(dConn, d)
		}
		log.Printf("Done analyzing for month %s, took %s", m.String(), time.Since(startLoadingMonth))
	}
	analyze.AggregateVehiclesInPublicSpacePerDay(dConn)
	analyze.AggregateWronglyParkedVehiclesPerDay(dConn, time.Date(year, time.January, 1, 0, 0, 0, 0, time.Now().Location()), time.Date(year, time.January, 31, 0, 0, 0, 0, time.Now().Location()))
	writeTmpTableToCSV(dConn)

	log.Printf("Done analyzing data, took %s", time.Since(startTime))
}

func analyzeDay(dConn *sql.DB, date time.Time) {
	loadZones(dConn)
	//loadParkEventOnDate(dConn, date)
	analyze.FindIntersectionsWithZones(dConn)

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
	analyze.CountWronglyParkedVehicles(dConn, date)
	analyze.CountVehiclesInPublicSpaceOnDate(dConn, measurementMoment)
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
