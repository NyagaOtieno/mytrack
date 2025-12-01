package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"fmb920-server/internal/repository"
)

// AssignVehicle assigns a vehicle to a device
func AssignVehicle(w http.ResponseWriter, r *http.Request) {
	imei := chi.URLParam(r, "imei")

	var body struct {
		VehicleID int `json:"vehicle_id"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	err := repository.AssignVehicleToDevice(imei, body.VehicleID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Write([]byte(`{"success":true}`))
}

// LatestPositions returns the latest positions of all devices
func LatestPositions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Fetch devices with last known position from repository
	devices, err := repository.GetAllDevicesWithLastPosition()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build response: only IMEI + last coordinates
	type Latest struct {
		IMEI      string  `json:"imei"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}

	var latest []Latest
	for _, d := range devices {
		latest = append(latest, Latest{
			IMEI:      d.IMEI,
			Latitude:  d.LastLat,
			Longitude: d.LastLng,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(latest)
}
