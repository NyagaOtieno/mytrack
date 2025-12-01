package repository

import (
	"fmb920-server/internal/db"
	"fmb920-server/internal/models"
)

func CreateVehicle(v models.Vehicle) (int, error) {
	var id int
	query := `
		INSERT INTO vehicles (
			plate_number, model, make, year,
			owner_name, owner_id_number, owner_phone, owner_email
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id;
	`

	err := db.DB.QueryRow(query,
		v.PlateNumber, v.Model, v.Make, v.Year,
		v.OwnerName, v.OwnerID, v.OwnerPhone, v.OwnerEmail,
	).Scan(&id)

	return id, err
}

func GetVehicle(id int) (models.Vehicle, error) {
	var v models.Vehicle

	query := `
		SELECT id, plate_number, model, make, year,
		       owner_name, owner_id_number, owner_phone, owner_email
		FROM vehicles WHERE id=$1;
	`

	err := db.DB.QueryRow(query, id).Scan(
		&v.ID, &v.PlateNumber, &v.Model, &v.Make, &v.Year,
		&v.OwnerName, &v.OwnerID, &v.OwnerPhone, &v.OwnerEmail,
	)
	return v, err
}

func ListVehicles() ([]models.Vehicle, error) {
	rows, err := db.DB.Query(`
		SELECT id, plate_number, model, make, year,
		       owner_name, owner_id_number, owner_phone, owner_email
		FROM vehicles ORDER BY id DESC;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Vehicle

	for rows.Next() {
		var v models.Vehicle
		rows.Scan(
			&v.ID, &v.PlateNumber, &v.Model, &v.Make, &v.Year,
			&v.OwnerName, &v.OwnerID, &v.OwnerPhone, &v.OwnerEmail,
		)
		list = append(list, v)
	}

	return list, nil
}

func UpdateVehicle(id int, v models.Vehicle) error {
	query := `
		UPDATE vehicles SET 
			plate_number=$1, model=$2, make=$3, year=$4,
			owner_name=$5, owner_id_number=$6, owner_phone=$7, owner_email=$8
		WHERE id=$9;
	`

	_, err := db.DB.Exec(query,
		v.PlateNumber, v.Model, v.Make, v.Year,
		v.OwnerName, v.OwnerID, v.OwnerPhone, v.OwnerEmail,
		id,
	)
	return err
}

func DeleteVehicle(id int) error {
	_, err := db.DB.Exec(`DELETE FROM vehicles WHERE id=$1`, id)
	return err
}
