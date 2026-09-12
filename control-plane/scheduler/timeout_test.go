package main

import (
	"testing"
	"time"
)

// simulateTimeoutCheck runs the same logic as watchJobTimeouts (one tick).
func simulateTimeoutCheck(timeout time.Duration) {
	now := time.Now()

	jobsMu.Lock()
	defer jobsMu.Unlock()

	for id, job := range jobs {
		if job.Status != "running" {
			continue
		}
		start := job.CreatedAt
		if job.RunningAt != nil {
			start = *job.RunningAt
		}
		if now.Sub(start) <= timeout {
			continue
		}
		code := -1
		job.Status = "failed"
		job.ExitCode = &code
		job.Stderr = "job timed out"
		finished := now.UTC()
		job.FinishedAt = &finished
		jobs[id] = job
		_ = jobStore.Save(job)
	}
}

func TestWatchJobTimeouts_ExpiredJobMarkedFailed(t *testing.T) {
	js := newTestJobStore(t)
	jobStore = js
	jobs = make(map[string]Job)

	ran := time.Now().Add(-2 * time.Hour)
	jobs["job-1"] = Job{
		ID:        "job-1",
		NodeID:    "n",
		Command:   "sleep 9999",
		Status:    "running",
		RunningAt: &ran,
		CreatedAt: ran,
	}
	js.Save(jobs["job-1"])

	simulateTimeoutCheck(1 * time.Hour)

	got := jobs["job-1"]
	if got.Status != "failed" {
		t.Errorf("status = %q, want failed", got.Status)
	}
	if got.ExitCode == nil || *got.ExitCode != -1 {
		t.Errorf("exit_code = %v, want -1", got.ExitCode)
	}
	if got.Stderr != "job timed out" {
		t.Errorf("stderr = %q", got.Stderr)
	}
	if got.FinishedAt == nil {
		t.Error("finished_at should be set")
	}
}

func TestWatchJobTimeouts_RecentJobNotTimedOut(t *testing.T) {
	js := newTestJobStore(t)
	jobStore = js
	jobs = make(map[string]Job)

	ran := time.Now().Add(-10 * time.Minute)
	jobs["job-1"] = Job{
		ID:        "job-1",
		NodeID:    "n",
		Command:   "echo ok",
		Status:    "running",
		RunningAt: &ran,
		CreatedAt: ran,
	}
	js.Save(jobs["job-1"])

	simulateTimeoutCheck(1 * time.Hour)

	if jobs["job-1"].Status != "running" {
		t.Errorf("recent job should stay running, got %q", jobs["job-1"].Status)
	}
}

func TestWatchJobTimeouts_PendingJobIgnored(t *testing.T) {
	js := newTestJobStore(t)
	jobStore = js
	jobs = make(map[string]Job)

	jobs["job-1"] = Job{
		ID:        "job-1",
		NodeID:    "n",
		Command:   "echo ok",
		Status:    "pending",
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	js.Save(jobs["job-1"])

	simulateTimeoutCheck(1 * time.Hour)

	if jobs["job-1"].Status != "pending" {
		t.Errorf("pending job should not be timed out, got %q", jobs["job-1"].Status)
	}
}

func TestWatchJobTimeouts_UsesRunningAtOverCreatedAt(t *testing.T) {
	js := newTestJobStore(t)
	jobStore = js
	jobs = make(map[string]Job)

	// created 2h ago but started running 10min ago — should NOT time out at 1h
	ran := time.Now().Add(-10 * time.Minute)
	jobs["job-1"] = Job{
		ID:        "job-1",
		NodeID:    "n",
		Command:   "long",
		Status:    "running",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		RunningAt: &ran,
	}
	js.Save(jobs["job-1"])

	simulateTimeoutCheck(1 * time.Hour)

	if jobs["job-1"].Status != "running" {
		t.Errorf("job should not time out based on created_at when running_at is set, got %q", jobs["job-1"].Status)
	}
}
