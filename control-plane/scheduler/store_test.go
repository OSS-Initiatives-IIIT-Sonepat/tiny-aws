package main

import (
	"fmt"
	"testing"
	"time"
)

func newTestJobStore(t *testing.T) *JobStore {
	t.Helper()
	return NewJobStore(":memory:")
}

func TestJobStoreSaveAndLoadAll(t *testing.T) {
	js := newTestJobStore(t)

	job := Job{
		ID:        "job-1",
		NodeID:    "node-1",
		Command:   "echo hello",
		Status:    "pending",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := js.Save(job); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, maxSeq, err := js.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 job, got %d", len(loaded))
	}
	if maxSeq != 1 {
		t.Errorf("maxSeq = %d, want 1", maxSeq)
	}
	got := loaded["job-1"]
	if got.Command != "echo hello" {
		t.Errorf("command = %q", got.Command)
	}
}

func TestJobStoreUpsert(t *testing.T) {
	js := newTestJobStore(t)

	job := Job{
		ID: "job-1", NodeID: "node-1", Command: "echo 1",
		Status: "pending", CreatedAt: time.Now().UTC(),
	}
	js.Save(job)

	job.Status = "running"
	now := time.Now().UTC()
	job.RunningAt = &now
	js.Save(job)

	loaded, _, _ := js.LoadAll()
	if loaded["job-1"].Status != "running" {
		t.Errorf("status = %q, want %q", loaded["job-1"].Status, "running")
	}
	if loaded["job-1"].RunningAt == nil {
		t.Error("running_at should be set")
	}
}

func TestJobStoreWithEnvVars(t *testing.T) {
	js := newTestJobStore(t)

	job := Job{
		ID: "job-1", NodeID: "node-1", Command: "echo $FOO",
		Status: "pending", CreatedAt: time.Now().UTC(),
		EnvVars: map[string]string{"FOO": "bar", "BAZ": "qux"},
	}
	js.Save(job)

	loaded, _, _ := js.LoadAll()
	got := loaded["job-1"]
	if got.EnvVars["FOO"] != "bar" || got.EnvVars["BAZ"] != "qux" {
		t.Errorf("env_vars = %v", got.EnvVars)
	}
}

func TestJobStoreMaxSequence(t *testing.T) {
	js := newTestJobStore(t)

	for i := 1; i <= 5; i++ {
		js.Save(Job{
			ID: fmt.Sprintf("job-%d", i), NodeID: "n", Command: "x",
			Status: "done", CreatedAt: time.Now().UTC(),
		})
	}

	_, maxSeq, _ := js.LoadAll()
	if maxSeq != 5 {
		t.Errorf("maxSeq = %d, want 5", maxSeq)
	}
}

func TestJobStoreDoneWithExitCode(t *testing.T) {
	js := newTestJobStore(t)

	code := 0
	now := time.Now().UTC()
	job := Job{
		ID: "job-1", NodeID: "n", Command: "echo ok",
		Status: "done", ExitCode: &code,
		Stdout: "ok\n", CreatedAt: time.Now().UTC(),
		FinishedAt: &now,
	}
	js.Save(job)

	loaded, _, _ := js.LoadAll()
	got := loaded["job-1"]
	if got.ExitCode == nil || *got.ExitCode != 0 {
		t.Errorf("exit_code = %v", got.ExitCode)
	}
	if got.Stdout != "ok\n" {
		t.Errorf("stdout = %q", got.Stdout)
	}
}
