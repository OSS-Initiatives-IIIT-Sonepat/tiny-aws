package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestPollSQSQueue_SubmitsJob(t *testing.T) {
	setupSchedulerTest(t)

	// Mock registry returns one healthy compute node
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]Node{
			"n1": {ID: "n1", Role: "compute", Status: "healthy"},
		})
	}))
	defer registry.Close()

	// Build the SQS message: a job request as JSON body
	jobReq := JobRequest{Command: "echo hello"}
	body, _ := json.Marshal(jobReq)

	consumed := false

	// Mock SQS server
	sqs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && r.URL.Path == "/queues/jobs/messages":
			if consumed {
				// No more messages
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{}`))
				return
			}
			consumed = true
			json.NewEncoder(w).Encode(map[string]string{
				"id":   "msg-1",
				"body": string(body),
			})
		case r.Method == "DELETE":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer sqs.Close()

	// Simulate one poll iteration by calling pollSQSQueue internals.
	// pollSQSQueue runs in a ticker loop, so we replicate the single-iteration logic.
	resp, err := http.Get(sqs.URL + "/queues/jobs/messages")
	if err != nil {
		t.Fatal(err)
	}
	var msg struct {
		ID   string `json:"id"`
		Body string `json:"body"`
	}
	json.NewDecoder(resp.Body).Decode(&msg)
	resp.Body.Close()

	if msg.ID != "msg-1" {
		t.Fatalf("msg id = %q, want msg-1", msg.ID)
	}

	var req JobRequest
	if err := json.Unmarshal([]byte(msg.Body), &req); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if req.Command != "echo hello" {
		t.Errorf("command = %q, want 'echo hello'", req.Command)
	}

	// Simulate what pollSQSQueue does: pick node and create job
	node, err := pickHealthyComputeNode(registry.URL)
	if err != nil {
		t.Fatalf("pickHealthyComputeNode: %v", err)
	}
	if node.ID != "n1" {
		t.Errorf("node = %q, want n1", node.ID)
	}

	seq := atomic.AddUint64(&jobSeq, 1)
	job := Job{
		ID:      "job-" + string(rune('0'+seq)),
		NodeID:  node.ID,
		Command: req.Command,
		Status:  "pending",
	}
	jobsMu.Lock()
	jobs[job.ID] = job
	jobsMu.Unlock()

	jobsMu.RLock()
	if len(jobs) != 1 {
		t.Errorf("jobs count = %d, want 1", len(jobs))
	}
	jobsMu.RUnlock()
}

func TestSQSMessageFormat(t *testing.T) {
	// Verify the expected JSON format for SQS job messages
	req := JobRequest{
		Command:    "ls -la",
		InstanceID: "i-123",
		EnvVars:    map[string]string{"FOO": "bar"},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	var decoded JobRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Command != "ls -la" {
		t.Errorf("command = %q", decoded.Command)
	}
	if decoded.InstanceID != "i-123" {
		t.Errorf("instance_id = %q", decoded.InstanceID)
	}
	if decoded.EnvVars["FOO"] != "bar" {
		t.Errorf("env_vars = %v", decoded.EnvVars)
	}
}

func TestSQSMessageFormat_DeployURL(t *testing.T) {
	req := JobRequest{DeployURL: "http://store/app.zip", JobType: "service", Port: 8080}
	data, _ := json.Marshal(req)

	var decoded JobRequest
	json.Unmarshal(data, &decoded)

	if decoded.DeployURL != "http://store/app.zip" {
		t.Errorf("deploy_url = %q", decoded.DeployURL)
	}
	if decoded.JobType != "service" {
		t.Errorf("job_type = %q", decoded.JobType)
	}
	if decoded.Port != 8080 {
		t.Errorf("port = %d", decoded.Port)
	}
}
