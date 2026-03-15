package process

import (
	"log"
)

// nonOperationalChange creates a new event when a vehicle changes from operational to non-operational or the other way around.
func (processor DataProcessor) nonOperationalChange(event Event) Event {
	lastEvents := processor.getLastEvents(event.getKey())
	if len(lastEvents) == 0 {
		log.Printf("Can't find a related park event for bike %s, %s, so the non-operational change is ignored.", event.getKey(), event.Bike.BikeID)
		return event
	}
	event.RelatedParkEventID = lastEvents[0].RelatedParkEventID

	if event.Bike.IsDisabled {
		log.Printf("Vehicle %s is now non-operational", event.Bike.BikeID)
		processor.StartNonOperationalEvent(event)
	} else {
		log.Printf("Vehicle %s is now operational", event.Bike.BikeID)
		processor.EndNonOperationalEvent(event)
	}
	return event
}

// StartNonOperationalEvent started a new non-operational event in the database when a vehicle becomes non-operational.
func (processor DataProcessor) StartNonOperationalEvent(checkIn Event) Event {
	log.Printf("Start non-operational event for %s, related park event id: %d, event type: %s", checkIn.getKey(), checkIn.RelatedParkEventID, checkIn.EventType)
	stmt := `INSERT INTO non_operational_event
		(park_event_id, start_time)
		VALUES ($1, $2);
	`
	processor.DB.MustExec(stmt, checkIn.RelatedParkEventID, checkIn.Timestamp)
	return checkIn
}

func (processor DataProcessor) EndNonOperationalEvent(event Event) {
	// fail safe
	log.Printf("End non-operational event for %s, related park event id: %d, event type: %s", event.getKey(), event.RelatedParkEventID, event.EventType)
	if event.RelatedParkEventID == 0 {
		log.Printf("Can't close non-operational event %s, %s", event.getKey(), event.Bike.BikeID)
		return
	}

	stmt := `UPDATE non_operational_event
	SET end_time = $1
	WHERE park_event_id = $2
	AND end_time is null`
	processor.DB.MustExec(stmt, event.Timestamp, event.RelatedParkEventID)
}

func (processor DataProcessor) CancelEndNonOperationalEvent(event Event) {
	log.Printf("Reopen existing non_operational_event %d %s", event.RelatedParkEventID, event.getKey())
	stmt := `WITH newest AS (
		SELECT non_operational_event_id
		FROM non_operational_event
		WHERE park_event_id = $1
		ORDER BY non_operational_event_id DESC
		LIMIT 1
	)
	UPDATE non_operational_event noe
	SET end_time = null
	FROM newest
	WHERE noe.non_operational_event_id = newest.non_operational_event_id;`
	processor.DB.MustExec(stmt, event.RelatedParkEventID)
}
