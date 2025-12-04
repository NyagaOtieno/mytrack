package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"fmb920-server/internal/storage"
	"fmb920-server/internal/api"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

// ----------------------- CORS MIDDLEWARE -----------------------
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowedOrigins := map[string]bool{
			"https://trackmykid-webapp.vercel.app": true,
			"https://app.trackmykid.co.ke": true,
			"http://localhost:5173":                true,
		}

		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

var (
	httpPort   string
	backendURL string
)

func init() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ No .env file found, relying on system environment variables")
	}

	httpPort = getEnv("PORT", "8080")

	// Set backend URL
	backendURL = getEnv("BACKEND_URL", "")
	if backendURL != "" {
		storage.SetBackendURL(backendURL)
		log.Println("✅ Backend URL set to:", backendURL)
	} else {
		log.Println("⚠️ BACKEND_URL not set, remote device creation will not work")
	}

	// PostgreSQL
	pgURL := getEnv("DATABASE_URL", "")
	if pgURL == "" {
		log.Fatal("❌ DATABASE_URL not set")
	}

	if err := storage.InitDB(pgURL); err != nil {
		log.Fatalf("❌ Failed to connect to DB: %v", err)
	}
	log.Println("✅ PostgreSQL connected")
}

func main() {
	mux := http.NewServeMux()

	// API routes wrapped with API key middleware
	mux.HandleFunc("/api/devices/create", storage.APIKeyAuthMiddleware(createDeviceHandler))
	mux.HandleFunc("/api/devices/list", storage.APIKeyAuthMiddleware(devicesListHandler))
	mux.HandleFunc("/api/devices/latest", storage.APIKeyAuthMiddleware(latestDeviceHandler))
	mux.Handle("/api/track", corsMiddleware(http.HandlerFunc(api.TrackHandler)))
	mux.HandleFunc("/dashboard", storage.APIKeyAuthMiddleware(dashboardHandler))
	mux.HandleFunc("/health", storage.APIKeyAuthMiddleware(healthHandler))
	mux.HandleFunc("/api/api-keys", storage.APIKeyAuthMiddleware(storage.CreateAPIKeyHandler))
	mux.HandleFunc("/api/admins", storage.APIKeyAuthMiddleware(createAdminHandler))
	



	// ---------------- MyTrack Proxy ----------------
     mux.Handle("/api/mytrack", corsMiddleware(http.HandlerFunc(api.MyTrackHandler)))






	server := &http.Server{
		Addr:         ":" + httpPort,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Println("🚀 HTTP server listening on port", httpPort)
	log.Fatal(server.ListenAndServe())
}

// ------------------- MyTrack Proxy Handler -------------------
func myTrackProxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodOptions {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	token := os.Getenv("MYTRACK_API_KEY")
	if token == "" {
		http.Error(w, "API key not set", http.StatusInternalServerError)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://mytrack-production.up.railway.app/api/devices/list", nil)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("X-API-Key", token)

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to fetch devices", http.StatusInternalServerError)
		log.Println("Error fetching devices:", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// ------------------- EXISTING HANDLERS -------------------

// createDeviceHandler
func createDeviceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var d storage.Device
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	d.SIM = strings.TrimSpace(d.SIM)
	d.VehicleNo = strings.TrimSpace(d.VehicleNo)
	d.ChassisNo = strings.TrimSpace(d.ChassisNo)

	id, err := storage.CreateDevice(d)
	if err != nil {
		log.Println("❌ CreateDevice error:", err)
		http.Error(w, "Failed to create device: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}

// devicesListHandler
func devicesListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	devices, err := storage.GetAllDevicesWithLastPosition()
	if err != nil {
		log.Println("❌ GetAllDevicesWithLastPosition error:", err)
		http.Error(w, "Failed to fetch devices: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

// latestDeviceHandler
func latestDeviceHandler(w http.ResponseWriter, r *http.Request) {
	imei := r.URL.Query().Get("imei")
	if r.Method != http.MethodGet || imei == "" {
		http.Error(w, "Missing IMEI", http.StatusBadRequest)
		return
	}

	pos, err := storage.GetLatestPositionByIMEI(imei)
	if err != nil {
		log.Println("❌ GetLatestPositionByIMEI error:", err)
		http.Error(w, "Failed to fetch latest position: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pos)
}

// httpTrackHandler
func httpTrackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var positions []storage.Position
	if err := json.NewDecoder(r.Body).Decode(&positions); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := storage.SavePositions(positions); err != nil {
		log.Println("❌ SavePositions error:", err)
		http.Error(w, "Failed to save positions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// dashboardHandler
func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	html := `<!DOCTYPE html>
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
	try {
		const res = await fetch('/api/devices/list');
		const devices = await res.json();
		const tbody = document.querySelector('#devices tbody');
		tbody.innerHTML = '';
		devices.forEach(d => {
			tbody.innerHTML += '<tr>' +
				'<td>' + d.imei + '</td>' +
				'<td>' + d.sim + '</td>' +
				'<td>' + d.vehicle_no + '</td>' +
				'<td>' + d.chassis_no + '</td>' +
				'<td>' + d.last_lat + '</td>' +
				'<td>' + d.last_lng + '</td>' +
			'</tr>';
		});
	} catch(err) {
		console.error("Failed to load devices:", err);
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

// healthHandler
func healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := storage.PingDB(); err != nil {
		http.Error(w, "Database connection failed", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("OK"))
}

// createAdminHandler
func createAdminHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var admin struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&admin); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(admin.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	id, err := storage.CreateAdmin(admin.Name, admin.Email, string(hashedPassword), admin.Role)
	if err != nil {
		http.Error(w, "Failed to create admin: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}

// ------------------- UTIL -------------------
func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
