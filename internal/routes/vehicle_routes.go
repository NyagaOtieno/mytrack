package routes

import (
	"encoding/json"
	"net/http"
	"strconv"

	"fmb920-server/internal/storage"
)

// Vehicle represents a vehicle record
type Vehicle struct {
	ID            int64  `json:"id"`
	PlateNumber   string `json:"plate_number"`
	Model         string `json:"model"`
	Make          string `json:"make"`
	Year          int    `json:"year"`
	OwnerName     string `json:"owner_name"`
	OwnerID       string `json:"owner_id_number"`
	OwnerPhone    string `json:"owner_phone"`
	OwnerEmail    string `json:"owner_email"`
}

// Device represents a device record
type Device struct {
	ID            int64   `json:"id"`
	IMEI          string  `json:"imei"`
	SIMNumber     string  `json:"sim_number,omitempty"`
	VehicleNumber string  `json:"vehicle_number,omitempty"`
	ChassisNumber string  `json:"chassis_number,omitempty"`
	LastLat       float64 `json:"last_lat,omitempty"`
	LastLng       float64 `json:"last_lng,omitempty"`
}

// --- Helper functions for JSON responses ---
func respondJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, message string, status int) {
	respondJSON(w, map[string]string{"error": message}, status)
}

// --- Vehicle Handlers ---

// POST /api/vehicles/create
func CreateVehicleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var v Vehicle
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	id, err := storage.CreateVehicle(v)
	if err != nil {
		respondError(w, "Failed to create vehicle: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]int64{"id": id}, http.StatusCreated)
}

// GET /api/vehicles/list
func ListVehiclesHandler(w http.ResponseWriter, r *http.Request) {
	vehicles, err := storage.ListVehicles()
	if err != nil {
		respondError(w, "Failed to fetch vehicles: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, vehicles, http.StatusOK)
}

// GET /api/vehicles/get?id=123
func GetVehicleHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		respondError(w, "id is required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, "Invalid id format", http.StatusBadRequest)
		return
	}

	v, err := storage.GetVehicle(id)
	if err != nil {
		respondError(w, "Vehicle not found", http.StatusNotFound)
		return
	}

	respondJSON(w, v, http.StatusOK)
}

// PUT /api/vehicles/update?id=123
func UpdateVehicleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		respondError(w, "id is required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, "Invalid id format", http.StatusBadRequest)
		return
	}

	var v Vehicle
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := storage.UpdateVehicle(id, v); err != nil {
		respondError(w, "Failed to update vehicle: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]bool{"success": true}, http.StatusOK)
}

// DELETE /api/vehicles/delete?id=123
func DeleteVehicleHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		respondError(w, "id is required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, "Invalid id format", http.StatusBadRequest)
		return
	}

	if err := storage.DeleteVehicle(id); err != nil {
		respondError(w, "Failed to delete vehicle: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]bool{"success": true}, http.StatusOK)
}

// --- Device Handlers ---

// POST /api/devices/create
func CreateDeviceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var d Device
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	id, err := storage.CreateDevice(d.IMEI, d.SIMNumber, d.VehicleNumber, d.ChassisNumber)
	if err != nil {
		respondError(w, "Failed to create device: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]int64{"id": id}, http.StatusCreated)
}

// GET /api/devices/list
func DevicesListHandler(w http.ResponseWriter, r *http.Request) {
	devices, err := storage.GetAllDevicesWithLastPosition()
	if err != nil {
		respondError(w, "Failed to fetch devices: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, devices, http.StatusOK)
}

// GET /api/devices/latest?imei=...
func LatestPositionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	imei := r.URL.Query().Get("imei")
	if imei == "" {
		respondError(w, "imei query parameter required", http.StatusBadRequest)
		return
	}

	pos, err := storage.GetLatestPositionByIMEI(imei)
	if err != nil {
		respondError(w, "Position not found: "+err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, pos, http.StatusOK)
}

// PUT /api/devices/assign?imei=123456789012345&vehicle_id=10
func AssignVehicleToDeviceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	imei := r.URL.Query().Get("imei")
	vehicleIDStr := r.URL.Query().Get("vehicle_id")

	if imei == "" || vehicleIDStr == "" {
		respondError(w, "imei and vehicle_id are required", http.StatusBadRequest)
		return
	}

	vehicleID, err := strconv.ParseInt(vehicleIDStr, 10, 64)
	if err != nil {
		respondError(w, "Invalid vehicle_id format", http.StatusBadRequest)
		return
	}

	if err := storage.AssignVehicleToDevice(imei, vehicleID); err != nil {
		respondError(w, "Failed to assign vehicle: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]bool{"success": true}, http.StatusOK)
}
