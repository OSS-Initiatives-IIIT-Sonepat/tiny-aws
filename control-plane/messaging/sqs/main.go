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
			created_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS messages (
			id            TEXT PRIMARY KEY,
			queue_name    TEXT NOT NULL,
			body          TEXT NOT NULL,
			visible_after TEXT NOT NULL,
			deleted       INTEGER NOT NULL DEFAULT 0
		);
	`)
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
func handleCreateQueue(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	_, err := db.Exec(
		`INSERT INTO queues (name, created_at) VALUES (?, ?) ON CONFLICT(name) DO NOTHING`,
		name, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	log.Printf("POST /queues/%s - created", name)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"name": name})
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
		rows.Scan(&name)
		names = append(names, name)
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
func handleReceive(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	now := time.Now().UTC()

	row := db.QueryRow(
		`SELECT id, body FROM messages
		 WHERE queue_name = ? AND deleted = 0 AND visible_after <= ?
		 ORDER BY rowid LIMIT 1`,
		name, now.Format(time.RFC3339),
	)
	var id, body string
	if err := row.Scan(&id, &body); err == sql.ErrNoRows {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nil)
		return
	} else if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	// set visibility timeout: hide for 30s
	visibleAfter := now.Add(30 * time.Second)
	db.Exec(`UPDATE messages SET visible_after = ? WHERE id = ?`,
		visibleAfter.Format(time.RFC3339), id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id, "body": body})
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
