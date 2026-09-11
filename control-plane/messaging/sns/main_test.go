package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "modernc.org/sqlite"
)

func setupSNSTest(t *testing.T) {
	t.Helper()
	var err error
	db, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS topics (name TEXT PRIMARY KEY, created_at TEXT NOT NULL);
		CREATE TABLE IF NOT EXISTS subscriptions (id TEXT PRIMARY KEY, topic TEXT NOT NULL, endpoint TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}
	subSeq = 0
	t.Cleanup(func() { db.Close() })
}

func TestSNSHealth(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handleHealth(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestSNSCreateTopic(t *testing.T) {
	setupSNSTest(t)

	req := httptest.NewRequest("POST", "/topics/test-topic", nil)
	req.SetPathValue("name", "test-topic")
	w := httptest.NewRecorder()
	handleCreateTopic(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}
}

func TestSNSListTopics(t *testing.T) {
	setupSNSTest(t)
	db.Exec(`INSERT INTO topics (name, created_at) VALUES ('t1', '2026-01-01')`)
	db.Exec(`INSERT INTO topics (name, created_at) VALUES ('t2', '2026-01-01')`)

	req := httptest.NewRequest("GET", "/topics", nil)
	w := httptest.NewRecorder()
	handleListTopics(w, req)

	var names []string
	json.Unmarshal(w.Body.Bytes(), &names)
	if len(names) != 2 {
		t.Errorf("expected 2 topics, got %d", len(names))
	}
}

func TestSNSSubscribe(t *testing.T) {
	setupSNSTest(t)
	db.Exec(`INSERT INTO topics (name, created_at) VALUES ('events', '2026-01-01')`)

	body, _ := json.Marshal(map[string]string{"endpoint": "http://localhost:9999/hook"})
	req := httptest.NewRequest("POST", "/topics/events/subscribe", bytes.NewReader(body))
	req.SetPathValue("name", "events")
	w := httptest.NewRecorder()
	handleSubscribe(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}
	var got map[string]string
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["id"] != "sub-1" {
		t.Errorf("id = %q", got["id"])
	}
}

func TestSNSSubscribe_EmptyEndpoint(t *testing.T) {
	setupSNSTest(t)

	body, _ := json.Marshal(map[string]string{"endpoint": ""})
	req := httptest.NewRequest("POST", "/topics/t/subscribe", bytes.NewReader(body))
	req.SetPathValue("name", "t")
	w := httptest.NewRecorder()
	handleSubscribe(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestSNSPublish(t *testing.T) {
	setupSNSTest(t)

	// Create a mock subscriber
	subscriber := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer subscriber.Close()

	db.Exec(`INSERT INTO topics (name, created_at) VALUES ('events', '2026-01-01')`)
	db.Exec(`INSERT INTO subscriptions (id, topic, endpoint) VALUES ('sub-1', 'events', ?)`, subscriber.URL)

	body, _ := json.Marshal(map[string]string{"message": "hello"})
	req := httptest.NewRequest("POST", "/topics/events/publish", bytes.NewReader(body))
	req.SetPathValue("name", "events")
	w := httptest.NewRecorder()
	handlePublish(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var result map[string]int
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["delivered"] != 1 {
		t.Errorf("delivered = %d, want 1", result["delivered"])
	}
}

func TestSNSPublish_EmptyMessage(t *testing.T) {
	setupSNSTest(t)

	body, _ := json.Marshal(map[string]string{"message": ""})
	req := httptest.NewRequest("POST", "/topics/t/publish", bytes.NewReader(body))
	req.SetPathValue("name", "t")
	w := httptest.NewRecorder()
	handlePublish(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
