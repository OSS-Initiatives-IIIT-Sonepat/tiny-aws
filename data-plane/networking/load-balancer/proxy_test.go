package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLBTargets_Endpoint(t *testing.T) {
	targetsMu.Lock()
	targets = []Target{
		{URL: "http://a:8080", Healthy: true, NodeID: "n1"},
		{URL: "http://b:3000", Healthy: false, NodeID: "n2", ServiceID: "svc-1"},
	}
	targetsMu.Unlock()

	req := httptest.NewRequest("GET", "/targets", nil)
	w := httptest.NewRecorder()
	http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetsMu.RLock()
		out, _ := json.Marshal(targets)
		targetsMu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	}).ServeHTTP(w, req)

	var got []Target
	json.Unmarshal(w.Body.Bytes(), &got)
	if len(got) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(got))
	}
	if got[1].ServiceID != "svc-1" {
		t.Errorf("target[1].ServiceID = %q", got[1].ServiceID)
	}
}

func TestLBProxy_NoBackends(t *testing.T) {
	targetsMu.Lock()
	targets = nil
	targetsMu.Unlock()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t0 := nextTarget()
		if t0 == nil {
			http.Error(w, "no healthy backends", http.StatusServiceUnavailable)
			return
		}
	})

	req := httptest.NewRequest("GET", "/anything", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestLBProxy_RoutesToBackend(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"from": "backend"})
	}))
	defer backend.Close()

	targetsMu.Lock()
	targets = []Target{{URL: backend.URL, Healthy: true, NodeID: "n1"}}
	counter = 0
	targetsMu.Unlock()

	t0 := nextTarget()
	if t0 == nil {
		t.Fatal("no target returned")
	}
	if t0.URL != backend.URL {
		t.Errorf("target URL = %q", t0.URL)
	}
}
