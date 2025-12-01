package repository

import (
	"fmb920-server/internal/db"
)

func AssignVehicleToDevice(imei string, vehicleID int) error {
	_, err := db.DB.Exec(`
		UPDATE devices SET vehicle_id=$1 WHERE imei=$2;
	`, vehicleID, imei)

	return err
}
