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
	Command    string `json:"command"`
	InstanceID string `json:"instance_id,omitempty"`
	DeployURL  string `json:"deploy_url,omitempty"`
}

type Job struct {
	ID         string     `json:"job_id"`
	NodeID     string     `json:"node_id"`
	InstanceID string     `json:"instance_id,omitempty"`
	Command    string     `json:"command"`
	DeployURL  string     `json:"deploy_url,omitempty"`
	Status     string     `json:"status"`
	RetryCount int        `json:"retry_count"`
	ExitCode   *int       `json:"exit_code,omitempty"`
	Stdout     string     `json:"stdout,omitempty"`
	Stderr     string     `json:"stderr,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

type Instance struct {
	ID     string `json:"id"`
	NodeID string `json:"node_id"`
	Status string `json:"status"`
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

const (
	jobTimeout   = 60 * time.Second
	maxJobRetries = 1
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

	go watchJobTimeouts()

	http.HandleFunc("GET /health", handleHealth)
	http.HandleFunc("GET /schedule", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleSchedule(w, r, registryURL)
	}))
	http.HandleFunc("GET /jobs/{id}", authMiddleware(handleJobByID))
	http.HandleFunc("PATCH /jobs/{id}", authMiddleware(handleJobByID))
	http.HandleFunc("GET /jobs", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleJobs(w, r, registryURL)
	}))
	http.HandleFunc("POST /jobs", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleJobs(w, r, registryURL)
	}))

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

	if req.Command == "" && req.DeployURL == "" {
		http.Error(w, "command or deploy_url is required", http.StatusBadRequest)
		return
	}

	var node *Node
	var err error

	if req.InstanceID != "" {
		node, err = pickNodeForInstance(registryURL, req.InstanceID)
	} else {
		node, err = pickHealthyComputeNode(registryURL)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	seq := atomic.AddUint64(&jobSeq, 1)
	job := Job{
		ID:         fmt.Sprintf("job-%d", seq),
		NodeID:     node.ID,
		InstanceID: req.InstanceID,
		Command:    req.Command,
		DeployURL:  req.DeployURL,
		Status:     "pending",
		CreatedAt:  time.Now().UTC(),
	}

	jobsMu.Lock()
	jobs[job.ID] = job
	if err := jobStore.Save(job); err != nil {
		log.Printf("failed to persist job %s: %v", job.ID, err)
	}
	jobsMu.Unlock()

	log.Printf("POST /jobs - job %s assigned to node %s instance=%s", job.ID, job.NodeID, job.InstanceID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

func listJobs(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	status := r.URL.Query().Get("status")

	jobsMu.RLock()
	defer jobsMu.RUnlock()

	// Enforce per-node concurrency limit when agent polls for pending jobs.
	if nodeID != "" && status == "pending" {
		maxJobs := maxJobsPerNode()
		running := 0
		for _, job := range jobs {
			if job.NodeID == nodeID && job.Status == "running" {
				running++
			}
		}
		if running >= maxJobs {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]Job{})
			return
		}
	}

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

	if req.Status == "failed" && job.RetryCount < maxJobRetries {
		job.RetryCount++
		job.Status = "pending"
		job.ExitCode = nil
		job.Stdout = ""
		if req.Stderr != "" {
			job.Stderr = req.Stderr
		}
		job.FinishedAt = nil
		jobs[job.ID] = job
		if err := jobStore.Save(job); err != nil {
			log.Printf("failed to persist job %s: %v", job.ID, err)
		}
		jobsMu.Unlock()
		log.Printf("PATCH /jobs/%s - retry %d queued", id, job.RetryCount)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(job)
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

func fetchComputeNodes(registryURL string) ([]Node, error) {
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

	out := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, node)
	}
	return out, nil
}

func pickNodeForInstance(registryURL, instanceID string) (*Node, error) {
	resp, err := http.Get(registryURL + "/instances/" + instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to reach registry")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("instance not found")
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry error: %s", string(body))
	}

	var inst Instance
	if err := json.NewDecoder(resp.Body).Decode(&inst); err != nil {
		return nil, fmt.Errorf("failed to decode instance")
	}
	if inst.Status != "running" {
		return nil, fmt.Errorf("instance %s not running (status=%s)", inst.ID, inst.Status)
	}

	nodes, err := fetchComputeNodes(registryURL)
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		if node.ID == inst.NodeID && node.Status == "healthy" {
			return &node, nil
		}
	}

	return nil, fmt.Errorf("node %s for instance %s is not healthy", inst.NodeID, inst.ID)
}

func pickHealthyComputeNode(registryURL string) (*Node, error) {
	nodes, err := fetchComputeNodes(registryURL)
	if err != nil {
		return nil, err
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

// maxJobsPerNode returns MAX_JOBS_PER_NODE env (default 1).
func maxJobsPerNode() int {
	v := os.Getenv("MAX_JOBS_PER_NODE")
	if v == "" {
		return 1
	}
	n := 1
	fmt.Sscanf(v, "%d", &n)
	if n < 1 {
		n = 1
	}
	return n
}

// Marks running jobs as failed if they exceed jobTimeout.
func watchJobTimeouts() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		jobsMu.Lock()
		for id, job := range jobs {
			if job.Status != "running" {
				continue
			}
			if now.Sub(job.CreatedAt) <= jobTimeout {
				continue
			}

			code := -1
			job.Status = "failed"
			job.ExitCode = &code
			job.Stderr = "job timed out"
			finished := now.UTC()
			job.FinishedAt = &finished
			jobs[id] = job

			if err := jobStore.Save(job); err != nil {
				log.Printf("failed to persist timed-out job %s: %v", id, err)
			} else {
				log.Printf("job %s timed out", id)
			}
		}
		jobsMu.Unlock()
	}
}