package process

import (
	"deelfietsdashboard-importer/feed"
)

type ProcessResult struct {
	currentBikesInFeed map[string]feed.Bike
	createdEvents []Event
}

func ProcessNewData(strategy string, old map[string]feed.Bike, new []feed.Bike) map[string]feed.Bike {
	switch strategy {
	case "clean":
		return CleanCompare(old, new)
	case "gps":
		return nil
	}
	return nil
}
