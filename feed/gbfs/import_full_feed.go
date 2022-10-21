package gbfs

import (
	"deelfietsdashboard-importer/feed"
	"deelfietsdashboard-importer/process"
	"encoding/json"
	"log"
)

func ImportFullFeed(feed *feed.Feed, dataProcessor process.DataProcessor) []feed.Bike {
	res := feed.DownloadData(feed.Url)
	if res == nil {
		return nil
	}
	decoder := json.NewDecoder(res.Body)
	var gbfsFeed GBFSOverview
	decoder.Decode(&gbfsFeed)

	var freeVehicleUrl string
	var vehicleTypeUrl string
	for _, feed := range gbfsFeed.Data.En.Feeds {
		if feed.Name == "free_bike_status" {
			freeVehicleUrl = feed.URL
		}
		if feed.Name == "vehicle_types" {
			vehicleTypeUrl = feed.URL
		}
	}
	if freeVehicleUrl == "" || vehicleTypeUrl == "" {
		log.Printf("[%s] freeVehicleUrl or vehicleTypeUrl is not filled. Status code: %s", feed.OperatorID, feed.Url)
		return FreeBikeStatus{}.Data.Bikes
	}
	vehicles := getData(feed, freeVehicleUrl)
	vehicleTypes := getVehicleTypes(feed, vehicleTypeUrl, dataProcessor)
	setVehicleTypeOnVehicles(vehicles, vehicleTypes)

	return vehicles
}

type GBFSOverview struct {
	LastUpdated int    `json:"last_updated"`
	TTL         int    `json:"ttl"`
	Version     string `json:"version"`
	Data        struct {
		En struct {
			Feeds []struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			} `json:"feeds"`
		} `json:"en"`
	} `json:"data"`
}

func setVehicleTypeOnVehicles(vehicles []feed.Bike, vehicleTypes []VehicleType) []feed.Bike {
	vehicleTypeMap := make(map[string]VehicleType)

	for _, vehicleType := range vehicleTypes {
		vehicleTypeMap[vehicleType.ExternalVehicleTypeId] = vehicleType
	}

	for index, vehicle := range vehicles {
		if vehicle.ExternalVehicleTypeID == nil {
			continue
		}
		externalVehicleTypeID := *vehicle.ExternalVehicleTypeID
		if vehicleType, ok := vehicleTypeMap[externalVehicleTypeID]; ok {
			vehicles[index].InternalVehicleID = &vehicleType.VehicleTypeId
			vehicles[index].VehicleType = vehicleType.FormFactor
		}
	}
	return vehicles

}

func getVehicleTypes(feed *feed.Feed, url string, dataProcessor process.DataProcessor) []VehicleType {
	dbVehicleTypes := getVehicleTypesFromDB(dataProcessor, feed.OperatorID)
	newVehicleTypes := getVehicleTypesFromApi(feed, url)
	for _, newVehicleType := range newVehicleTypes {
		if !contains(dbVehicleTypes, newVehicleType.ExternalVehicleTypeId) {
			insertedVehicleType := insertVehicleType(newVehicleType, process.InitDataProcessor())
			dbVehicleTypes = append(dbVehicleTypes, insertedVehicleType)
		}
	}
	return dbVehicleTypes
}

func contains(vehicleTypes []VehicleType, externalVehicleTypeId string) bool {
	for _, vehicleType := range vehicleTypes {
		if vehicleType.ExternalVehicleTypeId == externalVehicleTypeId {
			return true
		}
	}
	return false
}

func getVehicleTypesFromApi(feed *feed.Feed, url string) []VehicleType {
	vehicleTypes := []VehicleType{}
	res := feed.DownloadData(url)
	if res == nil {
		return nil
	}
	decoder := json.NewDecoder(res.Body)
	var gbfsVehicleTypeFeed GBFSVehicleTypeFeed
	decoder.Decode(&gbfsVehicleTypeFeed)
	log.Print(gbfsVehicleTypeFeed)

	for _, vehicleType := range gbfsVehicleTypeFeed.Data.VehicleTypes {
		vehicleTypes = append(vehicleTypes, convertGBFSVehicleType(vehicleType, feed.OperatorID))
	}
	return vehicleTypes
}

func convertGBFSVehicleType(gbfsVehicleType GBFSVehicleType, systemId string) VehicleType {
	return VehicleType{
		ExternalVehicleTypeId: gbfsVehicleType.VehicleTypeID,
		FormFactor:            gbfsVehicleType.FormFactor,
		PropulsionType:        gbfsVehicleType.PropulsionType,
		SystemId:              systemId,
		Name:                  gbfsVehicleType.Name,
	}
}

type GBFSVehicleTypeFeed struct {
	LastUpdated int    `json:"last_updated"`
	TTL         int    `json:"ttl"`
	Version     string `json:"version"`
	Data        struct {
		VehicleTypes []GBFSVehicleType `json:"vehicle_types"`
	} `json:"data"`
}

type GBFSVehicleType struct {
	VehicleTypeID  string `json:"vehicle_type_id"`
	FormFactor     string `json:"form_factor"`
	PropulsionType string `json:"propulsion_type"`
	MaxRangeMeters int    `json:"max_range_meters"`
	Name           string `json:"name"`
}

type VehicleType struct {
	VehicleTypeId         int
	ExternalVehicleTypeId string
	FormFactor            string
	PropulsionType        string
	SystemId              string
	Name                  string
}
