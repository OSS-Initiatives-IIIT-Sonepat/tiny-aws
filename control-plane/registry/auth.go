package main

import (
	"database/sql"
	"net/http"
	"os"
	"time"
)

// iamDB is the shared registry DB, wired in main after store is created.
var iamDB *sql.DB

// roleForKey looks up key in the api_keys table; falls back to the env var key (admin).
// Returns "" if the key is not recognized or is expired.
func roleForKey(key string) string {
	envKey := os.Getenv("TINYAWS_API_KEY")
	if envKey != "" && key == envKey {
		return "admin"
	}
	if iamDB == nil {
		return ""
	}
	var role string
	var expiresAt sql.NullString
	err := iamDB.QueryRow(`SELECT role, expires_at FROM api_keys WHERE key = ?`, key).Scan(&role, &expiresAt)
	if err != nil {
		return ""
	}
	// check expiry — NULL means never expires
	if expiresAt.Valid && expiresAt.String != "" {
		exp, err := time.Parse(time.RFC3339, expiresAt.String)
		if err == nil && time.Now().After(exp) {
			return "" // expired
		}
	}
	return role
}

// Returns false and writes 401/403 when a key is configured but the request is unauthorized.
// admin: full access. readonly: GET only.
func requireAuth(w http.ResponseWriter, r *http.Request) bool {
	envKey := os.Getenv("TINYAWS_API_KEY")
	// Auth is only enforced when env key is set OR api_keys table has rows.
	hasIAM := envKey != ""
	if !hasIAM && iamDB != nil {
		var n int
		iamDB.QueryRow(`SELECT COUNT(*) FROM api_keys`).Scan(&n)
		hasIAM = n > 0
	}
	if !hasIAM {
		return true
	}

	if r.URL.Path == "/health" {
		return true
	}
	if r.URL.Path == "/nodes/register" && r.Method == http.MethodPost {
		return true
	}
	if r.URL.Path == "/nodes/heartbeat" && r.Method == http.MethodPost {
		return true
	}

	auth := r.Header.Get("Authorization")
	if len(auth) < 8 || auth[:7] != "Bearer " {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	role := roleForKey(auth[7:])
	if role == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	// readonly keys may not mutate.
	if role == "readonly" && r.Method != http.MethodGet {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r) {
			return
		}
		next(w, r)
	}
}
