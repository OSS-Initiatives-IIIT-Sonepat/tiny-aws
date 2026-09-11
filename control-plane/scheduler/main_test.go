package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func setupSchedulerTest(t *testing.T) {
	t.Helper()
	t.Setenv("TINYAWS_API_KEY", "")
	jobStore = NewJobStore(":memory:")
	jobs = make(map[string]Job)
	jobSeq = 0
}

func TestSchedulerHealth(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["service"] != "scheduler" {
		t.Errorf("service = %q", body["service"])
	}
}

func TestGetJob(t *testing.T) {
	setupSchedulerTest(t)

	jobs["job-1"] = Job{
		ID: "job-1", NodeID: "node-1", Command: "echo hi",
		Status: "done", CreatedAt: time.Now().UTC(),
	}

	w := httptest.NewRecorder()
	getJob(w, "job-1")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got Job
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.Command != "echo hi" {
		t.Errorf("command = %q", got.Command)
	}
}

func TestGetJob_NotFound(t *testing.T) {
	setupSchedulerTest(t)

	w := httptest.NewRecorder()
	getJob(w, "missing")

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestListJobs_All(t *testing.T) {
	setupSchedulerTest(t)

	jobs["job-1"] = Job{ID: "job-1", NodeID: "n1", Status: "pending", CreatedAt: time.Now().UTC()}
	jobs["job-2"] = Job{ID: "job-2", NodeID: "n2", Status: "running", CreatedAt: time.Now().UTC()}

	req := httptest.NewRequest("GET", "/jobs", nil)
	w := httptest.NewRecorder()
	listJobs(w, req)

	var got []Job
	json.Unmarshal(w.Body.Bytes(), &got)
	if len(got) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(got))
	}
}

func TestListJobs_FilterByNodeAndStatus(t *testing.T) {
	setupSchedulerTest(t)
	t.Setenv("MAX_JOBS_PER_NODE", "5") // allow enough concurrency

	jobs["job-1"] = Job{ID: "job-1", NodeID: "n1", Status: "pending", CreatedAt: time.Now().UTC()}
	jobs["job-2"] = Job{ID: "job-2", NodeID: "n2", Status: "running", CreatedAt: time.Now().UTC()}
	jobs["job-3"] = Job{ID: "job-3", NodeID: "n1", Status: "running", CreatedAt: time.Now().UTC()}

	req := httptest.NewRequest("GET", "/jobs?node_id=n1&status=pending", nil)
	w := httptest.NewRecorder()
	listJobs(w, req)

	var got []Job
	json.Unmarshal(w.Body.Bytes(), &got)
	if len(got) != 1 {
		t.Errorf("expected 1 job, got %d", len(got))
	}
}

func TestUpdateJob_Running(t *testing.T) {
	setupSchedulerTest(t)

	jobs["job-1"] = Job{
		ID: "job-1", NodeID: "n1", Command: "echo",
		Status: "pending", CreatedAt: time.Now().UTC(),
	}

	body, _ := json.Marshal(JobUpdateRequest{Status: "running"})
	req := httptest.NewRequest("PATCH", "/jobs/job-1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	updateJob(w, req, "job-1")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if jobs["job-1"].Status != "running" {
		t.Errorf("status = %q, want running", jobs["job-1"].Status)
	}
	if jobs["job-1"].RunningAt == nil {
		t.Error("RunningAt should be set")
	}
}

func TestUpdateJob_Done(t *testing.T) {
	setupSchedulerTest(t)

	jobs["job-1"] = Job{
		ID: "job-1", NodeID: "n1", Command: "echo",
		Status: "running", CreatedAt: time.Now().UTC(),
	}

	code := 0
	body, _ := json.Marshal(JobUpdateRequest{Status: "done", ExitCode: &code, Stdout: "hello\n"})
	req := httptest.NewRequest("PATCH", "/jobs/job-1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	updateJob(w, req, "job-1")

	if jobs["job-1"].Status != "done" {
		t.Errorf("status = %q", jobs["job-1"].Status)
	}
	if jobs["job-1"].Stdout != "hello\n" {
		t.Errorf("stdout = %q", jobs["job-1"].Stdout)
	}
	if jobs["job-1"].FinishedAt == nil {
		t.Error("FinishedAt should be set")
	}
}

func TestUpdateJob_RetryOnFirstFailure(t *testing.T) {
	setupSchedulerTest(t)

	jobs["job-1"] = Job{
		ID: "job-1", NodeID: "n1", Command: "echo",
		Status: "running", RetryCount: 0, CreatedAt: time.Now().UTC(),
	}

	body, _ := json.Marshal(JobUpdateRequest{Status: "failed", Stderr: "oops"})
	req := httptest.NewRequest("PATCH", "/jobs/job-1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	updateJob(w, req, "job-1")

	job := jobs["job-1"]
	if job.Status != "pending" {
		t.Errorf("status = %q, want pending (retry)", job.Status)
	}
	if job.RetryCount != 1 {
		t.Errorf("retry_count = %d, want 1", job.RetryCount)
	}
}

func TestUpdateJob_FailAfterMaxRetries(t *testing.T) {
	setupSchedulerTest(t)

	jobs["job-1"] = Job{
		ID: "job-1", NodeID: "n1", Command: "echo",
		Status: "running", RetryCount: 1, CreatedAt: time.Now().UTC(),
	}

	body, _ := json.Marshal(JobUpdateRequest{Status: "failed"})
	req := httptest.NewRequest("PATCH", "/jobs/job-1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	updateJob(w, req, "job-1")

	if jobs["job-1"].Status != "failed" {
		t.Errorf("status = %q, want failed", jobs["job-1"].Status)
	}
}

func TestUpdateJob_InvalidStatus(t *testing.T) {
	setupSchedulerTest(t)

	jobs["job-1"] = Job{ID: "job-1", NodeID: "n1", Status: "pending", CreatedAt: time.Now().UTC()}

	body, _ := json.Marshal(JobUpdateRequest{Status: "bogus"})
	req := httptest.NewRequest("PATCH", "/jobs/job-1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	updateJob(w, req, "job-1")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestMaxJobsPerNode_Default(t *testing.T) {
	t.Setenv("MAX_JOBS_PER_NODE", "")
	if got := maxJobsPerNode(); got != 1 {
		t.Errorf("default = %d, want 1", got)
	}
}

func TestMaxJobsPerNode_Custom(t *testing.T) {
	t.Setenv("MAX_JOBS_PER_NODE", "5")
	if got := maxJobsPerNode(); got != 5 {
		t.Errorf("got = %d, want 5", got)
	}
}

func TestJobTimeout_Default(t *testing.T) {
	t.Setenv("JOB_TIMEOUT_SECS", "")
	if got := jobTimeout(); got != 3600*time.Second {
		t.Errorf("default = %v, want 3600s", got)
	}
}

func TestJobTimeout_Custom(t *testing.T) {
	t.Setenv("JOB_TIMEOUT_SECS", "120")
	if got := jobTimeout(); got != 120*time.Second {
		t.Errorf("got = %v, want 120s", got)
	}
}

func TestParseJobSeq(t *testing.T) {
	tests := []struct {
		id   string
		want uint64
		ok   bool
	}{
		{"job-1", 1, true},
		{"job-99", 99, true},
		{"not-a-job", 0, false},
		{"job-", 0, false},
	}
	for _, tt := range tests {
		got, ok := parseJobSeq(tt.id)
		if ok != tt.ok || got != tt.want {
			t.Errorf("parseJobSeq(%q) = %d, %v; want %d, %v", tt.id, got, ok, tt.want, tt.ok)
		}
	}
}
