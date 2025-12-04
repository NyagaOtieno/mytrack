package api

import (
	"log"
	"net/http"
)

// -------------------- CORS MIDDLEWARE --------------------
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow your frontend domain here for production
		w.Header().Set("Access-Control-Allow-Origin", "https://trackmykid.vercel.app")
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

// -------------------- START SERVER --------------------
func StartServer(addr string) {
	mux := http.NewServeMux()

	// Wrap each route individually with CORS middleware
	mux.Handle("/api/track", withCORS(http.HandlerFunc(TrackHandler)))
	mux.Handle("/api/devices/create", withCORS(http.HandlerFunc(CreateDeviceHandler)))
	mux.Handle("/api/devices/list", withCORS(http.HandlerFunc(DevicesListHandler)))
	mux.Handle("/api/devices/latest", withCORS(http.HandlerFunc(LatestPositionsHandler)))
	mux.Handle("/dashboard", withCORS(http.HandlerFunc(DashboardHandler)))

	// Catch-all route with CORS
	mux.Handle("/", withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})))

	log.Printf("🚀 HTTP server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("❌ Failed to start HTTP server: %v", err)
	}
}
