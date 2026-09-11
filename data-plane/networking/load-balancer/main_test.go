package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLBHealth(t *testing.T) {
	targets = nil

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetsMu.RLock()
		n := len(targets)
		targetsMu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "healthy", "service": "lb", "targets": n})
	}).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["service"] != "lb" {
		t.Errorf("service = %v", body["service"])
	}
}

func TestNextTarget_NoTargets(t *testing.T) {
	targetsMu.Lock()
	targets = nil
	targetsMu.Unlock()

	if got := nextTarget(); got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestNextTarget_AllUnhealthy(t *testing.T) {
	targetsMu.Lock()
	targets = []Target{
		{URL: "http://a:8080", Healthy: false},
		{URL: "http://b:8080", Healthy: false},
	}
	targetsMu.Unlock()

	if got := nextTarget(); got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestNextTarget_RoundRobin(t *testing.T) {
	targetsMu.Lock()
	targets = []Target{
		{URL: "http://a:8080", Healthy: true, NodeID: "n1"},
		{URL: "http://b:8080", Healthy: true, NodeID: "n2"},
	}
	counter = 0
	targetsMu.Unlock()

	urls := map[string]bool{}
	for i := 0; i < 4; i++ {
		t0 := nextTarget()
		if t0 == nil {
			t.Fatal("nextTarget returned nil")
		}
		urls[t0.URL] = true
	}
	if len(urls) != 2 {
		t.Errorf("expected 2 unique URLs, got %d", len(urls))
	}
}

func TestCheckHealth_Reachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if !checkHealth(server.URL) {
		t.Error("expected healthy for reachable server")
	}
}

func TestCheckHealth_Unreachable(t *testing.T) {
	if checkHealth("http://127.0.0.1:1") {
		t.Error("expected unhealthy for unreachable address")
	}
}
