package models

type Vehicle struct {
	ID            int    `json:"id"`
	PlateNumber   string `json:"plate_number"`
	Model         string `json:"model"`
	Make          string `json:"make"`
	Year          int    `json:"year"`

	OwnerName     string `json:"owner_name"`
	OwnerID       string `json:"owner_id_number"`
	OwnerPhone    string `json:"owner_phone"`
	OwnerEmail    string `json:"owner_email"`
}
