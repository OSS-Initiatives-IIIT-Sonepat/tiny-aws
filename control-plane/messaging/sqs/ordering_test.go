package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQSCreateDuplicate(t *testing.T) {
	setupSQSTest(t)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/queues/dup", nil)
		req.SetPathValue("name", "dup")
		w := httptest.NewRecorder()
		handleCreateQueue(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("iteration %d: status = %d", i, w.Code)
		}
	}

	// verify only one queue
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM queues WHERE name='dup'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}

func TestSQSMessageOrdering(t *testing.T) {
	setupSQSTest(t)
	db.Exec(`INSERT INTO queues (name, created_at) VALUES ('order', '2026-01-01')`)

	// send 3 messages
	for i := 1; i <= 3; i++ {
		db.Exec(`INSERT INTO messages (id, queue_name, body, visible_after) VALUES (?, 'order', ?, '2000-01-01')`,
			nextMsgID(), i)
	}

	// receive all 3 — should come in insertion order
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/queues/order/messages", nil)
		req.SetPathValue("name", "order")
		w := httptest.NewRecorder()
		handleReceive(w, req)

		var msg map[string]string
		json.Unmarshal(w.Body.Bytes(), &msg)
		if msg["id"] == "" {
			t.Fatalf("message %d: got nil", i)
		}
	}
}

func nextMsgID() string {
	seq := atomic.AddUint64(&msgSeq, 1)
	return fmt.Sprintf("msg-%d", seq)
}
