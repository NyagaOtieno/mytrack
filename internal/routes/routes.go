package routes

import (
	"encoding/json"
	"net/http"

	"fmb920-server/internal/storage"
)

type Device struct {
	ID            int64   `json:"id"`
	IMEI          string  `json:"imei"`
	SIMNumber     string  `json:"sim_number,omitempty"`
	VehicleNumber string  `json:"vehicle_number,omitempty"`
	ChassisNumber string  `json:"chassis_number,omitempty"`
	LastLat       float64 `json:"last_lat,omitempty"`
	LastLng       float64 `json:"last_lng,omitempty"`
}

/* ----------------------------- Helpers ----------------------------- */

func respondJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, code int, msg string) {
	respondJSON(w, code, map[string]interface{}{
		"success": false,
		"error":   msg,
	})
}

/* --------------------------- Create Device -------------------------- */
// POST /api/devices/create
func CreateDeviceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, 405, "Method not allowed")
		return
	}

	var d Device
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		respondError(w, 400, "Invalid request body")
		return
	}

	id, err := storage.CreateDevice(d.IMEI, d.SIMNumber, d.VehicleNumber, d.ChassisNumber)
	if err != nil {
		respondError(w, 500, "Failed to create device: "+err.Error())
		return
	}

	respondJSON(w, 201, map[string]int64{"id": id})
}

/* ------------------------ List Devices ------------------------ */
// GET /api/devices/list
func DevicesListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, 405, "Method not allowed")
		return
	}

	devices, err := storage.GetAllDevicesWithLastPosition()
	if err != nil {
		respondError(w, 500, "Failed to fetch devices: "+err.Error())
		return
	}

	respondJSON(w, 200, devices)
}

/* ------------------------ Latest Position ------------------------ */
// GET /api/devices/latest?imei=...
func LatestPositionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, 405, "Method not allowed")
		return
	}

	imei := r.URL.Query().Get("imei")
	if imei == "" {
		respondError(w, 400, "imei query parameter is required")
		return
	}

	// Direct DB lookup instead of looping all devices
	device, err := storage.GetDeviceByIMEI(imei)
	if err != nil {
		respondError(w, 404, "Device not found")
		return
	}

	pos, err := storage.GetLatestPosition(device.ID)
	if err != nil {
		respondError(w, 404, "Latest position not found")
		return
	}

	resp := map[string]interface{}{
		"imei":       imei,
		"lat":        pos.Latitude,
		"lng":        pos.Longitude,
		"speed":      pos.Speed,
		"angle":      pos.Angle,
		"altitude":   pos.Altitude,
		"satellites": pos.Satellites,
		"timestamp":  pos.Timestamp,
	}

	respondJSON(w, 200, resp)
}

/* ----------------------------- Dashboard ----------------------------- */
// GET /dashboard
func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, 405, "Method not allowed")
		return
	}

	html := `<!DOCTYPE html>
<html>
<head>
    <title>Devices Dashboard</title>
    <style>
        body { font-family: Arial, sans-serif; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; }
        th { background-color: #f7f7f7; }
        h2 { font-size: 22px; }
    </style>
</head>
<body>
    <h2>Devices Dashboard</h2>
    
    <table id="devices">
        <thead>
            <tr>
                <th>IMEI</th>
                <th>SIM Number</th>
                <th>Vehicle Number</th>
                <th>Chassis Number</th>
                <th>Last Latitude</th>
                <th>Last Longitude</th>
            </tr>
        </thead>
        <tbody></tbody>
    </table>

    <script>
        async function loadDevices() {
            try {
                const res = await fetch('/api/devices/list');
                if (!res.ok) return;

                const devices = await res.json();
                const tbody = document.querySelector('#devices tbody');
                tbody.innerHTML = '';

                devices.forEach(d => {
                    tbody.innerHTML += "<tr>" +
                        "<td>" + (d.imei || '') + "</td>" +
                        "<td>" + (d.sim_number || '') + "</td>" +
                        "<td>" + (d.vehicle_number || '') + "</td>" +
                        "<td>" + (d.chassis_number || '') + "</td>" +
                        "<td>" + (d.last_lat || '') + "</td>" +
                        "<td>" + (d.last_lng || '') + "</td>" +
                    "</tr>";
                });
            } catch (e) {
                console.error('Error loading devices:', e);
            }
        }

        loadDevices();
        setInterval(loadDevices, 5000);
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
