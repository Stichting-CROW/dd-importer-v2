package main

import (
	"context"
	"deelfietsdashboard-importer/feed"
	"deelfietsdashboard-importer/feed/mds"
	mdstwo "deelfietsdashboard-importer/feed/mds-v2"
	"deelfietsdashboard-importer/process"
	"fmt"
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
		log.Fatalf("Something went wrong while connecting with database %s\n", err)
	}
	feeds := process.LoadTripFeeds(dataProcessor)
	log.Print(feeds)
	if err := downloadFeeds(feeds, conn); err != nil {
		log.Fatalf("Importing trips failed: %s", err)
	}
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

	// import trips after 2025-01-01
	minimalStartDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if minimalStartDate.After(latestImportTime) {
		return minimalStartDate
	}
	latestImportTime = latestImportTime.Add(1 * time.Hour)
	latestImportTime = latestImportTime.Truncate(time.Hour)

	return latestImportTime
}

func downloadFeeds(feeds []feed.Feed, conn *pgx.Conn) error {
	for _, dataFeed := range feeds {
		switch dataFeed.Type {
		case "mds-trips-v1", "mds-trips-v2":
			latestImport := getLatestImportTime(dataFeed, conn)
			if err := loadDataUntilNow(&dataFeed, latestImport, conn); err != nil {
				return err
			}
		default:
			log.Printf("NOT SUPPORTED: %s", dataFeed.Type)
		}
	}
	return nil
}

func loadDataUntilNow(feed *feed.Feed, latestImport time.Time, conn *pgx.Conn) error {
	timeCursor := latestImport
	for {
		var toRequest []string
		toRequest, timeCursor = getTimestampsToRequest(timeCursor)
		if len(toRequest) == 0 {
			return nil
		}
		trips, err := getDataForTimestamps(feed, toRequest)
		if err != nil {
			return err
		}
		if err := storeTrips(feed, trips, conn); err != nil {
			return err
		}
	}
}

func getDataForTimestamps(feed *feed.Feed, toRequest []string) ([]mdstwo.Trips, error) {
	jobQueue := make(chan string, len(toRequest))
	response := make(chan []mdstwo.Trips, len(toRequest))
	errCh := make(chan error, len(toRequest))
	var wg sync.WaitGroup

	// Start the workers
	for i := 1; i <= 4; i++ {
		wg.Add(1)
		go getDataForTimestampsWorker(feed, jobQueue, response, errCh, &wg)
	}

	// Enqueue jobs
	for _, timestamp := range toRequest {
		jobQueue <- timestamp
	}

	close(jobQueue)
	wg.Wait()
	close(response)
	close(errCh)

	if err := <-errCh; err != nil {
		return nil, err
	}

	var trips []mdstwo.Trips
	for responseItem := range response {
		trips = append(trips, responseItem...)
	}
	return trips, nil
}

func getDataForTimestampsWorker(feed *feed.Feed, jobQueue <-chan string, response chan<- []mdstwo.Trips, errCh chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	for timestamp := range jobQueue {
		data, err := getTripsForFeed(feed, timestamp)
		if err != nil {
			errCh <- err
			return
		}
		log.Printf("timestamp %s contained %d records", timestamp, len(data))
		response <- data
	}
}

func getTripsForFeed(feed *feed.Feed, timestamp string) ([]mdstwo.Trips, error) {
	switch feed.Type {
	case "mds-trips-v1":
		trips, err := mds.ImportTrips(feed, timestamp)
		if err != nil {
			return nil, err
		}
		return convertMdsV1Trips(trips), nil
	case "mds-trips-v2":
		return mdstwo.ImportTrips(feed, timestamp)
	default:
		return nil, fmt.Errorf("NOT SUPPORTED: %s", feed.Type)
	}
}

func convertMdsV1Trips(trips []mds.Trips) []mdstwo.Trips {
	res := make([]mdstwo.Trips, 0, len(trips))
	for _, trip := range trips {
		start, ok := startLocationFromRoute(trip.Route)
		if !ok {
			continue
		}
		end, ok := endLocationFromRoute(trip.Route)
		if !ok {
			continue
		}
		res = append(res, mdstwo.Trips{
			DeviceID:      trip.DeviceID,
			Distance:      trip.TripDistance,
			Duration:      trip.TripDuration,
			EndLocation:   end,
			EndTime:       trip.EndTime,
			ProviderID:    trip.ProviderID,
			ProviderName:  trip.ProviderName,
			StartLocation: start,
			StartTime:     trip.StartTime,
			TripID:        trip.TripID,
			VehicleTypeID: mds.ConvertVehicleType(trip.VehicleType, trip.PropulsionTypes),
		})
	}
	return res
}

func startLocationFromRoute(route mds.Route) (mdstwo.StartLocation, bool) {
	if len(route.Features) == 0 {
		return mdstwo.StartLocation{}, false
	}
	coords := route.Features[0].Geometry.Coordinates
	if len(coords) < 2 {
		return mdstwo.StartLocation{}, false
	}
	return mdstwo.StartLocation{Lat: coords[1], Lng: coords[0]}, true
}

func endLocationFromRoute(route mds.Route) (mdstwo.EndLocation, bool) {
	if len(route.Features) == 0 {
		return mdstwo.EndLocation{}, false
	}
	coords := route.Features[len(route.Features)-1].Geometry.Coordinates
	if len(coords) < 2 {
		return mdstwo.EndLocation{}, false
	}
	return mdstwo.EndLocation{Lat: coords[1], Lng: coords[0]}, true
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
