package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type Node struct {
	ID     string `json:"id"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

type JobRequest struct {
	Command string `json:"command"`
}

type Job struct {
	ID         string     `json:"job_id"`
	NodeID     string     `json:"node_id"`
	Command    string     `json:"command"`
	Status     string     `json:"status"`
	ExitCode   *int       `json:"exit_code,omitempty"`
	Stdout     string     `json:"stdout,omitempty"`
	Stderr     string     `json:"stderr,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

type JobUpdateRequest struct {
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code,omitempty"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
}

var (
	counter  uint64
	jobs     = make(map[string]Job)
	jobsMu   sync.RWMutex
	jobSeq   uint64
	jobStore *JobStore
)

func main() {
	registryURL := getenv("REGISTRY_URL", "http://127.0.0.1:9000")
	dbPath := getenv("SCHEDULER_DB", "scheduler.db")

	jobStore = NewJobStore(dbPath)	

	loaded, maxSeq, err := jobStore.LoadAll()
	if err != nil {
		log.Fatal(err)
	}
	jobs = loaded
	jobSeq = maxSeq

	http.HandleFunc("GET /health", handleHealth)
	http.HandleFunc("GET /schedule", func(w http.ResponseWriter, r *http.Request) {
		handleSchedule(w, r, registryURL)
	})
	http.HandleFunc("GET /jobs/{id}", handleJobByID)
	http.HandleFunc("PATCH /jobs/{id}", handleJobByID)
	http.HandleFunc("GET /jobs", func(w http.ResponseWriter, r *http.Request) {
		handleJobs(w, r, registryURL)
	})
	http.HandleFunc("POST /jobs", func(w http.ResponseWriter, r *http.Request) {
		handleJobs(w, r, registryURL)
	})

	log.Println("scheduler listening on :9001")
	log.Fatal(http.ListenAndServe(":9001", nil))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"service": "scheduler",
	})
}

func handleSchedule(w http.ResponseWriter, r *http.Request, registryURL string) {
	node, err := pickHealthyComputeNode(registryURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"node_id": node.ID,
	})
}

func handleJobs(w http.ResponseWriter, r *http.Request, registryURL string) {
	switch r.Method {
	case http.MethodPost:
		submitJob(w, r, registryURL)
	case http.MethodGet:
		listJobs(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleJobByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "job id required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		getJob(w, id)
	case http.MethodPatch:
		updateJob(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func submitJob(w http.ResponseWriter, r *http.Request, registryURL string) {
	var req JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "failed to decode job", http.StatusBadRequest)
		return
	}

	if req.Command == "" {
		http.Error(w, "command is required", http.StatusBadRequest)
		return
	}

	node, err := pickHealthyComputeNode(registryURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	seq := atomic.AddUint64(&jobSeq, 1)
	job := Job{
		ID:        fmt.Sprintf("job-%d", seq),
		NodeID:    node.ID,
		Command:   req.Command,
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	}

	jobsMu.Lock()
	jobs[job.ID] = job
	if err := jobStore.Save(job); err != nil {
		log.Printf("failed to persist job %s: %v", job.ID, err)
	}
	jobsMu.Unlock()

	log.Printf("POST /jobs - job %s assigned to node %s", job.ID, job.NodeID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

func listJobs(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	status := r.URL.Query().Get("status")

	jobsMu.RLock()
	defer jobsMu.RUnlock()

	result := make([]Job, 0)
	for _, job := range jobs {
		if nodeID != "" && job.NodeID != nodeID {
			continue
		}
		if status != "" && job.Status != status {
			continue
		}
		result = append(result, job)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func getJob(w http.ResponseWriter, id string) {
	jobsMu.RLock()
	job, ok := jobs[id]
	jobsMu.RUnlock()

	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func updateJob(w http.ResponseWriter, r *http.Request, id string) {
	var req JobUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "failed to decode update", http.StatusBadRequest)
		return
	}

	allowed := map[string]bool{
		"running": true,
		"done":    true,
		"failed":  true,
	}
	if !allowed[req.Status] {
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}

	jobsMu.Lock()
	job, ok := jobs[id]
	if !ok {
		jobsMu.Unlock()
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	job.Status = req.Status
	if req.ExitCode != nil {
		job.ExitCode = req.ExitCode
	}
	if req.Stdout != "" {
		job.Stdout = req.Stdout
	}
	if req.Stderr != "" {
		job.Stderr = req.Stderr
	}
	if req.Status == "done" || req.Status == "failed" {
		now := time.Now().UTC()
		job.FinishedAt = &now
	}
	jobs[job.ID] = job
	if err := jobStore.Save(job); err != nil {
		log.Printf("failed to persist job %s: %v", job.ID, err)
	}
	jobsMu.Unlock()

	log.Printf("PATCH /jobs/%s - status=%s", id, req.Status)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func pickHealthyComputeNode(registryURL string) (*Node, error) {
	resp, err := http.Get(registryURL + "/nodes?role=compute")
	if err != nil {
		return nil, fmt.Errorf("failed to reach registry")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read registry response")
	}

	var nodes map[string]Node
	if err := json.Unmarshal(body, &nodes); err != nil {
		return nil, fmt.Errorf("failed to decode registry response")
	}

	healthy := make([]Node, 0)
	for _, node := range nodes {
		if node.Status == "healthy" {
			healthy = append(healthy, node)
		}
	}

	if len(healthy) == 0 {
		return nil, fmt.Errorf("no healthy compute nodes")
	}

	idx := atomic.AddUint64(&counter, 1) % uint64(len(healthy))
	selected := healthy[idx]
	return &selected, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}