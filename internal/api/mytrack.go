package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

// MyTrackHandler fetches devices from MyTrack and returns only IMEI + coordinates
func MyTrackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	apiKey := os.Getenv("MYTRACK_API_KEY")
	if apiKey == "" {
		http.Error(w, "API key not set", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://mytrack-production.up.railway.app/api/devices/list", nil)
	if err != nil {
		http.Error(w, "Failed to create request: "+err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("❌ Error fetching MyTrack:", err)
		http.Error(w, "Failed to fetch MyTrack data", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "MyTrack API returned status "+resp.Status, http.StatusBadGateway)
		return
	}

	var devices []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		http.Error(w, "Failed to parse MyTrack response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var output []map[string]interface{}
	for _, d := range devices {
		out := map[string]interface{}{"IMEI": d["IMEI"]}
		if lat, ok := d["LastLat"].(float64); ok {
			out["Latitude"] = lat
		}
		if lng, ok := d["LastLng"].(float64); ok {
			out["Longitude"] = lng
		}
		output = append(output, out)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(output)
}
