package main

import (
	"net/http"
	"os"
)

// Returns false and writes 401 when a key is configured but the request is unauthorized.
func requireAuth(w http.ResponseWriter, r *http.Request) bool {
	key := os.Getenv("TINYAWS_API_KEY")
	if key == "" {
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
