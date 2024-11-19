// This package handles the import of the /vehicles endpoint of MDS.
package mdstwo

import (
	"deelfietsdashboard-importer/feed"
	"encoding/json"
	"errors"
	"log"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

type MdsVehiclesResponseV2 struct {
	LastUpdated int `json:"last_updated"`
	Links       struct {
		First string `json:"first"`
		Last  string `json:"last"`
		Next  string `json:"next"`
		Prev  any    `json:"prev"`
	} `json:"links"`
	TTL      int            `json:"ttl"`
	Vehicles []MdsVehicleV2 `json:"vehicles"`
	Version  string         `json:"version"`
}

type MdsVehicleV2 struct {
	DeviceID        string   `json:"device_id"`
	MaximumSpeed    int      `json:"maximum_speed"`
	PropulsionTypes []string `json:"propulsion_types"`
	ProviderID      string   `json:"provider_id"`
	ProviderName    string   `json:"provider_name"`
	VehicleID       string   `json:"vehicle_id"`
	VehicleType     string   `json:"vehicle_type"`
}

type MdsVehiclesStatusResponseV2 struct {
	LastUpdated string `json:"last_updated"`
	Links       struct {
		First string      `json:"first"`
		Last  string      `json:"last"`
		Next  string      `json:"next"`
		Prev  interface{} `json:"prev"`
	} `json:"links"`
	TTL            int                `json:"ttl"`
	VehiclesStatus []MdsVehicleStatus `json:"vehicles_status"`
	Version        string             `json:"version"`
}

type MdsVehicleStatus struct {
	DeviceID  string `json:"device_id"`
	LastEvent struct {
		DeviceID     string `json:"device_id"`
		ProviderID   string `json:"provider_id"`
		VehicleState string `json:"vehicle_state"`
	} `json:"last_event"`
	LastTelemetry struct {
		DeviceID string `json:"device_id"`
		Location struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"location"`
		ProviderID string `json:"provider_id"`
		Timestamp  string `json:"timestamp"`
	} `json:"last_telemetry"`
	ProviderID string `json:"provider_id"`
}

func getUrl(url_string string, offset int, limit int) string {
	u, _ := url.Parse(url_string)
	q := u.Query()
	q.Set("page[offset]", strconv.Itoa(offset))
	q.Set("page[limit]", strconv.Itoa(limit))
	u.RawQuery = q.Encode()
	return u.String()
}

func getVehicles(feed *feed.Feed, u string) (MdsVehiclesResponseV2, error) {
	res := feed.DownloadData(u)
	if res == nil {
		return MdsVehiclesResponseV2{}, errors.New("something went wrong with getting vehicles")
	}
	decoder := json.NewDecoder(res.Body)
	var mdsFeed MdsVehiclesResponseV2
	decoder.Decode(&mdsFeed)
	return mdsFeed, nil
}

func getVehicleStatus(feed *feed.Feed, u string) (MdsVehiclesStatusResponseV2, error) {
	res := feed.DownloadData(u)
	if res == nil {
		return MdsVehiclesStatusResponseV2{}, errors.New("something went wrong with getting vehicle status")
	}
	decoder := json.NewDecoder(res.Body)
	var mdsFeedStatus MdsVehiclesStatusResponseV2
	decoder.Decode(&mdsFeedStatus)
	return mdsFeedStatus, nil
}

func DownloadData(f *feed.Feed) []feed.Bike {
	u, _ := url.Parse(f.Url)
	q := u.Query()
	q.Set("size", strconv.Itoa(100000))
	u.RawQuery = q.Encode()
	vehiclesUrl := u.String()

	u, _ = url.Parse(f.Url + "/status")
	q = u.Query()
	q.Set("size", strconv.Itoa(100000))
	u.RawQuery = q.Encode()
	vehiclesStatusUrl := u.String()

	mdsVehicles, err := getVehicles(f, vehiclesUrl)
	if err != nil {
		log.Print(err)
		return []feed.Bike{}
	}
	mdsVehicleStatus, err := getVehicleStatus(f, vehiclesStatusUrl)
	if err != nil {
		log.Print(err)
		return []feed.Bike{}
	}
	return combineFeeds(mdsVehicles.Vehicles, mdsVehicleStatus.VehiclesStatus, f.OperatorID)

}

func DownloadDataPaginated(f *feed.Feed, limit int) []feed.Bike {
	// This makes it possible to paralyze this call in the future
	u := getUrl(f.Url, 0, 1)
	mdsFeed, err := getVehicles(f, u)
	if err != nil {
		return []feed.Bike{}
	}

	last_url, _ := url.Parse(mdsFeed.Links.Last)
	q := last_url.Query()
	if q.Get("page[offset]") == "" {
		return nil
	}
	number_of_vehicles, _ := strconv.Atoi(q.Get("page[offset]"))

	offset_index := 0
	vehicles := []MdsVehicleV2{}
	vehicleStatuses := []MdsVehicleStatus{}
	for offset_index < number_of_vehicles {
		u = getUrl(f.Url, offset_index, limit)
		mdsVehicles, err := getVehicles(f, u)
		if err != nil {
			return []feed.Bike{}
		}
		vehicles = append(vehicles, mdsVehicles.Vehicles...)

		u = getUrl(f.Url+"/status", offset_index, limit)
		mdsVehicleStatus, err := getVehicleStatus(f, u)
		if err != nil {
			return []feed.Bike{}
		}
		vehicleStatuses = append(vehicleStatuses, mdsVehicleStatus.VehiclesStatus...)

		offset_index = offset_index + limit
	}
	return combineFeeds(vehicles, vehicleStatuses, f.OperatorID)
}

func combineFeeds(vehicles []MdsVehicleV2, vehicleStatuses []MdsVehicleStatus, operatorID string) []feed.Bike {
	vehiclesLookup := make(map[string]MdsVehicleV2)

	// Convert array to hashmap
	for _, vehicle := range vehicles {
		vehiclesLookup[vehicle.DeviceID] = vehicle
	}

	res := []feed.Bike{}
	for _, vehicleStatus := range vehicleStatuses {
		vehicleState := vehicleStatus.LastEvent.VehicleState
		if !slices.Contains([]string{"available", "non_operational", "reserved", "elsewhere"}, vehicleState) {
			continue
		}

		if vehicle, found := vehiclesLookup[vehicleStatus.DeviceID]; found {
			res = append(res, convertMdsToVehicle(vehicle, vehicleStatus, operatorID))
		} else {
			log.Printf("vehicle with device_id %s could not be found, complete feed is now ingnored", vehicleStatus.DeviceID)
			return []feed.Bike{}
		}
	}

	return res
}

func ImportFeed(feed *feed.Feed) []feed.Bike {
	feed.NumberOfPulls = feed.NumberOfPulls + 1
	return getData(feed)
}

func getData(feed *feed.Feed) []feed.Bike {
	if feed.OperatorID == "check" {
		return DownloadDataPaginated(feed, 250)
	}
	return DownloadData(feed)
}

func convertMdsToVehicle(vehicle MdsVehicleV2, vehicleStatus MdsVehicleStatus, systemID string) feed.Bike {
	isDisabled := vehicleStatus.LastEvent.VehicleState == "non_operational"
	isReserved := vehicleStatus.LastEvent.VehicleState == "reserved"

	return feed.Bike{
		BikeID:            convertToVehicleId(vehicle.VehicleID),
		Lat:               vehicleStatus.LastTelemetry.Location.Lat,
		Lon:               vehicleStatus.LastTelemetry.Location.Lng,
		IsReserved:        isReserved,
		IsDisabled:        isDisabled,
		SystemID:          systemID,
		InternalVehicleID: convertVehicleType(vehicle.VehicleType, vehicle.PropulsionTypes),
		VehicleType:       vehicle.VehicleType,
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
	}
	result, ok := defaultVehicleType[vehicleTypePropulsionType]
	if !ok {
		return nil
	}
	return &result

}
