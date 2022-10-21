package gbfs

import (
	"deelfietsdashboard-importer/process"
	"log"
)

func getVehicleTypesFromDB(processor process.DataProcessor, systemId string) []VehicleType {
	stmt := `SELECT vehicle_type_id, external_vehicle_type_id, form_factor,
		propulsion_type, system_id, name
		FROM vehicle_type
		WHERE system_id = $1
	`
	rows, err := processor.DB.Queryx(stmt, systemId)
	if err != nil {
		log.Print(err)
	}

	vehicleTypes := []VehicleType{}
	for rows.Next() {
		vehicleType := VehicleType{}
		if err := rows.Scan(&vehicleType.VehicleTypeId,
			&vehicleType.ExternalVehicleTypeId,
			&vehicleType.FormFactor,
			&vehicleType.PropulsionType,
			&vehicleType.SystemId,
			&vehicleType.Name); err != nil {
			log.Print(err)
		}
		vehicleTypes = append(vehicleTypes, vehicleType)
	}
	return vehicleTypes
}

func insertVehicleType(vehicleType VehicleType, processor process.DataProcessor) VehicleType {
	stmt := `INSERT INTO vehicle_type
		(external_vehicle_type_id, form_factor,
			propulsion_type, system_id, name)
		VALUES ($1, $2, $3, $4, $5)
		returning vehicle_type_id
	`
	row := processor.DB.QueryRowx(stmt, vehicleType.ExternalVehicleTypeId,
		vehicleType.FormFactor, vehicleType.PropulsionType, vehicleType.SystemId, vehicleType.Name)
	row.Scan(&vehicleType.VehicleTypeId)
	return vehicleType
}
