package storage

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// -------------------- API KEY FUNCTIONS --------------------

// GenerateAPIKey generates a new random API key
func GenerateAPIKey() (string, error) {
	key := make([]byte, 32) // 32-byte key
	_, err := rand.Read(key)
	if err != nil {
		return "", fmt.Errorf("failed to generate random key: %v", err)
	}
	return base64.URLEncoding.EncodeToString(key), nil
}

// AssignAPIKeyToAdmin inserts a new API key into the database for a given admin
func AssignAPIKeyToAdmin(adminID int) (string, error) {
	// Generate new API key
	key, err := GenerateAPIKey()
	if err != nil {
		return "", err
	}

	// Insert the new API key into the database
	_, err = Pool.Exec(
		context.Background(),
		`INSERT INTO api_keys (api_key, admin_id) VALUES ($1, $2)`,
		key, adminID,
	)
	if err != nil {
		return "", fmt.Errorf("failed to assign API key: %v", err)
	}

	return key, nil
}

// -------------------- HTTP HANDLER --------------------

// CreateAPIKeyHandler handles POST /api/api-keys
func CreateAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Expect JSON with admin_id
	var req struct {
		AdminID int64 `json:"admin_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	key, err := AssignAPIKeyToAdmin(int(req.AdminID))
	if err != nil {
		log.Println("❌ AssignAPIKeyToAdmin error:", err)
		http.Error(w, "Failed to generate API key: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"api_key": key,
	})
}
