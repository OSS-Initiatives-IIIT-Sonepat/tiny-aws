package main

import (
	"net/http"
	"os"
)

// Returns false and writes 401 when a key is configured but the request is unauthorized.
// Agent poll (GET /jobs) and update (PATCH /jobs/{id}) stay open so workers keep working.
func requireAuth(w http.ResponseWriter, r *http.Request) bool {
	key := os.Getenv("TINYAWS_API_KEY")
	if key == "" {
		return true
	}

	if r.URL.Path == "/health" {
		return true
	}
	if r.Method == http.MethodGet && r.URL.Path == "/jobs" {
		return true
	}
	if r.Method == http.MethodPatch && len(r.URL.Path) > len("/jobs/") && r.URL.Path[:len("/jobs/")] == "/jobs/" {
		return true
	}

	auth := r.Header.Get("Authorization")
	if auth != "Bearer "+key {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
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
