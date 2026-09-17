package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

// Message represents an SQS-like queue message.
type Message struct {
	ID           string    `json:"id"`
	QueueName    string    `json:"queue_name"`
	Body         string    `json:"body"`
	VisibleAfter time.Time `json:"visible_after"`
	ReceiveCount int       `json:"receive_count"`
	Deleted      bool      `json:"-"`
}

var (
	db      *sql.DB
	msgSeq  uint64
)

func main() {
	dbPath := getenv("SQS_DB", "sqs.db")
	listenAddr := getenv("SQS_ADDR", ":9003")

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS queues (
			name TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			dead_letter_queue TEXT NOT NULL DEFAULT '',
			max_receive_count INTEGER NOT NULL DEFAULT 5
		);
		CREATE TABLE IF NOT EXISTS messages (
			id            TEXT PRIMARY KEY,
			queue_name    TEXT NOT NULL,
			body          TEXT NOT NULL,
			visible_after TEXT NOT NULL,
			deleted       INTEGER NOT NULL DEFAULT 0,
			receive_count INTEGER NOT NULL DEFAULT 0
		);
	`)
	if err != nil {
		log.Fatal(err)
	}

	// migrate: add DLQ columns if missing
	_, err = db.Exec(`ALTER TABLE queues ADD COLUMN dead_letter_queue TEXT NOT NULL DEFAULT ''`)
	if err == nil {
		log.Println("migrated: added queues.dead_letter_queue")
	}
	_, err = db.Exec(`ALTER TABLE queues ADD COLUMN max_receive_count INTEGER NOT NULL DEFAULT 5`)
	if err == nil {
		log.Println("migrated: added queues.max_receive_count")
	}
	_, err = db.Exec(`ALTER TABLE messages ADD COLUMN receive_count INTEGER NOT NULL DEFAULT 0`)
	if err == nil {
		log.Println("migrated: added messages.receive_count")
	}
	err = nil
	if err != nil {
		log.Fatal(err)
	}

	// load max seq from DB so IDs don't reset
	var raw sql.NullString
	_ = db.QueryRow(`SELECT id FROM messages ORDER BY rowid DESC LIMIT 1`).Scan(&raw)
	if raw.Valid {
		var n uint64
		fmt.Sscanf(raw.String, "msg-%d", &n)
		msgSeq = n
	}

	http.HandleFunc("GET /health", handleHealth)
	http.HandleFunc("POST /queues/{name}", handleCreateQueue)
	http.HandleFunc("GET /queues", handleListQueues)
	http.HandleFunc("POST /queues/{name}/messages", handleSend)
	http.HandleFunc("GET /queues/{name}/messages", handleReceive)
	http.HandleFunc("DELETE /queues/{name}/messages/{id}", handleDelete)

	log.Printf("sqs listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, `{"status":"healthy","service":"sqs"}`)
}

// POST /queues/{name} — create queue.
// Optional JSON body: {"dead_letter_queue": "dlq-name", "max_receive_count": 3}
func handleCreateQueue(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var req struct {
		DeadLetterQueue string `json:"dead_letter_queue"`
		MaxReceiveCount int    `json:"max_receive_count"`
	}
	// body is optional — ignore decode errors
	json.NewDecoder(r.Body).Decode(&req)
	if req.MaxReceiveCount <= 0 {
		req.MaxReceiveCount = 5
	}

	_, err := db.Exec(
		`INSERT INTO queues (name, created_at, dead_letter_queue, max_receive_count)
		 VALUES (?, ?, ?, ?) ON CONFLICT(name) DO NOTHING`,
		name, time.Now().UTC().Format(time.RFC3339), req.DeadLetterQueue, req.MaxReceiveCount,
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	log.Printf("POST /queues/%s - created (dlq=%q, max_receives=%d)", name, req.DeadLetterQueue, req.MaxReceiveCount)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"name":              name,
		"dead_letter_queue": req.DeadLetterQueue,
		"max_receive_count": req.MaxReceiveCount,
	})
}

// GET /queues — list queues.
func handleListQueues(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT name FROM queues`)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			names = append(names, name)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(names)
}

// POST /queues/{name}/messages — send message.
func handleSend(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Body == "" {
		http.Error(w, "body required", http.StatusBadRequest)
		return
	}
	seq := atomic.AddUint64(&msgSeq, 1)
	id := fmt.Sprintf("msg-%d", seq)
	_, err := db.Exec(
		`INSERT INTO messages (id, queue_name, body, visible_after) VALUES (?, ?, ?, ?)`,
		id, name, req.Body, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	log.Printf("POST /queues/%s/messages - %s", name, id)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// GET /queues/{name}/messages — receive next visible message; sets 30s visibility timeout.
// Messages that exceed max_receive_count are moved to the dead letter queue.
func handleReceive(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	now := time.Now().UTC()

	row := db.QueryRow(
		`SELECT id, body, receive_count FROM messages
		 WHERE queue_name = ? AND deleted = 0 AND visible_after <= ?
		 ORDER BY rowid LIMIT 1`,
		name, now.Format(time.RFC3339),
	)
	var id, body string
	var receiveCount int
	if err := row.Scan(&id, &body, &receiveCount); err == sql.ErrNoRows {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nil)
		return
	} else if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	receiveCount++

	// check DLQ policy
	var dlqName string
	var maxReceives int
	err := db.QueryRow(`SELECT dead_letter_queue, max_receive_count FROM queues WHERE name = ?`, name).
		Scan(&dlqName, &maxReceives)
	if err != nil {
		// queue missing DLQ config — treat as no DLQ
		dlqName = ""
		maxReceives = 0
	}

	if dlqName != "" && maxReceives > 0 && receiveCount > maxReceives {
		// move to dead letter queue
		db.Exec(`UPDATE messages SET queue_name = ?, receive_count = ?, visible_after = ? WHERE id = ?`,
			dlqName, receiveCount, now.Format(time.RFC3339), id)
		log.Printf("DLQ: moved %s to %s after %d receives", id, dlqName, receiveCount)
		// return nothing — this message is gone from the source queue
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nil)
		return
	}

	// set visibility timeout: hide for 30s
	visibleAfter := now.Add(30 * time.Second)
	db.Exec(`UPDATE messages SET visible_after = ?, receive_count = ? WHERE id = ?`,
		visibleAfter.Format(time.RFC3339), receiveCount, id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": id, "body": body, "receive_count": receiveCount})
}

// DELETE /queues/{name}/messages/{id} — delete (ack) a message.
func handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	db.Exec(`UPDATE messages SET deleted = 1 WHERE id = ?`, id)
	log.Printf("DELETE message %s", id)
	w.WriteHeader(http.StatusNoContent)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
