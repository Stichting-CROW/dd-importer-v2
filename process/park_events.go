package process

// StartParkEvent started a new park_event in the database when a bike is parked.
func (processor DataProcessor) StartParkEvent(checkIn Event) Event {
	stmt := `INSERT INTO park_events
		(system_id, bike_id, location, start_time)
		VALUES ($1, $2, ST_SetSRID(ST_Point($3, $4), 4326), $5)
		RETURNING park_event_id
	`
	row := processor.DB.QueryRowx(stmt, checkIn.Bike.SystemID, checkIn.getKey(), checkIn.Bike.Lon, checkIn.Bike.Lat, checkIn.Timestamp)
	row.Scan(&checkIn.RelatedParkEventID)
	return checkIn
}

// EndParkEvent ends a park_event in the database when a bike is removed.
func (processor DataProcessor) EndParkEvent(checkOut Event) {
	stmt := `UPDATE park_events
		SET end_time = $1
		WHERE park_event_id = $2
	`
	processor.DB.MustExec(stmt, checkOut.Timestamp, checkOut.RelatedParkEventID)
}

// UpdateLocationParkEvent updates the location of a park_event.
func (processor DataProcessor) UpdateLocationParkEvent(newEvent Event, eventToUpdate Event) Event {
	stmt := `UPDATE park_events
	SET location = ST_SetSRID(ST_POINT($1, $2), 4326)
	WHERE park_event_id = $3`

	newEvent.RelatedParkEventID = eventToUpdate.RelatedParkEventID
	processor.DB.MustExec(stmt, newEvent.Bike.Lon, newEvent.Bike.Lat, eventToUpdate.RelatedParkEventID)
	return newEvent
}

func (processor DataProcessor) ResetEndParkEvent(event Event) {
	// EndParkEvent ends a park_event in the database when a bike is removed.
	stmt := `UPDATE park_events
		SET end_time = null
		WHERE park_event_id = $1
	`
	processor.DB.MustExec(stmt, event.RelatedParkEventID)
}
