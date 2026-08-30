package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

// Service is a long-running process deployed on a compute node.
type Service struct {
	ID         string    `json:"id"`
	InstanceID string    `json:"instance_id"`
	NodeID     string    `json:"node_id"`
	Port       int       `json:"port"`
	PID        int       `json:"pid"`
	Status     string    `json:"status"` // running | stopped | crashed
	DeployURL  string    `json:"deploy_url"`
	CreatedAt  time.Time `json:"created_at"`
}

type ServiceStore struct {
	db  *sql.DB
	seq uint64
}

var serviceStore *ServiceStore

// Opens services table on the shared registry database.
func NewServiceStore(db *sql.DB) *ServiceStore {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS services (
			id          TEXT PRIMARY KEY,
			instance_id TEXT NOT NULL DEFAULT '',
			node_id     TEXT NOT NULL,
			port        INTEGER NOT NULL DEFAULT 0,
			pid         INTEGER NOT NULL DEFAULT 0,
			status      TEXT NOT NULL DEFAULT 'running',
			deploy_url  TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL
		);
	`)
	if err != nil {
		log.Fatal(err)
	}

	// load max seq so IDs don't reset on restart
	var raw sql.NullString
	_ = db.QueryRow(`SELECT id FROM services ORDER BY rowid DESC LIMIT 1`).Scan(&raw)
	var maxSeq uint64
	if raw.Valid {
		var n uint64
		fmt.Sscanf(raw.String, "svc-%d", &n)
		maxSeq = n
	}

	return &ServiceStore{db: db, seq: maxSeq}
}

// Save upserts a service record.
func (s *ServiceStore) Save(svc Service) error {
	_, err := s.db.Exec(
		`INSERT INTO services (id, instance_id, node_id, port, pid, status, deploy_url, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   instance_id = excluded.instance_id,
		   node_id     = excluded.node_id,
		   port        = excluded.port,
		   pid         = excluded.pid,
		   status      = excluded.status,
		   deploy_url  = excluded.deploy_url,
		   created_at  = excluded.created_at`,
		svc.ID, svc.InstanceID, svc.NodeID, svc.Port, svc.PID,
		svc.Status, svc.DeployURL, svc.CreatedAt.Format(time.RFC3339),
	)
	return err
}

// LoadAll returns all service records.
func (s *ServiceStore) LoadAll() ([]Service, error) {
	rows, err := s.db.Query(`SELECT id, instance_id, node_id, port, pid, status, deploy_url, created_at FROM services`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Service{}
	for rows.Next() {
		var svc Service
		var created string
		if err := rows.Scan(&svc.ID, &svc.InstanceID, &svc.NodeID, &svc.Port, &svc.PID, &svc.Status, &svc.DeployURL, &created); err != nil {
			return nil, err
		}
		svc.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, svc)
	}
	return out, rows.Err()
}

// Create inserts a new service and returns it.
func (s *ServiceStore) Create(nodeID, instanceID, deployURL string, port, pid int) Service {
	seq := atomic.AddUint64(&s.seq, 1)
	svc := Service{
		ID:         fmt.Sprintf("svc-%d", seq),
		InstanceID: instanceID,
		NodeID:     nodeID,
		Port:       port,
		PID:        pid,
		Status:     "running",
		DeployURL:  deployURL,
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.Save(svc); err != nil {
		log.Printf("failed to persist service %s: %v", svc.ID, err)
	}
	return svc
}

// UpdateStatus sets service status by ID.
func (s *ServiceStore) UpdateStatus(id, status string) error {
	_, err := s.db.Exec(`UPDATE services SET status = ? WHERE id = ?`, status, id)
	return err
}

// POST /services — register a running service (called by agent after spawn).
func handleServiceCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeID     string `json:"node_id"`
		InstanceID string `json:"instance_id"`
		Port       int    `json:"port"`
		PID        int    `json:"pid"`
		DeployURL  string `json:"deploy_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NodeID == "" {
		http.Error(w, "node_id required", http.StatusBadRequest)
		return
	}
	svc := serviceStore.Create(req.NodeID, req.InstanceID, req.DeployURL, req.Port, req.PID)
	log.Printf("POST /services - %s node=%s port=%d pid=%d", svc.ID, svc.NodeID, svc.Port, svc.PID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(svc)
}

// GET /services — list all services (?node_id= ?status= filters).
func handleServiceList(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	status := r.URL.Query().Get("status")

	svcs, err := serviceStore.LoadAll()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	out := []Service{}
	for _, svc := range svcs {
		if nodeID != "" && svc.NodeID != nodeID {
			continue
		}
		if status != "" && svc.Status != status {
			continue
		}
		out = append(out, svc)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// GET /services/{id} — get one service.
func handleServiceGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	svcs, _ := serviceStore.LoadAll()
	for _, svc := range svcs {
		if svc.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(svc)
			return
		}
	}
	http.Error(w, "not found", http.StatusNotFound)
}

// PATCH /services/{id} — update status (agent reports crash, stop).
func handleServicePatch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Status == "" {
		http.Error(w, "status required", http.StatusBadRequest)
		return
	}
	if err := serviceStore.UpdateStatus(id, req.Status); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	log.Printf("PATCH /services/%s - status=%s", id, req.Status)
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /services/{id} — mark stopped (agent should kill the PID).
func handleServiceDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := serviceStore.UpdateStatus(id, "stopped"); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	log.Printf("DELETE /services/%s - marked stopped", id)
	w.WriteHeader(http.StatusNoContent)
}
