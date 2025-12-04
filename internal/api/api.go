package api

import (
	"log"
	"net/http"
)

// -------------------- CORS MIDDLEWARE --------------------
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Access-Control-Allow-Origin", "*") // allow all
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// If browser OPTIONS request → return OK immediately
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// -------------------- START SERVER --------------------
func StartServer(addr string) {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/track", TrackHandler)
	mux.HandleFunc("/api/devices/create", CreateDeviceHandler)
	mux.HandleFunc("/api/devices/list", DevicesListHandler)
	mux.HandleFunc("/dashboard", DashboardHandler)
	mux.HandleFunc("/api/devices/latest", LatestPositionsHandler)

	// Catch-all
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	// Wrap mux with CORS middleware
	handler := withCORS(mux)

	log.Printf("🚀 HTTP server listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("❌ Failed to start HTTP server: %v", err)
	}
}
