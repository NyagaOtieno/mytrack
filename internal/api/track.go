package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"fmb920-server/internal/storage"
)

// -------------------- PAYLOADS --------------------

type PositionPayload struct {
	IMEI       string  `json:"imei"`
	Timestamp  string  `json:"timestamp"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	Speed      int     `json:"speed"`
	Angle      int     `json:"angle"`
	Altitude   int     `json:"altitude"`
	Satellites int     `json:"satellites"`
}

type DevicePayload struct {
	IMEI      string `json:"imei"`
	SIM       string `json:"sim,omitempty"`
	VehicleNo string `json:"vehicle_no,omitempty"`
	ChassisNo string `json:"chassis_no,omitempty"`
}

// -------------------- TRACK HANDLER --------------------

// TrackHandler receives positions and saves them reliably, supports bulk saving
func TrackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read API key and URL from environment variables
	apiKey := os.Getenv("VITE_PUBLIC_MYTRACK")
	apiURL := os.Getenv("VITE_API_URL")
	if apiKey == "" || apiURL == "" {
		log.Println("❌ Missing API key or URL in environment variables")
		http.Error(w, "Server misconfiguration", http.StatusInternalServerError)
		return
	}

	var payload []PositionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("Failed to decode JSON: %v\n", err)
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(payload) == 0 {
		http.Error(w, "Empty payload", http.StatusBadRequest)
		return
	}

	var positions []storage.Position
	var saved int

	for _, p := range payload {
		if p.Latitude == 0 || p.Longitude == 0 {
			log.Println("⚠️ Skipping invalid lat/lng:", p)
			continue
		}

		devID, err := storage.GetDeviceIDByIMEI(p.IMEI)
		if err != nil {
			log.Println("❌ Unknown IMEI, skipping:", p.IMEI)
			continue
		}

		pos := storage.Position{
			DeviceID:   devID,
			IMEI:       p.IMEI,
			Timestamp:  parseTime(p.Timestamp),
			Latitude:   p.Latitude,
			Longitude:  p.Longitude,
			Speed:      float64(p.Speed),
			Angle:      float64(p.Angle),
			Altitude:   float64(p.Altitude),
			Satellites: p.Satellites,
		}

		positions = append(positions, pos)

		// Update device last known position
		if err := storage.UpdateDeviceLastPosition(devID, pos.Latitude, pos.Longitude); err != nil {
			log.Println("⚠️ Failed to update device last position:", err)
		}

		saved++
	}

	if len(positions) > 0 {
		if err := storage.SavePositions(positions); err != nil {
			log.Println("❌ Failed to save positions:", err)
			http.Error(w, "Failed to save positions: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	writeJSON(w, map[string]interface{}{
		"success":        true,
		"positionsSaved": saved,
		"api_url_used":   apiURL,  // optional, just for logging/debugging
	})
}

// parseTime converts timestamp string to time.Time, fallback to now
func parseTime(ts string) time.Time {
	if ts == "" {
		return time.Now()
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		log.Println("⚠️ Invalid timestamp, using now:", ts, err)
		return time.Now()
	}
	return t
}

// -------------------- DEVICE HANDLERS --------------------

func CreateDeviceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var d DevicePayload
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := storage.CreateDevice(storage.Device{
		IMEI:      d.IMEI,
		SIM:       d.SIM,
		VehicleNo: d.VehicleNo,
		ChassisNo: d.ChassisNo,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]int64{"id": id})
}

func DevicesListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	devices, err := storage.GetAllDevicesWithLastPosition()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, devices)
}

func LatestPositionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	devices, err := storage.GetAllDevicesWithLastPosition()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type Latest struct {
		IMEI      string  `json:"imei"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}

	latest := make([]Latest, 0, len(devices))
	for _, d := range devices {
		lat, lng := 0.0, 0.0
		if d.LastLat != nil {
			lat = *d.LastLat
		}
		if d.LastLng != nil {
			lng = *d.LastLng
		}

		latest = append(latest, Latest{
			IMEI:      d.IMEI,
			Latitude:  lat,
			Longitude: lng,
		})
	}

	writeJSON(w, latest)
}

// -------------------- DASHBOARD --------------------

func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(dashboardHTML))
}

// -------------------- UTILITIES --------------------

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Println("⚠️ Failed to write JSON response:", err)
	}
}

const dashboardHTML = `<!DOCTYPE html>
<html>
<head>
	<title>Device Dashboard</title>
	<style>
		table {border-collapse: collapse; width: 100%;}
		th, td {border: 1px solid #ddd; padding: 8px;}
		th {background-color: #f2f2f2;}
	</style>
</head>
<body>
	<h2>Devices Dashboard</h2>
	<table id="devices">
		<thead>
			<tr>
				<th>IMEI</th>
				<th>SIM</th>
				<th>Vehicle</th>
				<th>Chassis</th>
				<th>Last Latitude</th>
				<th>Last Longitude</th>
			</tr>
		</thead>
		<tbody></tbody>
	</table>
	<script>
		async function loadDevices() {
			const res = await fetch('/api/devices/list');
			const devices = await res.json();
			const tbody = document.querySelector('#devices tbody');
			tbody.innerHTML = '';
			devices.forEach(d => {
				tbody.innerHTML += '<tr>' +
					'<td>' + d.imei + '</td>' +
					'<td>' + (d.sim||'') + '</td>' +
					'<td>' + (d.vehicle_no||'') + '</td>' +
					'<td>' + (d.chassis_no||'') + '</td>' +
					'<td>' + (d.last_lat||'') + '</td>' +
					'<td>' + (d.last_lng||'') + '</td>' +
				'</tr>';
			});
		}
		loadDevices();
		setInterval(loadDevices, 5000);
	</script>
</body>
</html>`
