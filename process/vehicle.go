package process

import (
	"deelfietsdashboard-importer/feed"
	"log"
	"time"
)

// This function writes all retreived vehicle locations to tile38.
func (processor DataProcessor) VehicleProcessor() {
	for {
		vehicles := <-processor.VehicleChan
		if len(vehicles) > 0 {
			startTime := time.Now()
			processor.importVehicles(vehicles)
			log.Printf("[VehicleImporter] [%s] took %d ms", vehicles[0].SystemID, time.Since(startTime).Milliseconds())
		}
	}
}

func (processor DataProcessor) importVehicles(vehicles []feed.Bike) {
	for _, vehicle := range vehicles {
		vehicleId := vehicle.SystemID + ":" + vehicle.BikeID + ":" + vehicle.VehicleType

		if err := processor.tile38.Keys.Set("vehicles", vehicleId).Point(vehicle.Lat, vehicle.Lon).
			// optional params
			Expiration(75).
			Do(); err != nil {
			log.Print(err)
		}
	}
}
