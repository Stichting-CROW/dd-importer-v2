package process

import (
	"deelfietsdashboard-importer/feed"
	"log"
	"time"
)

// CleanCompare compares available bike data between consucetive datasets.
func CleanCompare(old map[string]feed.Bike, new []feed.Bike) map[string]feed.Bike {
	newMap := make(map[string]feed.Bike)
	for _, bike := range new {
		_, exists := old[bike.BikeID]
		if !exists {
			log.Printf("Check_in %s", bike.BikeID)
			newEvent := Event{
				Bike:      bike,
				EventType: "check_in",
				Timestamp:      time.Now(),
			}
			log.Print(newEvent)
		}
		newMap[bike.BikeID] = bike
		delete(old, bike.BikeID)
	}

	for _, oldBike := range old {
		newEvent := Event{
			Bike:      oldBike,
			EventType: "check_out",
			Timestamp:      time.Now(),
		}
		log.Print(newEvent)
	}
	return newMap
}
