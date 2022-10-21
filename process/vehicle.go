package process

import (
	"deelfietsdashboard-importer/feed"
	"log"
	"time"

	"github.com/go-redis/redis"
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
	pipe := processor.tile38.Pipeline()
	for _, vehicle := range vehicles {
		vehicleId := vehicle.SystemID + ":" + vehicle.BikeID + ":" + vehicle.VehicleType
		setCmd := redis.NewStringCmd("SET", "vehicles", vehicleId, "EX", 70, "POINT", vehicle.Lat, vehicle.Lon)
		err := pipe.Process(setCmd)
		if err != nil {
			log.Print(err)
		}
	}

	_, err := pipe.Exec()
	if err != nil {
		log.Print("Something went wrong with writing data to tile38.")
		log.Print(err)
	}
}
