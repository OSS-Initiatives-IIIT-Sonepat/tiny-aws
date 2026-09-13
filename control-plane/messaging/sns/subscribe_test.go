package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test subscribing multiple endpoints to the same topic.
func TestSNSSubscribeMultiple(t *testing.T) {
	setupSNSTest(t)
	db.Exec(`INSERT INTO topics (name, created_at) VALUES ('alerts', '2026-01-01')`)

	endpoints := []string{
		"http://localhost:9001/hook",
		"http://localhost:9002/hook",
		"http://localhost:9003/hook",
	}

	for _, ep := range endpoints {
		body, _ := json.Marshal(map[string]string{"endpoint": ep})
		req := httptest.NewRequest("POST", "/topics/alerts/subscribe", bytes.NewReader(body))
		req.SetPathValue("name", "alerts")
		w := httptest.NewRecorder()
		handleSubscribe(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("subscribe %s: status = %d, want 201", ep, w.Code)
		}
	}

	// verify all three subscriptions exist
	rows, err := db.Query(`SELECT endpoint FROM subscriptions WHERE topic = 'alerts'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var stored []string
	for rows.Next() {
		var ep string
		rows.Scan(&ep)
		stored = append(stored, ep)
	}
	if len(stored) != 3 {
		t.Errorf("expected 3 subscriptions, got %d", len(stored))
	}

	// publish should report 3 delivered
	msg, _ := json.Marshal(map[string]string{"message": "test alert"})
	pubReq := httptest.NewRequest("POST", "/topics/alerts/publish", bytes.NewReader(msg))
	pubReq.SetPathValue("name", "alerts")
	pw := httptest.NewRecorder()
	handlePublish(pw, pubReq)

	var result map[string]int
	json.Unmarshal(pw.Body.Bytes(), &result)
	if result["delivered"] != 3 {
		t.Errorf("delivered = %d, want 3", result["delivered"])
	}
}
