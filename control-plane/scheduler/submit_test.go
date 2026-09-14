package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubmitJob_Basic(t *testing.T) {
	setupSchedulerTest(t)

	// Mock registry returning one healthy compute node.
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]Node{
			"node-1": {ID: "node-1", Role: "compute", Status: "healthy"},
		})
	}))
	defer registry.Close()

	body, _ := json.Marshal(JobRequest{Command: "echo hello"})
	req := httptest.NewRequest("POST", "/jobs", bytes.NewReader(body))
	w := httptest.NewRecorder()
	submitJob(w, req, registry.URL)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}

	var got Job
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.ID != "job-1" {
		t.Errorf("id = %q, want 'job-1'", got.ID)
	}
	if got.NodeID != "node-1" {
		t.Errorf("node_id = %q, want 'node-1'", got.NodeID)
	}
	if got.Command != "echo hello" {
		t.Errorf("command = %q", got.Command)
	}
	if got.Status != "pending" {
		t.Errorf("status = %q, want 'pending'", got.Status)
	}
}

func TestSubmitJob_ServiceType(t *testing.T) {
	setupSchedulerTest(t)

	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]Node{
			"node-1": {ID: "node-1", Role: "compute", Status: "healthy"},
		})
	}))
	defer registry.Close()

	body, _ := json.Marshal(JobRequest{
		Command: "python server.py", JobType: "service", Port: 8080,
	})
	req := httptest.NewRequest("POST", "/jobs", bytes.NewReader(body))
	w := httptest.NewRecorder()
	submitJob(w, req, registry.URL)

	var got Job
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.JobType != "service" {
		t.Errorf("job_type = %q, want 'service'", got.JobType)
	}
	if got.Port != 8080 {
		t.Errorf("port = %d, want 8080", got.Port)
	}
}

func TestSubmitJob_EmptyCommand(t *testing.T) {
	setupSchedulerTest(t)

	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]Node{})
	}))
	defer registry.Close()

	body, _ := json.Marshal(JobRequest{})
	req := httptest.NewRequest("POST", "/jobs", bytes.NewReader(body))
	w := httptest.NewRecorder()
	submitJob(w, req, registry.URL)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestSubmitJob_NoHealthyNodes(t *testing.T) {
	setupSchedulerTest(t)

	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]Node{
			"node-1": {ID: "node-1", Role: "compute", Status: "unhealthy"},
		})
	}))
	defer registry.Close()

	body, _ := json.Marshal(JobRequest{Command: "echo hi"})
	req := httptest.NewRequest("POST", "/jobs", bytes.NewReader(body))
	w := httptest.NewRecorder()
	submitJob(w, req, registry.URL)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestSubmitJob_WithEnvVars(t *testing.T) {
	setupSchedulerTest(t)

	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]Node{
			"node-1": {ID: "node-1", Role: "compute", Status: "healthy"},
		})
	}))
	defer registry.Close()

	body, _ := json.Marshal(JobRequest{
		Command: "echo $FOO",
		EnvVars: map[string]string{"FOO": "bar"},
	})
	req := httptest.NewRequest("POST", "/jobs", bytes.NewReader(body))
	w := httptest.NewRecorder()
	submitJob(w, req, registry.URL)

	var got Job
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.EnvVars["FOO"] != "bar" {
		t.Errorf("env_vars = %v", got.EnvVars)
	}
}
