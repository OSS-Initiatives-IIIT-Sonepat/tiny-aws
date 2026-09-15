package main

import (
	"testing"
	"time"
)

// Test that env_vars JSON serialization/deserialization works through the store.
func TestJobStoreEnvVarsRoundTrip(t *testing.T) {
	js := newTestJobStore(t)

	envVars := map[string]string{
		"DB_HOST":   "localhost",
		"DB_PORT":   "5432",
		"LOG_LEVEL": "debug",
		"EMPTY_VAL": "",
	}

	job := Job{
		ID:        "job-1",
		NodeID:    "node-1",
		Command:   "start-server",
		Status:    "pending",
		EnvVars:   envVars,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := js.Save(job); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, _, err := js.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	got := loaded["job-1"]

	if got.EnvVars["DB_HOST"] != "localhost" {
		t.Errorf("DB_HOST = %q", got.EnvVars["DB_HOST"])
	}
	if got.EnvVars["DB_PORT"] != "5432" {
		t.Errorf("DB_PORT = %q", got.EnvVars["DB_PORT"])
	}
	if got.EnvVars["LOG_LEVEL"] != "debug" {
		t.Errorf("LOG_LEVEL = %q", got.EnvVars["LOG_LEVEL"])
	}
}

func TestJobStoreEnvVarsNil(t *testing.T) {
	js := newTestJobStore(t)

	job := Job{
		ID:        "job-2",
		NodeID:    "node-1",
		Command:   "echo hi",
		Status:    "pending",
		EnvVars:   nil,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	js.Save(job)

	loaded, _, _ := js.LoadAll()
	got := loaded["job-2"]
	// nil env_vars should not cause errors; loaded map should be nil or empty
	if got.EnvVars != nil && len(got.EnvVars) > 0 {
		t.Errorf("expected nil/empty env_vars, got %v", got.EnvVars)
	}
}

func TestJobStoreEnvVarsUpdate(t *testing.T) {
	js := newTestJobStore(t)

	job := Job{
		ID: "job-3", NodeID: "n", Command: "x",
		Status: "pending", CreatedAt: time.Now().UTC(),
		EnvVars: map[string]string{"A": "1"},
	}
	js.Save(job)

	// update env vars
	job.EnvVars = map[string]string{"A": "2", "B": "3"}
	js.Save(job)

	loaded, _, _ := js.LoadAll()
	got := loaded["job-3"]
	if got.EnvVars["A"] != "2" {
		t.Errorf("A = %q, want 2", got.EnvVars["A"])
	}
	if got.EnvVars["B"] != "3" {
		t.Errorf("B = %q, want 3", got.EnvVars["B"])
	}
}
