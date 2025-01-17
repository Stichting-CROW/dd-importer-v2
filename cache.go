package main

import (
	"crypto/sha256"
	"deelfietsdashboard-importer/feed"
	"deelfietsdashboard-importer/process"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
)

type CachedFeed struct {
	LastUpdated string `json:"last_updated"`
	TTL         int    `json:"ttl"`
	Data        Data   `json:"data"`
}

type Data struct {
	Vehicles []CachedVehicle `json:"vehicles"`
}

type CachedVehicle struct {
	SystemID       string  `json:"system_id"`
	VehicleID      string  `json:"vehicle_id"`
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	IsReserved     bool    `json:"is_reserved"`
	IsDisabled     bool    `json:"is_disabled"`
	FormFactor     *string `json:"form_factor"`
	PropulsionType *string `json:"propulsion_type"`
}

func cacheAvailableVehicles(dataProcessor process.DataProcessor, vehicleArrays chan []feed.Bike) {
	log.Print("Start caching available vehicles.")
	lookupRotatedIDs := getRotatingIDsForOpenParkEvents(dataProcessor)

	var vehicles []CachedVehicle
	for vehicleArray := range vehicleArrays {
		for _, vehicle := range vehicleArray {
			key := fmt.Sprintf("%s:%s", vehicle.SystemID, vehicle.BikeID)
			cachedVehicle, ok := lookupRotatedIDs[key]
			if !ok {
				log.Printf("%s not found during caching", key)
				continue
			}
			cachedVehicle.IsDisabled = vehicle.IsDisabled
			cachedVehicle.IsReserved = vehicle.IsReserved
			cachedVehicle.Lat = vehicle.Lat
			cachedVehicle.Lon = vehicle.Lon
			vehicles = append(vehicles, cachedVehicle)
		}
	}
	res := serializeCachedData(vehicles)
	dataProcessor.Rdb.Set("cached_vehicles", res, -1)

	log.Print("Finished caching available vehicles.")
}

func serializeCachedData(vehicles []CachedVehicle) string {
	res := CachedFeed{
		LastUpdated: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		TTL:         30,
		Data: Data{
			Vehicles: vehicles,
		},
	}
	jsonRes, _ := json.Marshal(res)

	return string(jsonRes)
}

func calculateRotatingID(systemID string, parkEventID int64, salt string) string {
	inputID := fmt.Sprintf("%s:%d:%s", systemID, parkEventID, salt)
	hash := sha256.Sum256([]byte(inputID))

	// Convert hash to hexadecimal string
	hashHex := hex.EncodeToString(hash[:])

	// Get the first 12 characters
	return fmt.Sprintf("%s:%s", systemID, hashHex[:12])
}

func getRotatingIDsForOpenParkEvents(dataProcessor process.DataProcessor) map[string]CachedVehicle {
	openParkEvents := getAllOpenParkEventsFromDB(dataProcessor)
	availableVehiclesIDSalt := os.Getenv("AVAILABLE_VEHICLES_ID_SALT")
	result := map[string]CachedVehicle{}
	for openParkEvents.Next() {
		var vehicle CachedVehicle
		var bikeID string
		var parkEventID int64
		openParkEvents.Scan(&vehicle.SystemID, &bikeID, &parkEventID, &vehicle.FormFactor, &vehicle.PropulsionType)
		vehicle.VehicleID = calculateRotatingID(vehicle.SystemID, parkEventID, availableVehiclesIDSalt)
		result[bikeID] = vehicle
	}
	return result
}

func getAllOpenParkEventsFromDB(dataProcessor process.DataProcessor) *sqlx.Rows {
	stmt := `SELECT park_events.system_id, bike_id, park_event_id, form_factor, propulsion_type
		FROM park_events
		LEFT JOIN vehicle_type
		USING (vehicle_type_id)
		WHERE end_time is null;
	`
	rows, err := dataProcessor.DB.Queryx(stmt)
	if err != nil {
		log.Print(err)
	}
	return rows
}
