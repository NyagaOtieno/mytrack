package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

// Admin represents an admin user
type Admin struct {
	ID    int64
	Name  string
	Email string
	Role  string
}

// -------------------- DB FUNCTION --------------------

// CreateAdmin inserts a new admin into the database with hashed password
func CreateAdmin(name, email, password, role string) (int64, error) {
	// Hash password internally
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("failed to hash password: %v", err)
	}

	var id int64
	err = Pool.QueryRow(
		context.Background(),
		`INSERT INTO admins (name, email, password_hash, role)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		name, email, string(hashed), role,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("failed to create admin: %v", err)
	}

	log.Println("✅ Admin created with ID:", id)
	return id, nil
}

// -------------------- HTTP HANDLER --------------------

// CreateAdminHandler handles POST /api/admins
func CreateAdminHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Println("❌ Invalid JSON:", err)
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	id, err := CreateAdmin(req.Name, req.Email, req.Password, req.Role)
	if err != nil {
		log.Println("❌ CreateAdmin error:", err)
		http.Error(w, "Failed to create admin: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}
