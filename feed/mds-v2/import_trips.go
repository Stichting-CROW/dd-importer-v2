package mdstwo

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

type MDSTrips struct {
	// last_updated is an integer in the MDS spec but some providers send a
	// string; it is not used, so accept whichever the provider sends.
	LastUpdated json.RawMessage `json:"last_updated"`
	Trips       []Trips         `json:"trips"`
	TTL         int             `json:"ttl"`
	Version     string          `json:"version"`
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
	res, status := feed.DownloadDataTripsWithRetry(u, time.Second*60)
	if res == nil {
		return nil, errors.New("something went wrong with importing trips MDS")
	}
	defer res.Body.Close()
	// Bolt returns 404 for hours it has no trips for; that is not an error.
	if status == http.StatusNotFound && feed.OperatorID == "bolt" {
		log.Printf("[%s_%d] no trips for %s (404), continuing", feed.OperatorID, feed.ID, u)
		return nil, nil
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("importing trips MDS: unexpected status %d", status)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var trips MDSTrips
	if err := json.Unmarshal(body, &trips); err != nil {
		return nil, err
	}
	if len(trips.Trips) == 0 {
		feed.LogZeroRecords(u, body)
	}
	return trips.Trips, nil
}
