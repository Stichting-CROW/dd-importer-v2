package mds

import (
	"deelfietsdashboard-importer/feed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type MdsTripsResponse struct {
	Version     string `json:"version"`
	LastUpdated int64  `json:"last_updated"`
	TTL         int    `json:"ttl"`
	Data        struct {
		Trips []Trips `json:"trips"`
	} `json:"data"`
}

type Trips struct {
	ProviderID      string   `json:"provider_id"`
	ProviderName    string   `json:"provider_name"`
	DeviceID        string   `json:"device_id"`
	VehicleID       string   `json:"vehicle_id"`
	VehicleType     string   `json:"vehicle_type"`
	PropulsionTypes []string `json:"propulsion_types"`
	TripID          string   `json:"trip_id"`
	TripDuration    int      `json:"trip_duration"`
	TripDistance    int      `json:"trip_distance"`
	Route           Route    `json:"route"`
	Accuracy        int      `json:"accuracy"`
	StartTime       int      `json:"start_time"`
	EndTime         int      `json:"end_time"`
}

type Route struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

type Feature struct {
	Type       string          `json:"type"`
	Geometry   FeatureGeometry `json:"geometry"`
	Properties map[string]any  `json:"properties"`
}

type FeatureGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

func ImportTrips(feed *feed.Feed, timestamp string) ([]Trips, error) {
	feed.NumberOfPulls = feed.NumberOfPulls + 1
	u := fmt.Sprintf("%s?end_time=%s", feed.Url, timestamp)
	res, status := feed.DownloadDataTripsWithRetry(u, time.Second*60)
	if res == nil {
		return nil, errors.New("something went wrong with importing MDS v1 trips")
	}
	defer res.Body.Close()

	// Bolt returns 404 for hours it has no trips for; that is not an error.
	if status == http.StatusNotFound && feed.OperatorID == "bolt" {
		log.Printf("[%s_%d] no trips for %s (404), continuing", feed.OperatorID, feed.ID, u)
		return nil, nil
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("importing MDS v1 trips: unexpected status %d", status)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var response MdsTripsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if len(response.Data.Trips) == 0 {
		feed.LogZeroRecords(u, body)
	}
	return response.Data.Trips, nil
}
