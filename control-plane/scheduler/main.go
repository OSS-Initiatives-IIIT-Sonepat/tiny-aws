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
	ID        string    `json:"job_id"`
	NodeID    string    `json:"node_id"`
	Command   string    `json:"command"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	counter  uint64
	jobs     = make(map[string]Job)
	jobsMu   sync.RWMutex
	jobSeq   uint64
)

func main() {
	registryURL := getenv("REGISTRY_URL", "http://127.0.0.1:9000")

	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/schedule", func(w http.ResponseWriter, r *http.Request) {
		handleSchedule(w, r, registryURL)
	})
	http.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		handleJobs(w, r, registryURL)
	})

	log.Println("scheduler listening on :9001")
	log.Fatal(http.ListenAndServe(":9001", nil))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"service": "scheduler",
	})
}

func handleSchedule(w http.ResponseWriter, r *http.Request, registryURL string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
	jobsMu.Unlock()

	log.Printf("POST /jobs - job %s assigned to node %s", job.ID, job.NodeID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

func listJobs(w http.ResponseWriter, r *http.Request) {
	jobsMu.RLock()
	defer jobsMu.RUnlock()

	result := make([]Job, 0, len(jobs))
	for _, job := range jobs {
		result = append(result, job)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
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
