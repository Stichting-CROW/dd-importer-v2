package main

import (
	"context"
	"deelfietsdashboard-importer/feed"
	mdstwo "deelfietsdashboard-importer/feed/mds-v2"
	"deelfietsdashboard-importer/process"
	"log"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	log.Print("Start trip_importer")
	dataProcessor := process.InitDataProcessor()

	conn, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Printf("Something went wrong while connecting with database %s\n", err)
	}
	feeds := process.LoadTripFeeds(dataProcessor)
	log.Print(feeds)
	data := downloadFeeds(feeds, conn)
	log.Print(data)
	// dataProcessor.ProcessGeofences(data)
}

func getLatestImportTime(feed feed.Feed, db *pgx.Conn) time.Time {
	stmt := `
	SELECT MAX(end_time)
	FROM trips
	WHERE source_feed_id = @feed_id;
	`

	var latestImportTime time.Time
	db.QueryRow(context.Background(), stmt, pgx.NamedArgs{
		"feed_id": feed.ID,
	}).Scan(&latestImportTime)

	// import trips after 2024-04-01
	minimalStartDate := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	if minimalStartDate.After(latestImportTime) {
		return minimalStartDate
	}
	latestImportTime = latestImportTime.Add(1 * time.Hour)
	latestImportTime = latestImportTime.Truncate(time.Hour)

	return latestImportTime
}

func downloadFeeds(feeds []feed.Feed, conn *pgx.Conn) []mdstwo.Trips {
	res := []mdstwo.Trips{}
	for _, dataFeed := range feeds {
		switch dataFeed.Type {
		case "mds-trips-v2":
			latesImport := getLatestImportTime(dataFeed, conn)
			loadDataUntilNow(&dataFeed, latesImport, conn)
			// res = append(res, mdstwo.ImportTripFeed(&dataFeed)...)
		default:
			log.Printf("NOT SUPPORTED: %s", dataFeed.Type)
		}
	}
	return res
}

func loadDataUntilNow(feed *feed.Feed, latestImport time.Time, conn *pgx.Conn) {
	timeCursor := latestImport
	for {
		var toRequest []string
		toRequest, timeCursor = getTimestampsToRequest(timeCursor)
		if len(toRequest) == 0 {
			return
		}
		trips := getDataForTimestamps(feed, toRequest)
		storeTrips(feed, trips, conn)

	}
}

func getDataForTimestamps(feed *feed.Feed, toRequest []string) []mdstwo.Trips {
	jobQueue := make(chan string, len(toRequest))
	response := make(chan []mdstwo.Trips, len(toRequest))
	var wg sync.WaitGroup

	// Start the workers
	for i := 1; i <= 4; i++ {
		wg.Add(1)
		go getDataForTimestampsWorker(feed, jobQueue, response, &wg)
	}

	// Enqueue jobs
	for _, timestamp := range toRequest {
		jobQueue <- timestamp
	}

	close(jobQueue)
	wg.Wait()
	close(response)

	var trips []mdstwo.Trips
	for responseItem := range response {
		trips = append(trips, responseItem...)
	}
	log.Printf("Storing %d trips in database", len(trips))
	return trips
}

func getDataForTimestampsWorker(feed *feed.Feed, jobQueue <-chan string, response chan<- []mdstwo.Trips, wg *sync.WaitGroup) {
	defer wg.Done()
	for timestamp := range jobQueue {
		data := mdstwo.ImportTrips(feed, timestamp)
		log.Printf("timestamp %s contained %d records", timestamp, len(data))
		response <- mdstwo.ImportTrips(feed, timestamp)
	}
}

func getTimestampsToRequest(startTime time.Time) ([]string, time.Time) {
	var toRequest []string
	counter := 0
	for counter < 100 && startTime.Before(time.Now().UTC().Truncate(time.Hour)) {
		toRequest = append(toRequest, startTime.Format("2006-01-02T15"))
		startTime = startTime.Add(1 * time.Hour)
		counter += 1
	}
	return toRequest, startTime
}
