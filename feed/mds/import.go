// This package handles the import of the /vehicles endpoint of MDS.
package mds

import (
	"deelfietsdashboard-importer/feed"
	"encoding/json"
	"log"
	"strings"
)

type MdsVehiclesRespone struct {
	Version string `json:"version"`
	Data    struct {
		Vehicle []MdsVehicle `json:"vehicles"`
	} `json:"data"`
	LastUpdated int64 `json:"last_updated"`
	TTL         int   `json:"ttl"`
}

type MdsVehicle struct {
	CurrentLocation struct {
		Type       string `json:"type"`
		Properties struct {
			Timestamp int64 `json:"timestamp"`
		} `json:"properties"`
		Geometry struct {
			Type        string    `json:"type"`
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
	} `json:"current_location"`
	LastEventLocation struct {
		Type       string `json:"type"`
		Properties struct {
			Timestamp int64 `json:"timestamp"`
		} `json:"properties"`
		Geometry struct {
			Type        string    `json:"type"`
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
	} `json:"last_event_location"`
	ProviderName     string   `json:"provider_name"`
	ProviderID       string   `json:"provider_id"`
	VehicleID        string   `json:"vehicle_id"`
	LastEventTime    int64    `json:"last_event_time"`
	DeviceID         string   `json:"device_id"`
	PropulsionTypes  []string `json:"propulsion_types"`
	VehicleType      string   `json:"vehicle_type"`
	LastVehicleState string   `json:"last_vehicle_state"`
	LastEventTypes   []string `json:"last_event_types"`
}

func ImportFeed(feed *feed.Feed) []feed.Bike {
	feed.NumberOfPulls = feed.NumberOfPulls + 1
	return getData(feed)
}

func getData(feed *feed.Feed) []feed.Bike {
	res := feed.DownloadData()
	log.Print(res)
	if res == nil {
		return nil
	}

	decoder := json.NewDecoder(res.Body)
	var mdsFeed MdsVehiclesRespone
	decoder.Decode(&mdsFeed)
	return convertMds(mdsFeed, feed.OperatorID)
}

func convertMds(mdsFeed MdsVehiclesRespone, systemID string) []feed.Bike {
	vehicles := []feed.Bike{}
	for _, mdsVehicle := range mdsFeed.Data.Vehicle {
		vehicle := convertMdsToVehicle(mdsVehicle, systemID)
		vehicles = append(vehicles, vehicle)
	}
	return vehicles
}

func convertMdsToVehicle(mdsVehicle MdsVehicle, systemID string) feed.Bike {
	isDisabled := mdsVehicle.LastVehicleState == "non_operational"
	isReserved := mdsVehicle.LastVehicleState == "reserved"
	return feed.Bike{
		BikeID:     convertToVehicleId(mdsVehicle.VehicleID),
		Lat:        mdsVehicle.CurrentLocation.Geometry.Coordinates[1],
		Lon:        mdsVehicle.CurrentLocation.Geometry.Coordinates[0],
		IsReserved: isReserved,
		IsDisabled: isDisabled,
		SystemID:   systemID,
	}
}

func convertToVehicleId(vehicleID string) string {
	return strings.ToLower(strings.Replace(vehicleID, " ", "_", -1))
}
