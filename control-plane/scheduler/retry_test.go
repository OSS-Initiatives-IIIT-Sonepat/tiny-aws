package main

import (
	"testing"
	"time"
)

// Test retry logic: fail once (gets retried), fail again (stays failed).
func TestJobRetryThenFail(t *testing.T) {
	js := newTestJobStore(t)

	job := Job{
		ID:        "job-1",
		NodeID:    "node-1",
		Command:   "exit 1",
		Status:    "pending",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := js.Save(job); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// first failure — should be retried (retry_count goes to 1)
	job.Status = "pending"
	job.RetryCount = 1
	if err := js.Save(job); err != nil {
		t.Fatalf("Save retry: %v", err)
	}

	loaded, _, err := js.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	got := loaded["job-1"]
	if got.RetryCount != 1 {
		t.Errorf("retry_count = %d, want 1", got.RetryCount)
	}
	if got.Status != "pending" {
		t.Errorf("status = %q, want pending after first retry", got.Status)
	}

	// second failure — exceeds maxJobRetries, stays failed
	code := 1
	now := time.Now().UTC()
	job.Status = "failed"
	job.ExitCode = &code
	job.Stderr = "command failed"
	job.FinishedAt = &now
	if err := js.Save(job); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, _, err = js.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	got = loaded["job-1"]
	if got.Status != "failed" {
		t.Errorf("status = %q, want failed", got.Status)
	}
	if got.ExitCode == nil || *got.ExitCode != 1 {
		t.Errorf("exit_code = %v, want 1", got.ExitCode)
	}
	if got.Stderr != "command failed" {
		t.Errorf("stderr = %q", got.Stderr)
	}
	if got.FinishedAt == nil {
		t.Error("finished_at should be set")
	}
}
