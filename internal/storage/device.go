package storage

import (
	"context"
	"fmt"
	
)

// Vehicle represents a vehicle object in the database
type Vehicle struct {
	ID            int64
	IMEI          string
	VehicleNumber string
	ChassisNumber string
	CreatedByID   int64
}

// CreateVehicle creates a new vehicle in the database
func CreateVehicle(vehicle Vehicle) (int64, error) {
	// Define a context for the query
	ctx := context.Background()

	var id int64
	// Insert the vehicle data into the database and retrieve the generated ID
	err := Pool.QueryRow(
		ctx,
		`INSERT INTO vehicles (imei, vehicle_number, chassis_number, created_by_admin_id) 
		VALUES ($1, $2, $3, $4) RETURNING id`,
		vehicle.IMEI, vehicle.VehicleNumber, vehicle.ChassisNumber, vehicle.CreatedByID,
	).Scan(&id)

	if err != nil {
		// Return a wrapped error if insertion fails
		return 0, fmt.Errorf("failed to create vehicle: %v", err)
	}

	// Return the generated ID and no error
	return id, nil
}
