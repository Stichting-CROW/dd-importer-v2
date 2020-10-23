package process

func (processor DataProcessor) StartParkEvent(checkIn Event) {
	stmt := `INSERT INTO park_events
		(system_id, bike_id, location, start_time)
		VALUES ($1, $2, ST_SetSRID(ST_Point($3, $4), 4326), $5)
	`
	processor.db.MustExec(stmt, "test", checkIn.Bike.BikeID, checkIn.Bike.Lon, checkIn.Bike.Lat, checkIn.Timestamp)
}

func (processor DataProcessor) EndParkEvent(checkOut Event) {
	stmt := `UPDATE park_events
		SET end_time = $1
		WHERE park_event_id = $2
	`
	processor.db.MustExec(stmt, checkOut.Timestamp, checkOut.RelatedParkEventID)
}

func (processor DataProcessor) UpdateLocationParkEvent(newEvent Event, eventToUpdate Event) {
	stmt := `UPDATE park_events
	SET location ST_SetSRID(ST_POINT($1, $2))
	WHERE park_event_id = $3`

	processor.db.MustExec(stmt, newEvent.Bike.Lon, newEvent.Bike.Lat, eventToUpdate.Bike.Lon, eventToUpdate.RelatedParkEventID)
}