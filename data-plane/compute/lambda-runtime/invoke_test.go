package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPollJobResult_Done(t *testing.T) {
	scheduler := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "done",
			"stdout": `{"result": 42}`,
		})
	}))
	defer scheduler.Close()

	result := pollJobResult(scheduler.URL, "job-1")

	if result.StatusCode != 200 {
		t.Errorf("status_code = %d, want 200", result.StatusCode)
	}
	if result.Output != `{"result": 42}` {
		t.Errorf("output = %q", result.Output)
	}
	if result.Error != "" {
		t.Errorf("error = %q, want empty", result.Error)
	}
}

func TestPollJobResult_Failed(t *testing.T) {
	scheduler := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "failed",
			"stdout": "",
			"stderr": "module not found",
		})
	}))
	defer scheduler.Close()

	result := pollJobResult(scheduler.URL, "job-2")

	if result.StatusCode != 500 {
		t.Errorf("status_code = %d, want 500", result.StatusCode)
	}
	if result.Error != "module not found" {
		t.Errorf("error = %q", result.Error)
	}
}

func TestPollJobResult_EventualSuccess(t *testing.T) {
	calls := 0
	scheduler := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls < 3 {
			json.NewEncoder(w).Encode(map[string]any{"status": "running"})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"status": "done",
				"stdout": "ok",
			})
		}
	}))
	defer scheduler.Close()

	result := pollJobResult(scheduler.URL, "job-3")

	if result.StatusCode != 200 {
		t.Errorf("status_code = %d, want 200", result.StatusCode)
	}
	if result.Output != "ok" {
		t.Errorf("output = %q", result.Output)
	}
}

func TestBuildInvokeCommand(t *testing.T) {
	py := buildInvokeCommand("python3")
	if py == "" || py == "echo unsupported-runtime" {
		t.Error("python3 should return a real command")
	}

	node := buildInvokeCommand("node20")
	if node == "" || node == "echo unsupported-runtime" {
		t.Error("node20 should return a real command")
	}

	unknown := buildInvokeCommand("ruby")
	if unknown != "echo unsupported-runtime" {
		t.Errorf("unknown runtime = %q, want echo unsupported-runtime", unknown)
	}
}
