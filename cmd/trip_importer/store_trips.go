package main

import (
	"context"
	"deelfietsdashboard-importer/feed"
	mdstwo "deelfietsdashboard-importer/feed/mds-v2"
	"log"

	"github.com/jackc/pgx/v5"
)

func storeTrips(feed *feed.Feed, trips []mdstwo.Trips, conn *pgx.Conn) {
	batch := pgx.Batch{}
	query := `
	INSERT INTO trips
	(system_id, bike_id, start_location, end_location, start_time, 
	end_time, vehicle_type_id, source_feed_id, distance_over_road, trip_source)
	VALUES (@system_id, @bike_id, ST_Point( @start_lng, @start_lat, 4326), ST_Point( @end_lng, @end_lat, 4326),
	TO_TIMESTAMP(@start_time), TO_TIMESTAMP(@end_time),
	@vehicle_type_id, @source_feed_id, @distance_over_road, 'trips')
	`
	for _, trip := range trips {
		batch.Queue(query, pgx.NamedArgs{
			"system_id":          feed.OperatorID,
			"bike_id":            trip.DeviceID,
			"start_lng":          trip.StartLocation.Lng,
			"start_lat":          trip.StartLocation.Lat,
			"end_lng":            trip.EndLocation.Lng,
			"end_lat":            trip.EndLocation.Lat,
			"start_time":         trip.StartTime,
			"end_time":           trip.EndTime,
			"vehicle_type_id":    feed.DefaultVehicleType,
			"source_feed_id":     feed.ID,
			"distance_over_road": trip.Distance,
		})
	}
	err := conn.SendBatch(context.Background(), &batch).Close()
	if err != nil {
		log.Fatalf("Storing trips was not working: %s", err)
	}
}
