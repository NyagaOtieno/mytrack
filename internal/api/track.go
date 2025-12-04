package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"fmb920-server/internal/storage"
)

// PositionPayload represents incoming JSON payload for tracking
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

// TrackHandler handles POST /api/track
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
	for i, p := range payload {
		if p.DeviceID == 0 && p.IMEI == "" {
			log.Printf("Skipping position at index %d: missing device_id and imei\n", i)
			continue
		}

		// Ensure device exists
		device, err := GetDeviceByIDOrIMEI(p.DeviceID, p.IMEI)
		if err != nil || device == nil {
			log.Printf("Skipping position at index %d: device not found (device_id=%d imei=%s)\n", i, p.DeviceID, p.IMEI)
			continue
		}

		// Use the correct device ID
		p.DeviceID = device.ID

		lat := p.Latitude
		lng := p.Longitude

		// Fallback to last known coordinates if missing
		if lat == 0 || lng == 0 {
			if device.LastLat != nil && device.LastLng != nil {
				lat = *device.LastLat
				lng = *device.LastLng
				log.Printf("Using fallback coordinates for device %d: lat=%f, lng=%f\n", device.ID, lat, lng)
			} else {
				log.Printf("Skipping position at index %d: no last coordinates available\n", i)
				continue
			}
		}

		positions = append(positions, storage.Position{
			DeviceID:   p.DeviceID,
			IMEI:       p.IMEI,
			Timestamp:  parseTime(p.Timestamp),
			Latitude:   lat,
			Longitude:  lng,
			Speed:      float64(p.Speed),
			Angle:      float64(p.Angle),
			Altitude:   float64(p.Altitude),
			Satellites: p.Satellites,
		})
	}

	if len(positions) == 0 {
		http.Error(w, "No valid positions to save", http.StatusBadRequest)
		return
	}

	if err := storage.SavePositions(positions); err != nil {
		log.Println("Failed to save positions:", err)
		http.Error(w, "Failed to save positions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"success": true, "saved_count": len(positions)})
}

// parseTime parses timestamp string to time.Time, fallback to now if invalid
func parseTime(ts string) time.Time {
	if ts == "" {
		return time.Now()
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		log.Println("Invalid timestamp, using now:", ts, err)
		return time.Now()
	}
	return t
}

// GetDeviceByIDOrIMEI returns device info including last coordinates
func GetDeviceByIDOrIMEI(deviceID int64, imei string) (*storage.Device, error) {
	ctx := context.Background()
	var d storage.Device
	err := storage.Pool.QueryRow(ctx,
		`SELECT id, last_lat, last_lng FROM devices WHERE id=$1 OR imei=$2`,
		deviceID, imei,
	).Scan(&d.ID, &d.LastLat, &d.LastLng)
	if err != nil {
		return nil, err
	}
	return &d, nil
}
