package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"fmb920-server/internal/models"
	"fmb920-server/internal/repository"
)

func CreateVehicle(w http.ResponseWriter, r *http.Request) {
	var v models.Vehicle
	json.NewDecoder(r.Body).Decode(&v)

	id, err := repository.CreateVehicle(v)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{"id": id})
}

func GetVehicle(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	v, err := repository.GetVehicle(id)
	if err != nil {
		http.Error(w, "Vehicle not found", 404)
		return
	}

	json.NewEncoder(w).Encode(v)
}

func ListVehicles(w http.ResponseWriter, r *http.Request) {
	list, err := repository.ListVehicles()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	json.NewEncoder(w).Encode(list)
}

func UpdateVehicle(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var v models.Vehicle
	json.NewDecoder(r.Body).Decode(&v)

	err := repository.UpdateVehicle(id, v)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Write([]byte(`{"success":true}`))
}

func DeleteVehicle(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	err := repository.DeleteVehicle(id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Write([]byte(`{"success":true}`))
}
