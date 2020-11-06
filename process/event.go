package process

import (
	"context"
	"deelfietsdashboard-importer/feed"
	"deelfietsdashboard-importer/geoutil"
	"fmt"
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
	Remark             string
}

func (event Event) getKey() string {
	return event.Bike.BikeID + ":" + event.Bike.BikeID
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
	if event.EventType == "" {
		return
	}

	bEvent, err := msgpack.Marshal(&event)
	_, err = processor.rdb.LPush(event.getKey(), bEvent).Result()
	if err != nil {
		log.Print(err)
	}
	// clean data, this must be improved for temporary keys
	_, err = processor.rdb.LTrim(event.getKey(), 0, 99).Result()
	if err != nil {
		log.Print(err)
	}
}

// CheckIn
func (processor DataProcessor) checkIn(event Event) Event {
	previousEvents := processor.getLastEvents(event.getKey())

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
		log.Printf("For now handle thas as a movement, only as the vehicle is moved this event is registered.")
		event.EventType = "check_in_after_reboot"
		event.Remark = "new check_in after reboot"
		return processor.vehicleMoved(event)
	}
	if checkIfTripShouldBeResetted(event, lastEvent) == true {
		log.Print("This trip should be resetted. ", event.Bike.BikeID)
		return processor.resetTrip(event, previousEvents)
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
	previousEvents := processor.getLastEvents(event.getKey())
	if len(previousEvents) == 0 {
		log.Print("There is something seriously wrong, a checkOut is always preceded at least one checkIn, possibly there is some data damaged", event)
		return event
	}

	event.RelatedParkEventID = previousEvents[0].RelatedParkEventID
	processor.EndParkEvent(event)
	event = processor.StartTrip(event)

	return event
}

func (processor DataProcessor) vehicleMoved(event Event) Event {
	previousEvents := processor.getLastEvents(event.getKey())
	if len(previousEvents) == 0 {
		log.Print("There is something seriously wrong, a moved is always preceded by another event.", event)
		return event
	}
	previousEvent := previousEvents[0]

	distanceMoved := geoutil.Distance(event.Bike.Lat, event.Bike.Lon, previousEvent.Bike.Lat, previousEvent.Bike.Lon)
	log.Print("Distance moved: ", distanceMoved)
	if distanceMoved > 500 {
		log.Print("End old park_event")
		previousEvent.Timestamp = event.Timestamp
		processor.EndParkEvent(previousEvent)
		log.Print("Create new park_event.")
		event = processor.StartParkEvent(event)
	} else if distanceMoved < 500 && distanceMoved > 0.1 {
		log.Print("Update existing park_event.")
		event = processor.UpdateLocationParkEvent(event, previousEvent)
	} else {
		log.Print("Do nothing, distance < 0.1m")
		return Event{}
	}
	event.Remark = fmt.Sprintf("Movement: %.2f", distanceMoved)

	return event
}

// This function gets the last registered events from the database.
func (processor DataProcessor) getLastEvents(bikeID string) []Event {
	results, err := processor.rdb.LRange(bikeID, 0, 4).Result()
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

func (processor DataProcessor) resetTrip(event Event, previousEvents []Event) Event {
	previousEvent := previousEvents[0]

	processor.ResetEndParkEvent(previousEvent)
	processor.CancelTrip(previousEvent)
	event.EventType = "cancel"
	event.RelatedParkEventID = previousEvent.RelatedParkEventID
	return event
}
