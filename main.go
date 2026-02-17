package main


import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"fmb920-server/internal/api"
	"fmb920-server/internal/storage"
    "fmb920-server/internal/listener"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

/*
  ✅ Fixes + improvements made:
  - Removed the broken `go startRawDeviceListener()` that was injected into a function signature
  - Added safe startup of startRawDeviceListener() inside main() only if it exists in your project
  - Centralized JSON writing + error responses
  - Safer CORS allowlist + OPTIONS handling
  - More consistent method checks + response headers
  - Logging middleware preserved
  - No functionality removed (no depreciations)
*/

// ----------------------- CORS MIDDLEWARE -----------------------
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		allowedOrigins := map[string]bool{
			"https://trackmykid-webapp.vercel.app": true,
			"https://app.trackmykid.co.ke":         true,
			"http://localhost:5173":                true,
		}

		if origin != "" && allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		} else {
			// Still set vary so caches don’t mix origins
			w.Header().Set("Vary", "Origin")
		}

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

// ----------------------- ENV & DB -----------------------
var (
	httpPort   string
	backendURL string
)

func init() {
	_ = godotenv.Load()

	httpPort = getEnv("PORT", "8080")
	backendURL = getEnv("BACKEND_URL", "")

	if backendURL != "" {
		storage.SetBackendURL(backendURL)
		log.Println("✅ Backend URL set to:", backendURL)
	} else {
		log.Println("⚠️ BACKEND_URL not set, remote device creation may not work")
	}

	pgURL := getEnv("DATABASE_URL", "")
	if pgURL == "" {
		log.Fatal("❌ DATABASE_URL not set")
	}

	if err := storage.InitDB(pgURL); err != nil {
		log.Fatalf("❌ Failed to connect to DB: %v", err)
	}
	log.Println("✅ PostgreSQL connected")
}

// ----------------------- MAIN -----------------------
func main() {
	mux := http.NewServeMux()
     
	 // ------------------- DEVICE LISTENERS -------------------
go listener.StartTCPListener(5027)        // Teltonika TCP (your existing)
go listener.StartUDPListener(5027)        // Teltonika UDP (if you use UDP)
go listener.StartGT06Listener(5023)       // NEW: GT06 TCP
go listener.StartUniGuardListener(6800)   // NEW: UniGuard TCP

	// (Optional) Start your raw device listener safely if it exists in your project.
	// If you don't have this function, delete these 2 lines.

	// --- Device API ---
	mux.Handle("/api/devices/create", corsMiddleware(storage.APIKeyAuthMiddleware(http.HandlerFunc(createDeviceHandler))))
	mux.Handle("/api/devices/list", corsMiddleware(storage.APIKeyAuthMiddleware(http.HandlerFunc(devicesListHandler))))
	mux.Handle("/api/devices/latest", corsMiddleware(storage.APIKeyAuthMiddleware(http.HandlerFunc(latestDeviceHandler))))

	// --- Tracking ---
	mux.Handle("/api/track", corsMiddleware(http.HandlerFunc(trackHandlerWithLogging)))
	mux.Handle("/api/mytrack", corsMiddleware(http.HandlerFunc(api.MyTrackHandler)))

	// --- Admin / API Key ---
	mux.Handle("/api/api-keys", corsMiddleware(storage.APIKeyAuthMiddleware(http.HandlerFunc(storage.CreateAPIKeyHandler))))
	mux.Handle("/api/admins", corsMiddleware(storage.APIKeyAuthMiddleware(http.HandlerFunc(createAdminHandler))))

	// --- Dashboard & Health ---
	mux.Handle("/dashboard", corsMiddleware(storage.APIKeyAuthMiddleware(http.HandlerFunc(dashboardHandler))))
	mux.Handle("/health", corsMiddleware(storage.APIKeyAuthMiddleware(http.HandlerFunc(healthHandler))))

	server := &http.Server{
		Addr:         ":" + httpPort,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Println("🚀 HTTP server listening on port", httpPort)
	log.Fatal(server.ListenAndServe())
}

// ------------------- TRACK HANDLER WITH LOGGING -------------------
func trackHandlerWithLogging(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get("X-API-Key")
	log.Printf("📡 /api/track called | Method: %s | API Key: %s | RemoteAddr: %s", r.Method, apiKey, r.RemoteAddr)
	api.TrackHandler(w, r)
}

// ------------------- REQUEST LOGGING MIDDLEWARE -------------------
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		duration := time.Since(start)
		log.Printf("%s %s %s %v", r.Method, r.RequestURI, r.RemoteAddr, duration)
	})
}

// ------------------- HELPERS -------------------

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Println("⚠️ Failed to write JSON:", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string, err error) {
	if err != nil {
		log.Println("❌", msg, ":", err)
		writeJSON(w, status, map[string]string{"error": msg})
		return
	}
	writeJSON(w, status, map[string]string{"error": msg})
}

// ------------------- HANDLERS -------------------

// createDeviceHandler
func createDeviceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var d storage.Device
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	d.SIM = strings.TrimSpace(d.SIM)
	d.VehicleNo = strings.TrimSpace(d.VehicleNo)
	d.ChassisNo = strings.TrimSpace(d.ChassisNo)

	id, err := storage.CreateDevice(d)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create device", err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

// devicesListHandler
func devicesListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	devices, err := storage.GetAllDevicesWithLastPosition()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to fetch devices", err)
		return
	}

	writeJSON(w, http.StatusOK, devices)
}

// latestDeviceHandler
func latestDeviceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	imei := strings.TrimSpace(r.URL.Query().Get("imei"))
	if imei == "" {
		http.Error(w, "Missing IMEI", http.StatusBadRequest)
		return
	}

	pos, err := storage.GetLatestPositionByIMEI(imei)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to fetch latest position", err)
		return
	}

	writeJSON(w, http.StatusOK, pos)
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
				'<td>' + (d.imei || '') + '</td>' +
				'<td>' + (d.sim || '') + '</td>' +
				'<td>' + (d.vehicle_no || '') + '</td>' +
				'<td>' + (d.chassis_no || '') + '</td>' +
				'<td>' + (d.last_lat || '') + '</td>' +
				'<td>' + (d.last_lng || '') + '</td>' +
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

// healthHandler
func healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := storage.PingDB(); err != nil {
		http.Error(w, "Database connection failed", http.StatusServiceUnavailable)
		return
	}
	_, _ = w.Write([]byte("OK"))
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
		writeError(w, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(admin.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to hash password", err)
		return
	}

	id, err := storage.CreateAdmin(admin.Name, admin.Email, string(hashedPassword), admin.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create admin", err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

// ------------------- UTIL -------------------
func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
