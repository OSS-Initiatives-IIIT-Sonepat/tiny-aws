package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVisibilityTimeout(t *testing.T) {
	setupSQSTest(t)
	db.Exec(`INSERT INTO queues (name, created_at) VALUES ('vq', '2026-01-01')`)

	// send a message
	body, _ := json.Marshal(map[string]string{"body": "hello"})
	req := httptest.NewRequest("POST", "/queues/vq/messages", bytes.NewReader(body))
	req.SetPathValue("name", "vq")
	w := httptest.NewRecorder()
	handleSend(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("send status = %d", w.Code)
	}

	// first receive — should get the message
	req = httptest.NewRequest("GET", "/queues/vq/messages", nil)
	req.SetPathValue("name", "vq")
	w = httptest.NewRecorder()
	handleReceive(w, req)
	var msg map[string]string
	json.Unmarshal(w.Body.Bytes(), &msg)
	if msg["body"] != "hello" {
		t.Fatalf("first receive body = %q, want %q", msg["body"], "hello")
	}

	// immediate second receive — message should be invisible (30s timeout)
	req = httptest.NewRequest("GET", "/queues/vq/messages", nil)
	req.SetPathValue("name", "vq")
	w = httptest.NewRecorder()
	handleReceive(w, req)
	if w.Body.String() != "null\n" {
		t.Errorf("second receive = %q, want null (message should be invisible)", w.Body.String())
	}
}

func TestVisibilityTimeout_Expired(t *testing.T) {
	setupSQSTest(t)
	db.Exec(`INSERT INTO queues (name, created_at) VALUES ('vq2', '2026-01-01')`)

	// send a message
	body, _ := json.Marshal(map[string]string{"body": "comeback"})
	req := httptest.NewRequest("POST", "/queues/vq2/messages", bytes.NewReader(body))
	req.SetPathValue("name", "vq2")
	w := httptest.NewRecorder()
	handleSend(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("send status = %d", w.Code)
	}

	// receive — sets visibility timeout 30s in the future
	req = httptest.NewRequest("GET", "/queues/vq2/messages", nil)
	req.SetPathValue("name", "vq2")
	w = httptest.NewRecorder()
	handleReceive(w, req)
	var msg map[string]string
	json.Unmarshal(w.Body.Bytes(), &msg)
	if msg["body"] != "comeback" {
		t.Fatalf("first receive body = %q", msg["body"])
	}

	// manually set visible_after to the past — simulating timeout expiry
	past := time.Now().UTC().Add(-1 * time.Minute).Format(time.RFC3339)
	db.Exec(`UPDATE messages SET visible_after = ? WHERE id = ?`, past, msg["id"])

	// receive again — should get the same message back
	req = httptest.NewRequest("GET", "/queues/vq2/messages", nil)
	req.SetPathValue("name", "vq2")
	w = httptest.NewRecorder()
	handleReceive(w, req)
	var msg2 map[string]string
	json.Unmarshal(w.Body.Bytes(), &msg2)
	if msg2["body"] != "comeback" {
		t.Errorf("after expiry, body = %q, want %q", msg2["body"], "comeback")
	}
	if msg2["id"] != msg["id"] {
		t.Errorf("after expiry, id = %q, want %q", msg2["id"], msg["id"])
	}
}
