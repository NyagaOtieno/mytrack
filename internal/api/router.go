package api

import (
	"net/http"
)

// NewRouter returns a http.Handler with all routes registered
func NewRouter() http.Handler {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/track", TrackHandler)
	mux.HandleFunc("/api/devices/list", DevicesListHandler)
	mux.HandleFunc("/api/devices/latest", LatestPositionsHandler)
	mux.HandleFunc("/api/devices/create", CreateDeviceHandler)

	// Dashboard route
	mux.HandleFunc("/dashboard", DashboardHandler)

	return mux
}
