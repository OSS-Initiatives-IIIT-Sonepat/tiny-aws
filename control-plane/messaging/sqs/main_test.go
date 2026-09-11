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

func setupSQSTest(t *testing.T) {
	t.Helper()
	var err error
	db, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS queues (name TEXT PRIMARY KEY, created_at TEXT NOT NULL);
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY, queue_name TEXT NOT NULL,
			body TEXT NOT NULL, visible_after TEXT NOT NULL,
			deleted INTEGER NOT NULL DEFAULT 0
		);
	`)
	if err != nil {
		t.Fatal(err)
	}
	msgSeq = 0
	t.Cleanup(func() { db.Close() })
}

func TestSQSHealth(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handleHealth(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestSQSCreateQueue(t *testing.T) {
	setupSQSTest(t)

	req := httptest.NewRequest("POST", "/queues/test-q", nil)
	req.SetPathValue("name", "test-q")
	w := httptest.NewRecorder()
	handleCreateQueue(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}
}

func TestSQSListQueues(t *testing.T) {
	setupSQSTest(t)
	db.Exec(`INSERT INTO queues (name, created_at) VALUES ('q1', '2026-01-01')`)
	db.Exec(`INSERT INTO queues (name, created_at) VALUES ('q2', '2026-01-01')`)

	req := httptest.NewRequest("GET", "/queues", nil)
	w := httptest.NewRecorder()
	handleListQueues(w, req)

	var names []string
	json.Unmarshal(w.Body.Bytes(), &names)
	if len(names) != 2 {
		t.Errorf("expected 2 queues, got %d", len(names))
	}
}

func TestSQSSendAndReceive(t *testing.T) {
	setupSQSTest(t)
	db.Exec(`INSERT INTO queues (name, created_at) VALUES ('jobs', '2026-01-01')`)

	// send
	body, _ := json.Marshal(map[string]string{"body": `{"command":"echo hi"}`})
	sendReq := httptest.NewRequest("POST", "/queues/jobs/messages", bytes.NewReader(body))
	sendReq.SetPathValue("name", "jobs")
	sw := httptest.NewRecorder()
	handleSend(sw, sendReq)
	if sw.Code != http.StatusCreated {
		t.Fatalf("send status = %d", sw.Code)
	}

	// receive
	recvReq := httptest.NewRequest("GET", "/queues/jobs/messages", nil)
	recvReq.SetPathValue("name", "jobs")
	rw := httptest.NewRecorder()
	handleReceive(rw, recvReq)

	var msg map[string]string
	json.Unmarshal(rw.Body.Bytes(), &msg)
	if msg["body"] != `{"command":"echo hi"}` {
		t.Errorf("body = %q", msg["body"])
	}
}

func TestSQSSendEmptyBody(t *testing.T) {
	setupSQSTest(t)
	db.Exec(`INSERT INTO queues (name, created_at) VALUES ('jobs', '2026-01-01')`)

	body, _ := json.Marshal(map[string]string{"body": ""})
	req := httptest.NewRequest("POST", "/queues/jobs/messages", bytes.NewReader(body))
	req.SetPathValue("name", "jobs")
	w := httptest.NewRecorder()
	handleSend(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestSQSReceiveEmpty(t *testing.T) {
	setupSQSTest(t)
	db.Exec(`INSERT INTO queues (name, created_at) VALUES ('empty', '2026-01-01')`)

	req := httptest.NewRequest("GET", "/queues/empty/messages", nil)
	req.SetPathValue("name", "empty")
	w := httptest.NewRecorder()
	handleReceive(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	// should return null
	if w.Body.String() != "null\n" {
		t.Errorf("body = %q, want null", w.Body.String())
	}
}

func TestSQSDeleteMessage(t *testing.T) {
	setupSQSTest(t)
	db.Exec(`INSERT INTO queues (name, created_at) VALUES ('q', '2026-01-01')`)
	db.Exec(`INSERT INTO messages (id, queue_name, body, visible_after) VALUES ('msg-1', 'q', 'hi', '2000-01-01')`)

	req := httptest.NewRequest("DELETE", "/queues/q/messages/msg-1", nil)
	req.SetPathValue("id", "msg-1")
	req.SetPathValue("name", "q")
	w := httptest.NewRecorder()
	handleDelete(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}

	// verify deleted flag
	var deleted int
	db.QueryRow(`SELECT deleted FROM messages WHERE id='msg-1'`).Scan(&deleted)
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}
}
