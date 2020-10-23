package process

func (processor DataProcessor) StartTrip(checkOut Event) {
	stmt := `INSERT INTO trips
		(system_id, bike_id, start_location, start_time)
		VALUES ($1, $2, ST_SetSRID(ST_Point($3, $4), 4326), $5)
	`
	processor.db.MustExec(stmt, "test", checkIn.Bike.BikeID, checkIn.Bike.Lon, checkIn.Bike.Lat, checkIn.Timestamp)
}

func (processor DataProcessor) EndTrip(checkIn Event) {
	stmt := `UPDATE trips
		SET end_location = ST_SetSRID(ST_Point($1, $2), 4326), end_time = $3
		WHERE park_event_id = $4
	`
	processor.db.MustExec(stmt, checkOut.Bike.Lon, checkOut.Bike.Lat, 
		checkOut.Bike.Timestamp, checkOut.RelatedParkEventID)
}

func (processor DataProcessor) CancelTrip(checkIn Event) {
	stmt :=  `DELETE 
		FROM trips
		WHERE trip_id = $1`

	processor.db.MustExec(stmt, checkIn.relatedTripID)
}

func (processor DataProcessor) UpdateEndLocationTrip(newEvent Event, eventToUpdate Event) {
	stmt := `UPDATE trips
		SET location ST_SetSRID(ST_POINT($1, $2))
		WHERE park_event_id = $3`

	processor.db.MustExec(stmt, newEvent.Bike.Lon, newEvent.Bike.Lat, eventToUpdate.Bike.Lon, eventToUpdate.RelatedTripID)
}