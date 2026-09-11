package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupIAMTest(t *testing.T) {
	t.Helper()
	t.Setenv("TINYAWS_API_KEY", "")
	s := NewNodeStore(":memory:")
	iamDB = s.DB()
}

func TestHandleIAMKeyCreate(t *testing.T) {
	setupIAMTest(t)

	body, _ := json.Marshal(APIKey{Key: "k1", Role: "admin"})
	req := httptest.NewRequest("POST", "/iam/keys", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleIAMKeyCreate(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}
	var got APIKey
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.Key != "k1" || got.Role != "admin" {
		t.Errorf("got key=%q role=%q", got.Key, got.Role)
	}
}

func TestHandleIAMKeyCreate_BadRole(t *testing.T) {
	setupIAMTest(t)

	body, _ := json.Marshal(APIKey{Key: "k1", Role: "superadmin"})
	req := httptest.NewRequest("POST", "/iam/keys", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleIAMKeyCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleIAMKeyCreate_WithExpiry(t *testing.T) {
	setupIAMTest(t)

	body, _ := json.Marshal(APIKey{Key: "k1", Role: "admin", ExpiresAt: "2099-12-31T00:00:00Z"})
	req := httptest.NewRequest("POST", "/iam/keys", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleIAMKeyCreate(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}
	var got APIKey
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.ExpiresAt != "2099-12-31T00:00:00Z" {
		t.Errorf("expires_at = %q", got.ExpiresAt)
	}
}

func TestHandleIAMKeyList(t *testing.T) {
	setupIAMTest(t)

	iamDB.Exec(`INSERT INTO api_keys (key, role) VALUES (?, ?)`, "k1", "admin")
	iamDB.Exec(`INSERT INTO api_keys (key, role) VALUES (?, ?)`, "k2", "readonly")

	req := httptest.NewRequest("GET", "/iam/keys", nil)
	w := httptest.NewRecorder()
	handleIAMKeyList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var keys []APIKey
	json.Unmarshal(w.Body.Bytes(), &keys)
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestHandleIAMKeyDelete(t *testing.T) {
	setupIAMTest(t)

	iamDB.Exec(`INSERT INTO api_keys (key, role) VALUES (?, ?)`, "k1", "admin")

	req := httptest.NewRequest("DELETE", "/iam/keys/k1", nil)
	req.SetPathValue("key", "k1")
	w := httptest.NewRecorder()
	handleIAMKeyDelete(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}

	// verify deleted
	var count int
	iamDB.QueryRow(`SELECT COUNT(*) FROM api_keys`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 keys after delete, got %d", count)
	}
}

func TestHandleIAMKeyCreate_UpsertRole(t *testing.T) {
	setupIAMTest(t)

	// create as admin
	body, _ := json.Marshal(APIKey{Key: "k1", Role: "admin"})
	req := httptest.NewRequest("POST", "/iam/keys", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleIAMKeyCreate(w, req)

	// upsert to readonly
	body, _ = json.Marshal(APIKey{Key: "k1", Role: "readonly"})
	req = httptest.NewRequest("POST", "/iam/keys", bytes.NewReader(body))
	w = httptest.NewRecorder()
	handleIAMKeyCreate(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("upsert status = %d, want 201", w.Code)
	}

	// verify role changed
	role := roleForKey("k1")
	if role != "readonly" {
		t.Errorf("role after upsert = %q, want %q", role, "readonly")
	}
}
