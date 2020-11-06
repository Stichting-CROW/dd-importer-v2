package process

// StartTrip create a record in the trips table what is open.
func (processor DataProcessor) StartTrip(checkOut Event) Event {
	stmt := `INSERT INTO trips
		(system_id, bike_id, start_location, start_time)
		VALUES ($1, $2, ST_SetSRID(ST_Point($3, $4), 4326), $5)
		RETURNING trip_id
	`
	row := processor.DB.QueryRowx(stmt, checkOut.Bike.SystemID, checkOut.Bike.BikeID,
		checkOut.Bike.Lon, checkOut.Bike.Lat, checkOut.Timestamp)
	row.Scan(&checkOut.RelatedTripID)
	return checkOut
}

// EndTrip updates a record in the trip table what was opened before.
func (processor DataProcessor) EndTrip(checkIn Event) Event {
	stmt := `UPDATE trips
		SET end_location = ST_SetSRID(ST_Point($1, $2), 4326), end_time = $3
		WHERE trip_id = $4
	`
	processor.DB.MustExec(stmt, checkIn.Bike.Lon, checkIn.Bike.Lat,
		checkIn.Timestamp, checkIn.RelatedTripID)
	return checkIn
}

// CancelTrip should be calle when a trip was started but not completed.
func (processor DataProcessor) CancelTrip(checkIn Event) {
	stmt := `DELETE 
		FROM trips
		WHERE trip_id = $1`

	processor.DB.MustExec(stmt, checkIn.RelatedTripID)
}

// UpdateEndLocationTrip can be updated the end location of a trip.
func (processor DataProcessor) UpdateEndLocationTrip(newEvent Event, eventToUpdate Event) {
	stmt := `UPDATE trips
		SET location ST_SetSRID(ST_POINT($1, $2))
		WHERE trip_id = $3`

	processor.DB.MustExec(stmt, newEvent.Bike.Lon, newEvent.Bike.Lat, eventToUpdate.Bike.Lon, eventToUpdate.RelatedTripID)
}
