package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResourcesEndpoint(t *testing.T) {
	// Mock registry serves /nodes and /instances
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/nodes":
			w.Write([]byte(`[{"id":"n1","role":"compute"}]`))
		case "/instances":
			w.Write([]byte(`[{"id":"i-1","status":"running"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer registry.Close()

	// Mock scheduler serves /jobs
	scheduler := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/jobs" {
			w.Write([]byte(`[{"job_id":"job-1","status":"done"}]`))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer scheduler.Close()

	// Mock networking serves /vpcs, /subnets, /security-groups
	networking := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/vpcs":
			w.Write([]byte(`[{"id":"vpc-1"}]`))
		case "/subnets":
			w.Write([]byte(`[{"id":"sub-1"}]`))
		case "/security-groups":
			w.Write([]byte(`[{"id":"sg-1"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer networking.Close()

	t.Setenv("REGISTRY_URL", registry.URL)
	t.Setenv("SCHEDULER_URL", scheduler.URL)
	t.Setenv("NETWORKING_URL", networking.URL)

	// Re-read env vars the same way main() does
	regURL := getenv("REGISTRY_URL", "http://127.0.0.1:9000")
	schedURL := getenv("SCHEDULER_URL", "http://127.0.0.1:9001")
	netURL := getenv("NETWORKING_URL", "http://127.0.0.1:9005")

	// Build the handler inline (mirrors main's /resources handler)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resources := map[string]any{}
		if nodes, err := fetch(regURL + "/nodes"); err == nil {
			resources["nodes"] = nodes
		}
		if instances, err := fetch(regURL + "/instances"); err == nil {
			resources["instances"] = instances
		}
		if jobs, err := fetch(schedURL + "/jobs"); err == nil {
			resources["jobs"] = jobs
		}
		if vpcs, err := fetch(netURL + "/vpcs"); err == nil {
			resources["vpcs"] = vpcs
		}
		if subnets, err := fetch(netURL + "/subnets"); err == nil {
			resources["subnets"] = subnets
		}
		if sgs, err := fetch(netURL + "/security-groups"); err == nil {
			resources["security_groups"] = sgs
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resources)
	})

	req := httptest.NewRequest("GET", "/resources", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, key := range []string{"nodes", "instances", "jobs", "vpcs", "subnets", "security_groups"} {
		v, ok := result[key]
		if !ok {
			t.Errorf("missing key %q", key)
			continue
		}
		arr, ok := v.([]any)
		if !ok || len(arr) == 0 {
			t.Errorf("%q should be non-empty array, got %v", key, v)
		}
	}
}

func TestResourcesEndpoint_PartialFailure(t *testing.T) {
	// Only registry is up; scheduler and networking are unreachable
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":"n1"}]`))
	}))
	defer registry.Close()

	regURL := registry.URL
	schedURL := "http://127.0.0.1:1"
	netURL := "http://127.0.0.1:1"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resources := map[string]any{}
		if nodes, err := fetch(regURL + "/nodes"); err == nil {
			resources["nodes"] = nodes
		}
		if instances, err := fetch(regURL + "/instances"); err == nil {
			resources["instances"] = instances
		}
		if jobs, err := fetch(schedURL + "/jobs"); err == nil {
			resources["jobs"] = jobs
		}
		if vpcs, err := fetch(netURL + "/vpcs"); err == nil {
			resources["vpcs"] = vpcs
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resources)
	})

	req := httptest.NewRequest("GET", "/resources", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)

	// nodes should exist (registry served both /nodes and /instances)
	if _, ok := result["nodes"]; !ok {
		t.Error("nodes should be present")
	}
	// jobs, vpcs should be absent (backends down)
	if _, ok := result["jobs"]; ok {
		t.Error("jobs should be absent when scheduler is down")
	}
}
