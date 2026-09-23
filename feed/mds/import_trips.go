package mds

import (
	"deelfietsdashboard-importer/feed"
	"encoding/json"
	"errors"
	"fmt"
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
	res := feed.DownloadDataAllowTimeout(u, time.Second*60)
	if res == nil {
		return nil, errors.New("something went wrong with importing MDS v1 trips")
	}
	defer res.Body.Close()

	decoder := json.NewDecoder(res.Body)
	var response MdsTripsResponse
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}
	return response.Data.Trips, nil
}
