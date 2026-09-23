package mdstwo

import (
	"deelfietsdashboard-importer/feed"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type MDSTrips struct {
	LastUpdated string  `json:"last_updated"`
	Trips       []Trips `json:"trips"`
	TTL         int     `json:"ttl"`
	Version     string  `json:"version"`
}

type EndLocation struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type StartLocation struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type Trips struct {
	DeviceID      string        `json:"device_id"`
	Distance      int           `json:"distance"`
	Duration      int           `json:"duration"`
	EndLocation   EndLocation   `json:"end_location"`
	EndTime       int           `json:"end_time"`
	ProviderID    string        `json:"provider_id"`
	ProviderName  string        `json:"provider_name"`
	StartLocation StartLocation `json:"start_location"`
	StartTime     int           `json:"start_time"`
	TripID        string        `json:"trip_id"`
	VehicleTypeID *int          `json:"-"`
}

func ImportTrips(feed *feed.Feed, timestamp string) ([]Trips, error) {
	feed.NumberOfPulls = feed.NumberOfPulls + 1
	u := fmt.Sprintf("%s?end_time=%s", feed.Url, timestamp)
	return getTrips(feed, u)
}

func getTrips(feed *feed.Feed, u string) ([]Trips, error) {
	res := feed.DownloadDataAllowTimeout(u, time.Second*60)
	if res == nil {
		return nil, errors.New("something went wrong with importing trips MDS")
	}
	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)
	var trips MDSTrips
	if err := decoder.Decode(&trips); err != nil {
		return nil, err
	}
	return trips.Trips, nil
}
