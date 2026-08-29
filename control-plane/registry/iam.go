package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// APIKey represents a key/role pair stored in the api_keys table.
type APIKey struct {
	Key  string `json:"key"`
	Role string `json:"role"`
}

// POST /iam/keys — add a key. Body: {"key":"...","role":"admin|readonly"}
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
	_, err := iamDB.Exec(
		`INSERT INTO api_keys (key, role) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET role = excluded.role`,
		k.Key, k.Role,
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	log.Printf("POST /iam/keys - upserted key role=%s", k.Role)
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

// GET /iam/keys — list all keys (roles only, not the key values).
func handleIAMKeyList(w http.ResponseWriter, r *http.Request) {
	rows, err := iamDB.Query(`SELECT key, role FROM api_keys`)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	out := []APIKey{}
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.Key, &k.Role); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		out = append(out, k)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
