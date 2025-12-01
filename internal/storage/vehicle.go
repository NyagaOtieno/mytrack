package storage

import (
	"context"
	"fmt"
)

// CheckVehicleAccess checks if an admin has access to a specific vehicle
func CheckVehicleAccess(adminID, vehicleID int) (bool, error) {
	var exists bool
	err := Pool.QueryRow(
		context.Background(),
		`SELECT EXISTS (SELECT 1 FROM api_keys WHERE admin_id = $1 AND vehicle_id = $2)`,
		adminID, vehicleID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check access: %v", err)
	}
	return exists, nil
}
