package main

import (
	"bytes"
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

// Subscription is an HTTP endpoint subscribed to a topic.
type Subscription struct {
	ID       string `json:"id"`
	Topic    string `json:"topic"`
	Endpoint string `json:"endpoint"`
}

var (
	db     *sql.DB
	subSeq uint64
)

func main() {
	dbPath := getenv("SNS_DB", "sns.db")
	listenAddr := getenv("SNS_ADDR", ":9004")

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS topics (
			name TEXT PRIMARY KEY,
			created_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS subscriptions (
			id       TEXT PRIMARY KEY,
			topic    TEXT NOT NULL,
			endpoint TEXT NOT NULL
		);
	`)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("GET /health", handleHealth)
	http.HandleFunc("POST /topics/{name}", handleCreateTopic)
	http.HandleFunc("GET /topics", handleListTopics)
	http.HandleFunc("POST /topics/{name}/subscribe", handleSubscribe)
	http.HandleFunc("POST /topics/{name}/publish", handlePublish)

	log.Printf("sns listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, `{"status":"healthy","service":"sns"}`)
}

// POST /topics/{name} — create topic.
func handleCreateTopic(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	db.Exec(`INSERT INTO topics (name, created_at) VALUES (?, ?) ON CONFLICT(name) DO NOTHING`,
		name, time.Now().UTC().Format(time.RFC3339))
	log.Printf("POST /topics/%s - created", name)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"name": name})
}

// GET /topics — list topics.
func handleListTopics(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query(`SELECT name FROM topics`)
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

// POST /topics/{name}/subscribe — subscribe an HTTP endpoint to a topic.
func handleSubscribe(w http.ResponseWriter, r *http.Request) {
	topic := r.PathValue("name")
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Endpoint == "" {
		http.Error(w, "endpoint required", http.StatusBadRequest)
		return
	}
	seq := atomic.AddUint64(&subSeq, 1)
	id := fmt.Sprintf("sub-%d", seq)
	db.Exec(`INSERT INTO subscriptions (id, topic, endpoint) VALUES (?, ?, ?)`, id, topic, req.Endpoint)
	log.Printf("POST /topics/%s/subscribe - %s -> %s", topic, id, req.Endpoint)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// POST /topics/{name}/publish — fan out message to all subscribers.
func handlePublish(w http.ResponseWriter, r *http.Request) {
	topic := r.PathValue("name")
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Message == "" {
		http.Error(w, "message required", http.StatusBadRequest)
		return
	}

	rows, err := db.Query(`SELECT endpoint FROM subscriptions WHERE topic = ?`, topic)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var endpoints []string
	for rows.Next() {
		var ep string
		rows.Scan(&ep)
		endpoints = append(endpoints, ep)
	}

	payload, _ := json.Marshal(map[string]string{"topic": topic, "message": req.Message})
	for _, ep := range endpoints {
		go func(ep string) {
			if _, err := http.Post(ep, "application/json", bytes.NewReader(payload)); err != nil {
				log.Printf("sns: delivery to %s failed: %v", ep, err)
			}
		}(ep)
	}

	log.Printf("POST /topics/%s/publish - delivered to %d subscribers", topic, len(endpoints))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"delivered": len(endpoints)})
}

// PublishEvent is called by registry/scheduler hooks via HTTP if SNS_URL is set.
// POST SNS_URL/topics/{topic}/publish with {"message":"..."}
func PublishEvent(snsURL, topic, message string) {
	payload, _ := json.Marshal(map[string]string{"message": message})
	resp, err := http.Post(
		fmt.Sprintf("%s/topics/%s/publish", snsURL, topic),
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		log.Printf("sns publish error: %v", err)
		return
	}
	resp.Body.Close()
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
