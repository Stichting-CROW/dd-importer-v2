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
	return event.Bike.SystemID + ":" + event.Bike.BikeID
}

var ctx = context.Background()

func (processor DataProcessor) EventProcessor() {
	counter := 0
	for {
		events := <-processor.EventChan
		if len(processor.EventChan) > 5 {
			log.Printf("%d events in queue", len(processor.EventChan))
			log.Println("EventProcessor werkt te langzaam, vandaar deze melding.")
		}
		for _, event := range events {
			processor.ProcessEvent(event)
			counter += 1
			if counter%1000 == 0 {
				log.Printf("[EventProcessor] Processed %d messages.", counter)
			}
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
	case "correcting_check_out":
		event = processor.correctingCheckOut(event)
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
		event = processor.firstCheckIn(event)
		return event
	}
	event = processor.checkIfTripIsMade(event, previousEvents)
	return event
}

// This function should be called when a vehicle is checked in for the first time.
// This function tries to reuse a existing park_event.
func (processor DataProcessor) firstCheckIn(event Event) Event {
	newEvent := processor.GetLastParkEvent(event)
	// No existing park_event exits.
	if newEvent.RelatedParkEventID == 0 {
		newEvent = processor.StartParkEvent(event)
	} else {
	}
	return newEvent
}

func (processor DataProcessor) checkIfTripIsMade(event Event, previousEvents []Event) Event {
	lastEvent := previousEvents[0]
	if lastEvent.EventType != "check_out" {
		event.EventType = "check_in_after_reboot"
		event.Remark = "new check_in after reboot"
		return processor.vehicleMoved(event)
	}
	if checkIfTripShouldBeResetted(event, lastEvent) == true {
		return processor.resetTrip(event, previousEvents)
	}

	event.RelatedTripID = lastEvent.RelatedTripID
	event = processor.EndTrip(event)
	event = processor.StartParkEvent(event)
	return event
}

func checkIfTripShouldBeResetted(checkIn Event, previousCheckOut Event) bool {
	durationShorterThanThreshold := checkIn.Timestamp.Sub(previousCheckOut.Timestamp) < time.Minute*15
	distanceShorterThenThreshold := geoutil.Distance(checkIn.Bike.Lat, checkIn.Bike.Lon,
		previousCheckOut.Bike.Lat, previousCheckOut.Bike.Lon) < 100
	return durationShorterThanThreshold && distanceShorterThenThreshold
}

func (processor DataProcessor) checkOut(event Event) Event {
	previousEvents := processor.getLastEvents(event.getKey())
	if len(previousEvents) == 0 {
		log.Print("There is something seriously wrong, a checkOut is always preceded at least one checkIn, possibly there is some data damaged.", event)
		return event
	}

	event.RelatedParkEventID = previousEvents[0].RelatedParkEventID
	processor.EndParkEvent(event)
	event = processor.StartTrip(event)

	return event
}

func (processor DataProcessor) correctingCheckOut(event Event) Event {
	log.Print("End a park event that was missed before this program was started.")
	processor.EndParkEvent(event)
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
	if distanceMoved > 500 {
		//log.Print("End old park_event")
		previousEvent.Timestamp = event.Timestamp
		processor.EndParkEvent(previousEvent)
		//log.Print("Create new park_event.")
		event = processor.StartParkEvent(event)
	} else if distanceMoved < 500 && distanceMoved > 0.1 {
		//log.Print("Update existing park_event.")
		event = processor.UpdateLocationParkEvent(event, previousEvent)
	} else {
		//log.Print("Do nothing, distance < 0.1m")
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
