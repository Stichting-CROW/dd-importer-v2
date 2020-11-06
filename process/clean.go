package process

import (
	"deelfietsdashboard-importer/feed"
	"deelfietsdashboard-importer/geoutil"
	"log"
	"time"
)

// CleanCompare compares available bike data between consucetive datasets.
func CleanCompare(old map[string]feed.Bike, new []feed.Bike) Result {
	processResult := Result{
		CurrentBikesInFeed: map[string]feed.Bike{},
		CreatedEvents:      []Event{},
	}
	if len(new) == 0 {
		log.Print("Suddenly all bikes are gone, possibly something is wrong with the feed, so the new data is ignored.")
		processResult.FeedIsEmpty = true
		processResult.CurrentBikesInFeed = old
		return processResult
	}

	for _, bike := range new {
		oldBike, exists := old[bike.BikeID]
		if !exists {
			log.Printf("Check_in %s", bike.BikeID)
			newEvent := Event{
				Bike:      bike,
				EventType: "check_in",
				Timestamp: time.Now(),
			}
			processResult.CreatedEvents = append(processResult.CreatedEvents, newEvent)
		} else if geoutil.Distance(oldBike.Lat, oldBike.Lon,
			bike.Lat, bike.Lon) > 0.1 {
			// This event is created when a bicycle is moved more then 0.1m
			log.Print("Vehicle_moved %s", bike.BikeID)
			newEvent := Event{
				Bike:      bike,
				EventType: "vehicle_moved",
				Timestamp: time.Now(),
			}
			processResult.CreatedEvents = append(processResult.CreatedEvents, newEvent)
		}

		processResult.CurrentBikesInFeed[bike.BikeID] = bike
		delete(old, bike.BikeID)
	}

	for _, oldBike := range old {
		log.Printf("Check_out %s", oldBike.BikeID)
		newEvent := Event{
			Bike:      oldBike,
			EventType: "check_out",
			Timestamp: time.Now(),
		}
		processResult.CreatedEvents = append(processResult.CreatedEvents, newEvent)
	}
	return processResult
}
