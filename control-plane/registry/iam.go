package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// APIKey represents a key/role pair stored in the api_keys table.
type APIKey struct {
	Key       string `json:"key"`
	Role      string `json:"role"`
	ExpiresAt string `json:"expires_at,omitempty"` // RFC3339 or empty = never
}

// POST /iam/keys — add a key. Body: {"key":"...","role":"admin|readonly","expires_at":"2026-12-31T00:00:00Z"}
func handleIAMKeyCreate(w http.ResponseWriter, r *http.Request) {
	var k APIKey
	if err := json.NewDecoder(r.Body).Decode(&k); err != nil || k.Key == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if k.Role != "admin" && k.Role != "readonly" {
		http.Error(w, "role must be admin or readonly", http.StatusBadRequest)
		return
	}
	var expiresAt interface{} = nil
	if k.ExpiresAt != "" {
		expiresAt = k.ExpiresAt
	}
	_, err := iamDB.Exec(
		`INSERT INTO api_keys (key, role, expires_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET role = excluded.role, expires_at = excluded.expires_at`,
		k.Key, k.Role, expiresAt,
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	log.Printf("POST /iam/keys - upserted key role=%s expires=%s", k.Role, k.ExpiresAt)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(k)
}

// DELETE /iam/keys/{key} — remove a key.
func handleIAMKeyDelete(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		http.Error(w, "key required", http.StatusBadRequest)
		return
	}
	if _, err := iamDB.Exec(`DELETE FROM api_keys WHERE key = ?`, key); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	log.Printf("DELETE /iam/keys/%s", key)
	w.WriteHeader(http.StatusNoContent)
}

// GET /iam/keys — list all keys with role and expiry.
func handleIAMKeyList(w http.ResponseWriter, r *http.Request) {
	rows, err := iamDB.Query(`SELECT key, role, expires_at FROM api_keys`)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	out := []APIKey{}
	for rows.Next() {
		var k APIKey
		var exp *string
		if err := rows.Scan(&k.Key, &k.Role, &exp); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		if exp != nil {
			k.ExpiresAt = *exp
		}
		out = append(out, k)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
