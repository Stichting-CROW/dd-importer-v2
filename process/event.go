package process

import (
	"context"
	"deelfietsdashboard-importer/feed"
	"deelfietsdashboard-importer/geoutil"
	"log"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

type Event struct {
	OperatorID         string
	Bike               feed.Bike
	EventType          string
	Timestamp          time.Time
	RelatedTripID      int64
	RelatedParkEventID int64
}

var ctx = context.Background()

func (processor DataProcessor) EventProcessor() {
	for {
		events := <-processor.eventChan
		log.Print(events)
		for _, event := range events {
			processor.ProcessEvent(event)
		}
	}
}

func (processor DataProcessor) ProcessEvent(event Event) {
	switch event.EventType {
	case "check_in":
		event = processor.checkIn(event)
	case "check_out":
		event = processor.checkOut(event)
	case "vehicle_moved":
		event = processor.vehicleMoved(event)
	}
	bEvent, err := msgpack.Marshal(&event)
	_, err = processor.rdb.LPush(event.Bike.BikeID, bEvent).Result()
	if err != nil {
		log.Print(err)
	}
}

// CheckIn
func (processor DataProcessor) checkIn(event Event) Event {
	previousEvents := processor.getLastEvents(event.Bike.BikeID)

	if len(previousEvents) == 0 {
		event = processor.StartParkEvent(event)
		return event
	}
	event = processor.checkIfTripIsMade(event, previousEvents)

	return event
}

func (processor DataProcessor) checkIfTripIsMade(event Event, previousEvents []Event) Event {
	lastEvent := previousEvents[0]
	if lastEvent.EventType != "check_out" {
		log.Printf("Last Event was not a check_out that is strange behaviour.... see details %v, there is no trip made.", event)
		return event
	}
	if checkIfTripShouldBeResetted(event, lastEvent) == true {
		log.Print("This trip should be resetted. (still has to be implemented)")
		log.Print(event)
		log.Print(lastEvent)
	}

	event.RelatedTripID = lastEvent.RelatedTripID
	log.Printf("End tripEvent %v", event)
	log.Print("Previous event %v", lastEvent)
	event = processor.EndTrip(event)
	event = processor.StartParkEvent(event)
	return event
}

func checkIfTripShouldBeResetted(checkIn Event, previousCheckOut Event) bool {
	durationShorterThanThreshold := checkIn.Timestamp.Sub(previousCheckOut.Timestamp) < time.Minute*15
	distanceShorterThenThreshold := geoutil.Distance(checkIn.Bike.Lon, checkIn.Bike.Lat,
		previousCheckOut.Bike.Lon, previousCheckOut.Bike.Lon) < 100
	return durationShorterThanThreshold && distanceShorterThenThreshold
}

func (processor DataProcessor) checkOut(event Event) Event {
	previousEvents := processor.getLastEvents(event.Bike.BikeID)
	if len(previousEvents) == 0 {
		log.Print("There is something seriously wrong, a checkOut is always preceded at least one checkIn, possibly there is some data damaged", event)
		return event
	}

	event.RelatedParkEventID = previousEvents[0].RelatedParkEventID
	event = processor.EndParkEvent(event)
	event = processor.StartTrip(event)

	return event
}

func (processor DataProcessor) vehicleMoved(event Event) Event {
	return event
}

// This function gets the last registered events from the database.
func (processor DataProcessor) getLastEvents(bikeID string) []Event {
	results, err := processor.rdb.LRange(bikeID, 0, -1).Result()
	if err != nil {
		log.Printf("Error in receiving latest events of bike_id %v", err)
	}
	events := []Event{}
	for _, result := range results {
		var event Event
		msgpack.Unmarshal([]byte(result), &event)
		events = append(events, event)
	}
	return events

}
