package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSNSCreateDuplicateTopic(t *testing.T) {
	setupSNSTest(t)

	// create twice — should not error (ON CONFLICT DO NOTHING)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/topics/dup", nil)
		req.SetPathValue("name", "dup")
		w := httptest.NewRecorder()
		handleCreateTopic(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("iteration %d: status = %d", i, w.Code)
		}
	}

	// verify only one topic exists
	rows, _ := db.Query(`SELECT COUNT(*) FROM topics WHERE name='dup'`)
	defer rows.Close()
	rows.Next()
	var count int
	rows.Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 topic, got %d", count)
	}
}

func TestSNSPublish_NoSubscribers(t *testing.T) {
	setupSNSTest(t)
	db.Exec(`INSERT INTO topics (name, created_at) VALUES ('empty-topic', '2026-01-01')`)

	body, _ := json.Marshal(map[string]string{"message": "hello"})
	req := httptest.NewRequest("POST", "/topics/empty-topic/publish", bytes.NewReader(body))
	req.SetPathValue("name", "empty-topic")
	w := httptest.NewRecorder()
	handlePublish(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var result map[string]int
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["delivered"] != 0 {
		t.Errorf("delivered = %d, want 0", result["delivered"])
	}
}

func TestSNSMultipleSubscribers(t *testing.T) {
	setupSNSTest(t)

	received := make(chan string, 3)
	for i := 0; i < 3; i++ {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			received <- "ok"
			w.WriteHeader(http.StatusOK)
		}))
		defer s.Close()
		db.Exec(`INSERT INTO subscriptions (id, topic, endpoint) VALUES (?, 'multi', ?)`,
			nextSubID(), s.URL)
	}
	db.Exec(`INSERT INTO topics (name, created_at) VALUES ('multi', '2026-01-01')`)

	body, _ := json.Marshal(map[string]string{"message": "broadcast"})
	req := httptest.NewRequest("POST", "/topics/multi/publish", bytes.NewReader(body))
	req.SetPathValue("name", "multi")
	w := httptest.NewRecorder()
	handlePublish(w, req)

	var result map[string]int
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["delivered"] != 3 {
		t.Errorf("delivered = %d, want 3", result["delivered"])
	}
}

func nextSubID() string {
	seq := atomic.AddUint64(&subSeq, 1)
	return fmt.Sprintf("sub-%d", seq)
}
