package process

import (
	"deelfietsdashboard-importer/feed"
	"log"
	"time"
)

// CleanCompare compares available bike data between consucetive datasets.
func CleanCompare(old map[string]feed.Bike, new []feed.Bike) ProcessResult {
	processResult := ProcessResult{
		CurrentBikesInFeed: map[string]feed.Bike{},
		createdEvents:      []Event{},
	}
	if len(new) == 0 {
		log.Print("Suddenly all bikes are gone, possibly something is wrong with the feed, so the new data is ignored.")
		processResult.feedIsEmpty = true
		processResult.CurrentBikesInFeed = old
		return processResult
	}

	for _, bike := range new {
		_, exists := old[bike.BikeID]
		if !exists {
			log.Printf("Check_in %s", bike.BikeID)
			newEvent := Event{
				Bike:      bike,
				EventType: "check_in",
				Timestamp: time.Now(),
			}
			processResult.createdEvents = append(processResult.createdEvents, newEvent)
		}
		processResult.CurrentBikesInFeed[bike.BikeID] = bike
		delete(old, bike.BikeID)
	}

	for _, oldBike := range old {
		newEvent := Event{
			Bike:      oldBike,
			EventType: "check_out",
			Timestamp: time.Now(),
		}
		processResult.createdEvents = append(processResult.createdEvents, newEvent)
	}
	return processResult
}
