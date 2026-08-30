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

// Instance is a compute instance — backed by a real systemd-nspawn container on the agent.
type Instance struct {
	ID           string    `json:"id"`
	NodeID       string    `json:"node_id"`
	Status       string    `json:"status"`
	InstanceType string    `json:"instance_type"`
	CPULimit     string    `json:"cpu_limit"`
	MemLimitMB   int       `json:"mem_limit_mb"`
	BaseImage    string    `json:"base_image,omitempty"` // path to rootfs base on agent machine
	CreatedAt    time.Time `json:"created_at"`
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
			id            TEXT PRIMARY KEY,
			node_id       TEXT NOT NULL,
			status        TEXT NOT NULL,
			instance_type TEXT NOT NULL DEFAULT 'small',
			cpu_limit     TEXT NOT NULL DEFAULT '50%',
			mem_limit_mb  INTEGER NOT NULL DEFAULT 512,
			base_image    TEXT NOT NULL DEFAULT '',
			created_at    TEXT NOT NULL
		);
	`)
	if err != nil {
		log.Fatal(err)
	}
	_, _ = db.Exec(`ALTER TABLE instances ADD COLUMN instance_type TEXT NOT NULL DEFAULT 'small'`)
	_, _ = db.Exec(`ALTER TABLE instances ADD COLUMN cpu_limit TEXT NOT NULL DEFAULT '50%'`)
	_, _ = db.Exec(`ALTER TABLE instances ADD COLUMN mem_limit_mb INTEGER NOT NULL DEFAULT 512`)
	_, _ = db.Exec(`ALTER TABLE instances ADD COLUMN base_image TEXT NOT NULL DEFAULT ''`)

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
		`INSERT INTO instances (id, node_id, status, instance_type, cpu_limit, mem_limit_mb, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   node_id = excluded.node_id,
		   status = excluded.status,
		   instance_type = excluded.instance_type,
		   cpu_limit = excluded.cpu_limit,
		   mem_limit_mb = excluded.mem_limit_mb,
		   created_at = excluded.created_at`,
		inst.ID, inst.NodeID, inst.Status, inst.InstanceType, inst.CPULimit, inst.MemLimitMB,
		inst.CreatedAt.Format(time.RFC3339),
	)
	return err
}

// Loads all instances from DB.
func (s *InstanceStore) LoadAll() ([]Instance, error) {
	rows, err := s.db.Query(`SELECT id, node_id, status, instance_type, cpu_limit, mem_limit_mb, created_at FROM instances`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Instance{}
	for rows.Next() {
		var inst Instance
		var created string
		if err := rows.Scan(&inst.ID, &inst.NodeID, &inst.Status, &inst.InstanceType, &inst.CPULimit, &inst.MemLimitMB, &created); err != nil {
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

// instanceTypeSpec maps instance_type names to cpu/mem limits.
var instanceTypeSpec = map[string][2]interface{}{
	// [cpu_quota_percent, mem_mb]
	"nano":   {"25%", 128},
	"micro":  {"50%", 256},
	"small":  {"100%", 512},
	"medium": {"200%", 1024},
	"large":  {"400%", 2048},
}

// Creates a new running instance on the given compute node.
func (s *InstanceStore) Create(nodeID, instanceType string) Instance {
	if instanceType == "" {
		instanceType = "small"
	}
	spec, ok := instanceTypeSpec[instanceType]
	if !ok {
		spec = instanceTypeSpec["small"]
	}
	seq := atomic.AddUint64(&s.seq, 1)
	inst := Instance{
		ID:           fmt.Sprintf("i-%d", seq),
		NodeID:       nodeID,
		Status:       "provisioning",
		InstanceType: instanceType,
		CPULimit:     spec[0].(string),
		MemLimitMB:   spec[1].(int),
		CreatedAt:    time.Now().UTC(),
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

// SetStatus updates instance status by ID.
func (s *InstanceStore) SetStatus(id, status string) error {
	_, err := s.db.Exec(`UPDATE instances SET status = ? WHERE id = ?`, status, id)
	return err
}

// POST /instances (launch) or GET /instances (list).
func handleInstances(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listInstances(w, r)
	case http.MethodPost:
		launchInstance(w, r)
	}
}

// PATCH /instances/{id} — agent calls this to update status (running, failed).
func handleInstancePatch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Status == "" {
		http.Error(w, "status required", http.StatusBadRequest)
		return
	}
	if err := instanceStore.SetStatus(id, req.Status); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	instancesMu.Lock()
	for i, inst := range instances {
		if inst.ID == id {
			instances[i].Status = req.Status
			break
		}
	}
	instancesMu.Unlock()
	log.Printf("PATCH /instances/%s - status=%s", id, req.Status)
	w.WriteHeader(http.StatusNoContent)
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
func launchInstance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InstanceType string `json:"instance_type"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}

	node, err := pickHealthyComputeNodeLocal()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	inst := instanceStore.Create(node.ID, req.InstanceType)

	instancesMu.Lock()
	instances = append(instances, inst)
	instancesMu.Unlock()

	log.Printf("POST /instances - provisioning %s (%s) on node %s", inst.ID, inst.InstanceType, inst.NodeID)

	// Tell the agent to provision the real container in the background.
	// Agent will PATCH status=running when done (or failed).
	go provisionOnAgent(*node, inst)

	// D6: notify SNS on instance launch
	if snsURL := os.Getenv("SNS_URL"); snsURL != "" {
		go snsPublish(snsURL, "instance-launch", fmt.Sprintf(`{"instance_id":"%s","node_id":"%s"}`, inst.ID, inst.NodeID))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(inst)
}

// provisionOnAgent calls the agent's /instances/{id}/provision endpoint.
// Agent provisions the nspawn container and PATCHes status back when ready.
func provisionOnAgent(node Node, inst Instance) {
	// derive agent addr — prefer node.Addr, fall back to hostname
	host := node.Addr
	if host == "" {
		host = node.Hostname
	}
	agentPort := os.Getenv("AGENT_HTTP_PORT")
	if agentPort == "" {
		agentPort = "8080"
	}
	url := fmt.Sprintf("http://%s:%s/instances/%s/provision", host, agentPort, inst.ID)
	payload, _ := json.Marshal(inst)
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Printf("provision agent call failed for %s: %v", inst.ID, err)
		// mark failed so user knows
		instanceStore.SetStatus(inst.ID, "failed")
		instancesMu.Lock()
		for i, v := range instances {
			if v.ID == inst.ID {
				instances[i].Status = "failed"
				break
			}
		}
		instancesMu.Unlock()
		return
	}
	resp.Body.Close()
	log.Printf("provision request sent for %s", inst.ID)
}

// Sets instance status to terminated and asks agent to destroy the container.
func terminateInstance(w http.ResponseWriter, id string) {
	// find the instance to get node info
	instancesMu.RLock()
	var inst *Instance
	for i := range instances {
		if instances[i].ID == id {
			inst = &instances[i]
			break
		}
	}
	instancesMu.RUnlock()

	if err := instanceStore.Terminate(id); err != nil {
		http.Error(w, "terminate failed", http.StatusInternalServerError)
		return
	}

	instancesMu.Lock()
	for i, v := range instances {
		if v.ID == id {
			instances[i].Status = "terminated"
			break
		}
	}
	instancesMu.Unlock()

	// tell agent to destroy the container
	if inst != nil {
		nodesMu.RLock()
		node, ok := nodes[inst.NodeID]
		nodesMu.RUnlock()
		if ok {
			go destroyOnAgent(node, id)
		}
	}

	log.Printf("DELETE /instances/%s - terminated", id)
	w.WriteHeader(http.StatusNoContent)
}

// destroyOnAgent calls DELETE /instances/{id} on the agent.
func destroyOnAgent(node Node, instanceID string) {
	host := node.Addr
	if host == "" {
		host = node.Hostname
	}
	agentPort := os.Getenv("AGENT_HTTP_PORT")
	if agentPort == "" {
		agentPort = "8080"
	}
	url := fmt.Sprintf("http://%s:%s/instances/%s", host, agentPort, instanceID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		log.Printf("destroy agent call failed for %s: %v", instanceID, err)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("destroy agent call failed for %s: %v", instanceID, err)
		return
	}
	resp.Body.Close()
	log.Printf("destroy request sent for %s", instanceID)
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
