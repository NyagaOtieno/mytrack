package api

import (
	"log"
	"net/http"
)

// StartServer starts the HTTP server with all routes
func StartServer(addr string) {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/track", TrackHandler)        // ← ADD THIS
	mux.HandleFunc("/api/devices/create", CreateDeviceHandler)
	mux.HandleFunc("/api/devices/list", DevicesListHandler)
	mux.HandleFunc("/dashboard", DashboardHandler)
	mux.HandleFunc("/api/devices/latest", LatestPositionsHandler)



	// Catch-all
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	log.Printf("🚀 HTTP server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("❌ Failed to start HTTP server: %v", err)
	}
}
