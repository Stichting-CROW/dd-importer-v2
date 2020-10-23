package process

import (
	"context"
	"deelfietsdashboard-importer/feed"
	"github.com/vmihailenco/msgpack/v5"
	"log"
	"time"
)

type Event struct {
	OperatorID		   string
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
		processor.CheckIn(event)
	case "check_out":
		processor.CheckOut(event)
	}

	results, err := processor.rdb.LRange(event.Bike.BikeID, 0, -1).Result()
	if len(results) == 0 {
		processor.CheckIn(event)
	}
	for _, result := range results {
		var testEvent Event
		msgpack.Unmarshal([]byte(result), &testEvent)

		// distance := Distance(testEvent.Bike.Lat, testEvent.Bike.Lon, event.Bike.Lat, event.Bike.Lon)
		// if distance > 0.1 {
		// 	log.Printf("Movement of %v %f", event, distance)
		// }
	}

	if err != nil {
		log.Print(err)
	}
}

// CheckIn
func (processor DataProcessor) CheckIn(event Event) {
	bEvent, err := msgpack.Marshal(&event)
	if err != nil {
		panic(err)
	}
	_, err = processor.rdb.LPush(event.Bike.BikeID, bEvent).Result()
	log.Print(err)
	processor.StartParkEvent(event)

	// Create new park Event

}




func (processor DataProcessor) CheckOut(event Event) {

}
