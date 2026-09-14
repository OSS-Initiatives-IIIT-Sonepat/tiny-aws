package main

import (
	"testing"
	"time"
)

func TestJobTypeDefaultRun(t *testing.T) {
	js := newTestJobStore(t)

	job := Job{
		ID: "job-1", NodeID: "n1", Command: "echo hi",
		Status: "pending", CreatedAt: time.Now().UTC(),
	}
	js.Save(job)

	loaded, _, _ := js.LoadAll()
	got := loaded["job-1"]
	if got.JobType != "" {
		// When job_type is empty, the DB default column is 'run',
		// but the Go struct stays "" unless explicitly set.
		// Either "" or "run" is acceptable from LoadAll.
		if got.JobType != "run" {
			t.Errorf("job_type = %q, want empty or 'run'", got.JobType)
		}
	}
}

func TestJobTypeService(t *testing.T) {
	js := newTestJobStore(t)

	job := Job{
		ID: "job-1", NodeID: "n1", Command: "python server.py",
		JobType: "service", Port: 8080,
		Status: "pending", CreatedAt: time.Now().UTC(),
	}
	js.Save(job)

	loaded, _, _ := js.LoadAll()
	got := loaded["job-1"]
	if got.JobType != "service" {
		t.Errorf("job_type = %q, want 'service'", got.JobType)
	}
	if got.Port != 8080 {
		t.Errorf("port = %d, want 8080", got.Port)
	}
}

func TestJobTypeServicePortZeroDefault(t *testing.T) {
	js := newTestJobStore(t)

	job := Job{
		ID: "job-1", NodeID: "n1", Command: "echo",
		JobType: "run",
		Status:  "pending", CreatedAt: time.Now().UTC(),
	}
	js.Save(job)

	loaded, _, _ := js.LoadAll()
	if loaded["job-1"].Port != 0 {
		t.Errorf("port = %d, want 0 for run job", loaded["job-1"].Port)
	}
}
