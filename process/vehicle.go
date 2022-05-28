package process

import (
	"deelfietsdashboard-importer/feed"
	"log"
)

// This function writes all retreived vehicle locations to tile38.
func (processor DataProcessor) VehicleProcessor() {
	for {
		vehicles := <-processor.VehicleChan
		processor.importVehicles(vehicles)
	}
}

func (processor DataProcessor) importVehicles(vehicles []feed.Bike) {
	for _, vehicle := range vehicles {
		vehicleId := vehicle.SystemID + ":" + vehicle.BikeID + ":" + vehicle.VehicleType

		if err := processor.tile38.Keys.Set("vehicles", vehicleId).Point(vehicle.Lat, vehicle.Lon).
			// optional params
			Expiration(45).
			Do(); err != nil {
			log.Print(err)
		}
	}
}
