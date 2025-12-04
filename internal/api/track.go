package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"fmb920-server/internal/storage"
)

type PositionPayload struct {
	DeviceID   int64   `json:"device_id"`
	IMEI       string  `json:"imei"`
	Timestamp  string  `json:"timestamp"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	Speed      int     `json:"speed"`
	Angle      int     `json:"angle"`
	Altitude   int     `json:"altitude"`
	Satellites int     `json:"satellites"`
}

func TrackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload []PositionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Println("Failed to decode JSON:", err)
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(payload) == 0 {
		http.Error(w, "Empty payload", http.StatusBadRequest)
		return
	}

	var positions []storage.Position
	for _, p := range payload {
		positions = append(positions, storage.Position{
			DeviceID:  p.DeviceID,
			IMEI:      p.IMEI,
			Timestamp: parseTime(p.Timestamp),
			Latitude:  p.Latitude,
			Longitude: p.Longitude,
			Speed:     float64(p.Speed),
			Angle:     float64(p.Angle),
			Altitude:  float64(p.Altitude),
			Satellites: p.Satellites,
		})
	}

	if err := storage.SavePositions(positions); err != nil {
		log.Println("Failed to save positions:", err)
		http.Error(w, "Failed to save positions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "saved_count": len(positions)})
}

func parseTime(ts string) time.Time {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Now()
	}
	return t
}
