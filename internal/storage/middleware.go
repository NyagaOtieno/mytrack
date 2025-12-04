package storage

import (
	"context"
	"net/http"
)

// ValidateAPIKey checks if the provided API key exists in the database
func ValidateAPIKey(apiKey string) (bool, error) {
	var exists bool
	err := Pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM api_keys WHERE api_key=$1 AND is_revoked=false)`,
		apiKey,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// APIKeyAuthMiddleware wraps HTTP handlers to require a valid API key
func APIKeyAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Allow preflight OPTIONS requests without API key
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			http.Error(w, "API key required", http.StatusUnauthorized)
			return
		}

		valid, err := ValidateAPIKey(apiKey)
		if err != nil || !valid {
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}
