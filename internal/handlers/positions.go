package handlers

import (
	"encoding/json"
	"net/http"

	"fmb920-server/internal/repository"
)

func ListLatestPositions(w http.ResponseWriter, r *http.Request) {
	positions, err := repository.ListLatestPositions()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(positions)
}
