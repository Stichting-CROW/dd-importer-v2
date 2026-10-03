package main

import (
	"context"
	"deelfietsdashboard-importer/feed"
	"deelfietsdashboard-importer/feed/mds"
	mdstwo "deelfietsdashboard-importer/feed/mds-v2"
	"deelfietsdashboard-importer/process"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// importLag is how long after an hour finishes we wait before importing it, so
// providers have time to finalize their trip data.
const importLag = time.Hour

// feedLabel identifies a feed in logs and errors. A feed is always identified
// by both its feed_id and its system_id, so they are reported together.
func feedLabel(f feed.Feed) string {
	return fmt.Sprintf("feed_id=%d system_id=%q type=%s", f.ID, f.OperatorID, f.Type)
}

func main() {
	log.Print("Start trip_importer")
	dataProcessor := process.InitDataProcessor()

	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Something went wrong while connecting with database %s\n", err)
	}
	defer pool.Close()
	feeds := process.LoadTripFeeds(dataProcessor)
	for _, dataFeed := range feeds {
		log.Printf("Loaded trip feed: %s", feedLabel(dataFeed))
	}
	if err := downloadFeeds(feeds, pool); err != nil {
		log.Printf("Some feeds failed to import, continuing: %s", err)
	}
}

func getLatestImportTime(feed feed.Feed, db *pgxpool.Pool) time.Time {
	stmt := `
	SELECT MAX(end_time)
	FROM trips
	WHERE source_feed_id = @feed_id;
	`

	var latestImportTime time.Time
	db.QueryRow(context.Background(), stmt, pgx.NamedArgs{
		"feed_id": feed.ID,
	}).Scan(&latestImportTime)

	// import trips after 2026-10-01
	minimalStartDate := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if minimalStartDate.After(latestImportTime) {
		return minimalStartDate
	}
	latestImportTime = latestImportTime.Add(1 * time.Hour)
	latestImportTime = latestImportTime.Truncate(time.Hour)

	// pgx returns timestamptz in the process local zone; the API and the rest of
	// the import logic work in UTC.
	return latestImportTime.UTC()
}

func downloadFeeds(feeds []feed.Feed, pool *pgxpool.Pool) error {
	// Feeds that share a system_id hit the same provider endpoint, so loading
	// them at the same time can trigger rate limiting (429). Group them and load
	// each group sequentially; different system_ids still run in parallel.
	feedsBySystem := make(map[string][]feed.Feed)
	for _, dataFeed := range feeds {
		if dataFeed.OperatorID == "" {
			log.Printf("Skipping feed_id=%d: missing system_id", dataFeed.ID)
			continue
		}
		feedsBySystem[dataFeed.OperatorID] = append(feedsBySystem[dataFeed.OperatorID], dataFeed)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(feeds))

	for _, group := range feedsBySystem {
		group := group
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, dataFeed := range group {
				switch dataFeed.Type {
				case "mds-trips-v1", "mds-trips-v2":
					latestImport := getLatestImportTime(dataFeed, pool)
					if err := loadDataUntilNow(&dataFeed, latestImport, pool); err != nil {
						errCh <- fmt.Errorf("%s: %w", feedLabel(dataFeed), err)
					}
				default:
					log.Printf("NOT SUPPORTED: %s", feedLabel(dataFeed))
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func loadDataUntilNow(feed *feed.Feed, latestImport time.Time, pool *pgxpool.Pool) error {
	// Trip data is only complete once a provider has finalized it, which takes
	// up to an hour. So only request hour buckets that finished at least
	// `importLag` ago: `[cursor, cutoff)` holds exactly those buckets. This also
	// means a restart within the same hour never requests the running hour.
	cutoff := importCutoff(time.Now(), importLag)

	// A cursor derived from the last stored end_time could point at or past the
	// cutoff; clamp it so a not-yet-finalized hour is never requested.
	timeCursor := latestImport
	if timeCursor.After(cutoff) {
		timeCursor = cutoff
	}

	for {
		var toRequest []string
		toRequest, timeCursor = getTimestampsToRequest(timeCursor, cutoff)
		if len(toRequest) == 0 {
			return nil
		}
		trips, err := getDataForTimestamps(feed, toRequest)
		if err != nil {
			return err
		}
		if err := storeTrips(feed, trips, pool); err != nil {
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
	for i := 1; i <= 2; i++ {
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
		log.Printf("%s: timestamp %s contained %d records", feedLabel(*feed), timestamp, len(data))
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

// importCutoff returns the exclusive upper bound for hour buckets that are safe
// to import. Every hour bucket starting before the returned time finished at
// least `lag` ago; buckets at or after it are still too recent to be complete.
func importCutoff(now time.Time, lag time.Duration) time.Time {
	return now.UTC().Add(-lag).Truncate(time.Hour)
}

func getTimestampsToRequest(startTime time.Time, until time.Time) ([]string, time.Time) {
	// The API expects UTC timestamps; format in UTC regardless of the location
	// the time was scanned in.
	startTime = startTime.UTC()
	var toRequest []string
	counter := 0
	for counter < 100 && startTime.Before(until) {
		toRequest = append(toRequest, startTime.Format("2006-01-02T15"))
		startTime = startTime.Add(1 * time.Hour)
		counter += 1
	}
	return toRequest, startTime
}
