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
	Links       Links `json:"links"`
}

type Links struct {
	First string `json:"first"`
	Next  string `json:"next"`
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
	return getDataRecursively(feed.Url, feed, 0)
}

func getDataRecursively(url string, feed *feed.Feed, counter int) []feed.Bike {
	if counter > 50 {
		log.Printf("too much recursion >50, this indicates a problem %s.", url)
		return nil
	}

	res := feed.DownloadData(url)
	if res == nil {
		return nil
	}

	decoder := json.NewDecoder(res.Body)
	var mdsFeed MdsVehiclesRespone
	decoder.Decode(&mdsFeed)

	vehicles := convertMds(mdsFeed, feed.OperatorID)
	if mdsFeed.Links.Next != "" {
		vehicles = append(vehicles, getDataRecursively(mdsFeed.Links.Next, feed, counter+1)...)
	}
	return vehicles
}

func convertMds(mdsFeed MdsVehiclesRespone, systemID string) []feed.Bike {
	vehicles := []feed.Bike{}
	for _, mdsVehicle := range mdsFeed.Data.Vehicle {
		if mdsVehicle.LastVehicleState == "available" ||
			mdsVehicle.LastVehicleState == "non_operational" ||
			mdsVehicle.LastVehicleState == "reserved" ||
			mdsVehicle.LastVehicleState == "elsewhere" {
			vehicle := convertMdsToVehicle(mdsVehicle, systemID)
			vehicles = append(vehicles, vehicle)
		}

	}
	return vehicles
}

func convertMdsToVehicle(mdsVehicle MdsVehicle, systemID string) feed.Bike {
	isDisabled := mdsVehicle.LastVehicleState == "non_operational"
	isReserved := mdsVehicle.LastVehicleState == "reserved"
	if len(mdsVehicle.CurrentLocation.Geometry.Coordinates) == 0 {
		mdsVehicle.CurrentLocation.Geometry = mdsVehicle.LastEventLocation.Geometry
	}

	return feed.Bike{
		BikeID:            convertToVehicleId(mdsVehicle.VehicleID),
		Lat:               mdsVehicle.CurrentLocation.Geometry.Coordinates[1],
		Lon:               mdsVehicle.CurrentLocation.Geometry.Coordinates[0],
		IsReserved:        isReserved,
		IsDisabled:        isDisabled,
		SystemID:          systemID,
		InternalVehicleID: convertVehicleType(mdsVehicle.VehicleType, mdsVehicle.PropulsionTypes),
		VehicleType:       mdsVehicle.VehicleType,
	}
}

func convertToVehicleId(vehicleID string) string {
	return strings.ToLower(strings.Replace(vehicleID, " ", "_", -1))
}

func convertVehicleType(vehicleType string, propulsionTypes []string) *int {
	vehicleTypePropulsionType := vehicleType
	if len(propulsionTypes) > 0 {
		vehicleTypePropulsionType = vehicleTypePropulsionType + ":" + propulsionTypes[0]
	}

	// Hardcoded values from DB.
	defaultVehicleType := map[string]int{
		"bicycle:human":                 5,
		"bicycle:electric_assist":       4,
		"cargo_bicycle:electric_assist": 2,
		"moped:electric":                1,
		"car:electric":                  20,
		"car:combustion":                21,
	}
	result, ok := defaultVehicleType[vehicleTypePropulsionType]
	if !ok {
		return nil
	}
	return &result

}
