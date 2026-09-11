package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func TestListJobs_ConcurrencyCap(t *testing.T) {
	setupSchedulerTest(t)
	t.Setenv("MAX_JOBS_PER_NODE", "1")

	// one running job on n1
	jobs["job-1"] = Job{ID: "job-1", NodeID: "n1", Status: "running", CreatedAt: time.Now().UTC()}
	// one pending job on n1
	jobs["job-2"] = Job{ID: "job-2", NodeID: "n1", Status: "pending", CreatedAt: time.Now().UTC()}

	req := httptest.NewRequest("GET", "/jobs?node_id=n1&status=pending", nil)
	w := httptest.NewRecorder()
	listJobs(w, req)

	var got []Job
	json.Unmarshal(w.Body.Bytes(), &got)
	// max 1 running, so pending returns empty (cap reached)
	if len(got) != 0 {
		t.Errorf("expected 0 pending (cap hit), got %d", len(got))
	}
}
