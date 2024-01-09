package gbfs

import (
	"deelfietsdashboard-importer/feed"
	"encoding/json"
	"log"

	"github.com/jmoiron/sqlx"
)

func importFullFeed(feed *feed.Feed) GBFSOverview {
	res := feed.DownloadData(feed.Url)
	if res == nil {
		return GBFSOverview{}
	}
	decoder := json.NewDecoder(res.Body)
	var gbfsFeed GBFSOverview
	decoder.Decode(&gbfsFeed)
	return gbfsFeed
}

func importFullFeedV3(feed *feed.Feed) GBFSOverviewV3 {
	res := feed.DownloadData(feed.Url)
	if res == nil {
		return GBFSOverviewV3{}
	}
	decoder := json.NewDecoder(res.Body)
	var gbfsFeed GBFSOverviewV3
	decoder.Decode(&gbfsFeed)
	return gbfsFeed
}

func ImportFullFeedVehicles(db *sqlx.DB, feed *feed.Feed) []feed.Bike {
	gbfsFeed := importFullFeed(feed)

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
	vehicleTypes := getVehicleTypes(feed, vehicleTypeUrl, db)
	setVehicleTypeOnVehicles(vehicles, vehicleTypes)

	return vehicles
}

func ImportFullGeofenceV3(dataFeed feed.Feed) GBFSGeofencing {
	gbfsFeed := importFullFeedV3(&dataFeed)

	var geofenceUrl string
	for _, feed := range gbfsFeed.Data.Feeds {
		if feed.Name == "geofencing_zones" {
			geofenceUrl = feed.URL
		}
	}
	if geofenceUrl == "" {
		log.Printf("[%s] geofenceUrl is not filled. Status code: %s", dataFeed.OperatorID, dataFeed.Url)
		return GBFSGeofencing{}
	}
	return ImportGeofence(dataFeed, geofenceUrl)
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

type GBFSOverviewV3 struct {
	LastUpdated int    `json:"last_updated"`
	TTL         int    `json:"ttl"`
	Version     string `json:"version"`
	Data        struct {
		Feeds []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"feeds"`
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

func getVehicleTypes(feed *feed.Feed, url string, db *sqlx.DB) []VehicleType {
	dbVehicleTypes := getVehicleTypesFromDB(db, feed.OperatorID)
	newVehicleTypes := getVehicleTypesFromApi(feed, url)
	for _, newVehicleType := range newVehicleTypes {
		if !contains(dbVehicleTypes, newVehicleType.ExternalVehicleTypeId) {
			insertedVehicleType := insertVehicleType(newVehicleType, db)
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
