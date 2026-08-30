package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Fake EC2 instance record.
type Instance struct {
	ID        string    `json:"id"`
	NodeID    string    `json:"node_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type InstanceStore struct {
	db  *sql.DB
	seq uint64
}

var (
	instanceStore *InstanceStore
	instances     []Instance
	instancesMu   sync.RWMutex
)

// Opens instances table on the shared registry database.
func NewInstanceStore(db *sql.DB) *InstanceStore {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS instances (
			id         TEXT PRIMARY KEY,
			node_id    TEXT NOT NULL,
			status     TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
	`)
	if err != nil {
		log.Fatal(err)
	}

	// Load max sequence so IDs don't reset on restart.
	var maxSeq uint64
	var raw sql.NullString
	_ = db.QueryRow(`SELECT id FROM instances ORDER BY rowid DESC LIMIT 1`).Scan(&raw)
	if raw.Valid {
		var n uint64
		fmt.Sscanf(raw.String, "i-%d", &n)
		maxSeq = n
	}

	return &InstanceStore{db: db, seq: maxSeq}
}

// Saves instance row to SQLite.
func (s *InstanceStore) Save(inst Instance) error {
	_, err := s.db.Exec(
		`INSERT INTO instances (id, node_id, status, created_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   node_id = excluded.node_id,
		   status = excluded.status,
		   created_at = excluded.created_at`,
		inst.ID, inst.NodeID, inst.Status, inst.CreatedAt.Format(time.RFC3339),
	)
	return err
}

// Loads all instances from DB.
func (s *InstanceStore) LoadAll() ([]Instance, error) {
	rows, err := s.db.Query(`SELECT id, node_id, status, created_at FROM instances`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Instance{}
	for rows.Next() {
		var inst Instance
		var created string
		if err := rows.Scan(&inst.ID, &inst.NodeID, &inst.Status, &created); err != nil {
			return nil, err
		}
		inst.CreatedAt, err = time.Parse(time.RFC3339, created)
		if err != nil {
			return nil, err
		}
		out = append(out, inst)
	}
	return out, rows.Err()
}

// Creates a new running instance on the given compute node.
func (s *InstanceStore) Create(nodeID string) Instance {
	seq := atomic.AddUint64(&s.seq, 1)
	inst := Instance{
		ID:        fmt.Sprintf("i-%d", seq),
		NodeID:    nodeID,
		Status:    "running",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.Save(inst); err != nil {
		log.Fatal(err)
	}
	return inst
}

// Marks instance as terminated.
func (s *InstanceStore) Terminate(id string) error {
	_, err := s.db.Exec(`UPDATE instances SET status = 'terminated' WHERE id = ?`, id)
	return err
}

// POST /instances (launch) or GET /instances (list).
func handleInstances(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listInstances(w, r)
	case http.MethodPost:
		launchInstance(w)
	}
}

// DELETE /instances/{id} — terminate instance.
func handleInstanceByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "instance id required", http.StatusBadRequest)
		return
	}
	terminateInstance(w, id)
}

// GET /instances/{id} — fetch one instance.
func handleInstanceGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "instance id required", http.StatusBadRequest)
		return
	}

	instancesMu.RLock()
	defer instancesMu.RUnlock()

	for _, inst := range instances {
		if inst.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(inst)
			return
		}
	}

	http.Error(w, "instance not found", http.StatusNotFound)
}

// Returns instances as JSON array (?node_id= and ?status= filters).
func listInstances(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	status := r.URL.Query().Get("status")

	instancesMu.RLock()
	defer instancesMu.RUnlock()

	filtered := make([]Instance, 0)
	for _, inst := range instances {
		if nodeID != "" && inst.NodeID != nodeID {
			continue
		}
		if status != "" && inst.Status != status {
			continue
		}
		filtered = append(filtered, inst)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

// Picks healthy compute node and creates a running instance.
func launchInstance(w http.ResponseWriter) {
	node, err := pickHealthyComputeNodeLocal()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	inst := instanceStore.Create(node.ID)

	instancesMu.Lock()
	instances = append(instances, inst)
	instancesMu.Unlock()

	log.Printf("POST /instances - launched %s on node %s", inst.ID, inst.NodeID)

	// D6: notify SNS on instance launch
	if snsURL := os.Getenv("SNS_URL"); snsURL != "" {
		go snsPublish(snsURL, "instance-launch", fmt.Sprintf(`{"instance_id":"%s","node_id":"%s"}`, inst.ID, inst.NodeID))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(inst)
}

// Sets instance status to terminated.
func terminateInstance(w http.ResponseWriter, id string) {
	if err := instanceStore.Terminate(id); err != nil {
		http.Error(w, "terminate failed", http.StatusInternalServerError)
		return
	}

	instancesMu.Lock()
	for i, inst := range instances {
		if inst.ID == id {
			instances[i].Status = "terminated"
			break
		}
	}
	instancesMu.Unlock()

	log.Printf("DELETE /instances/%s - terminated", id)
	w.WriteHeader(http.StatusNoContent)
}

// Picks first healthy compute node from registry.
func pickHealthyComputeNodeLocal() (*Node, error) {
	nodesMu.RLock()
	defer nodesMu.RUnlock()

	healthy := []Node{}
	for _, n := range nodes {
		if n.Role == "compute" && n.Status == "healthy" {
			healthy = append(healthy, n)
		}
	}
	if len(healthy) == 0 {
		return nil, fmt.Errorf("no healthy compute nodes")
	}
	return &healthy[0], nil
}

// snsPublish fires a best-effort publish to an SNS topic.
func snsPublish(snsURL, topic, message string) {
	payload := fmt.Sprintf(`{"message":%q}`, message)
	resp, err := http.Post(
		fmt.Sprintf("%s/topics/%s/publish", snsURL, topic),
		"application/json",
		bytes.NewBufferString(payload),
	)
	if err != nil {
		log.Printf("sns publish error: %v", err)
		return
	}
	resp.Body.Close()
}
