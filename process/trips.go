package process

// StartTrip create a record in the trips table what is open.
func (processor DataProcessor) StartTrip(checkOut Event) {
	stmt := `INSERT INTO trips
		(system_id, bike_id, start_location, start_time)
		VALUES ($1, $2, ST_SetSRID(ST_Point($3, $4), 4326), $5)
	`
	processor.db.MustExec(stmt, checkOut.Bike.SystemID, checkOut.Bike.BikeID,
		checkOut.Bike.Lon, checkOut.Bike.Lat, checkOut.Timestamp)
}

// EndTrip updates a record in the trip table what was opened before.
func (processor DataProcessor) EndTrip(checkIn Event) {
	stmt := `UPDATE trips
		SET end_location = ST_SetSRID(ST_Point($1, $2), 4326), end_time = $3
		WHERE park_event_id = $4
	`
	processor.db.MustExec(stmt, checkIn.Bike.Lon, checkIn.Bike.Lat,
		checkIn.Timestamp, checkIn.RelatedParkEventID)
}

// CancelTrip should be calle when a trip was started but not completed.
func (processor DataProcessor) CancelTrip(checkIn Event) {
	stmt := `DELETE 
		FROM trips
		WHERE trip_id = $1`

	processor.db.MustExec(stmt, checkIn.RelatedTripID)
}

// UpdateEndLocationTrip can be updated the end location of a trip.
func (processor DataProcessor) UpdateEndLocationTrip(newEvent Event, eventToUpdate Event) {
	stmt := `UPDATE trips
		SET location ST_SetSRID(ST_POINT($1, $2))
		WHERE park_event_id = $3`

	processor.db.MustExec(stmt, newEvent.Bike.Lon, newEvent.Bike.Lat, eventToUpdate.Bike.Lon, eventToUpdate.RelatedTripID)
}
