package api

import (
	"log"
	"net/http"
	"os"
	"strings"
)

// -------------------- CORS MIDDLEWARE --------------------
// withCORS wraps HTTP handlers to handle CORS preflight and headers.
// Allows multiple origins, configurable via environment variable or defaults.
func withCORS(next http.Handler) http.Handler {
	// Read allowed origins from environment (comma-separated)
	allowedOrigins := []string{"https://trackmykid.vercel.app", "http://localhost:5173"}
	if envOrigins := os.Getenv("ALLOWED_ORIGINS"); envOrigins != "" {
		allowedOrigins = strings.Split(envOrigins, ",")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		for _, o := range allowedOrigins {
			if o == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight OPTIONS request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// -------------------- SERVER START --------------------
// StartServer initializes and starts the HTTP server with all routes.
func StartServer(addr string) {
	mux := http.NewServeMux()

	// Register routes with CORS
	registerRoute(mux, "/api/track", TrackHandler)
	registerRoute(mux, "/api/devices/create", CreateDeviceHandler)
	registerRoute(mux, "/api/devices/list", DevicesListHandler)
	registerRoute(mux, "/api/devices/latest", LatestPositionsHandler)
	registerRoute(mux, "/dashboard", DashboardHandler)

	// Catch-all 404 route
	mux.Handle("/", withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})))

	log.Printf("🚀 HTTP server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("❌ Failed to start HTTP server: %v", err)
	}
}

// -------------------- HELPER --------------------
// registerRoute wraps the handler with CORS and registers it to the mux
func registerRoute(mux *http.ServeMux, path string, handlerFunc http.HandlerFunc) {
	mux.Handle(path, withCORS(handlerFunc))
}
