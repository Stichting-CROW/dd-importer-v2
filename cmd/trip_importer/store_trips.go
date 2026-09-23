package main

import (
	"context"
	"deelfietsdashboard-importer/feed"
	mdstwo "deelfietsdashboard-importer/feed/mds-v2"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

func storeTrips(feed *feed.Feed, trips []mdstwo.Trips, conn *pgx.Conn) error {
	query := `
	INSERT INTO trips
	(system_id, bike_id, start_location, end_location, start_time, 
	end_time, vehicle_type_id, source_feed_id, distance_over_road, trip_source)
	VALUES (@system_id, @bike_id, ST_Point( @start_lng, @start_lat, 4326), ST_Point( @end_lng, @end_lat, 4326),
	TO_TIMESTAMP(@start_time / 1000.0), TO_TIMESTAMP(@end_time / 1000.0),
	@vehicle_type_id, @source_feed_id, @distance_over_road, 'trips')
	`

	const maxRetries = 3
	baseDelay := 1 * time.Second
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<uint(attempt-1))
			log.Printf("Retrying storing trips (attempt %d/%d) after %v", attempt+1, maxRetries, delay)
			time.Sleep(delay)
		}

		batch := pgx.Batch{}
		for _, trip := range trips {
			vehicleTypeID := feed.DefaultVehicleType
			if trip.VehicleTypeID != nil {
				vehicleTypeID = trip.VehicleTypeID
			}
			batch.Queue(query, pgx.NamedArgs{
				"system_id":          feed.OperatorID,
				"bike_id":            trip.DeviceID,
				"start_lng":          trip.StartLocation.Lng,
				"start_lat":          trip.StartLocation.Lat,
				"end_lng":            trip.EndLocation.Lng,
				"end_lat":            trip.EndLocation.Lat,
				"start_time":         trip.StartTime,
				"end_time":           trip.EndTime,
				"vehicle_type_id":    vehicleTypeID,
				"source_feed_id":     feed.ID,
				"distance_over_road": trip.Distance,
			})
		}

		err := conn.SendBatch(context.Background(), &batch).Close()
		if err == nil {
			return nil
		}
		lastErr = err
		log.Printf("Storing trips failed (attempt %d/%d): %s", attempt+1, maxRetries, err)
	}
	return lastErr
}
