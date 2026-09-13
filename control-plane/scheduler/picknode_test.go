package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchComputeNodes(t *testing.T) {
	setupSchedulerTest(t)

	nodesMap := map[string]Node{
		"c1": {ID: "c1", Role: "compute", Status: "healthy"},
		"c2": {ID: "c2", Role: "compute", Status: "unhealthy"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nodesMap)
	}))
	defer srv.Close()

	got, err := fetchComputeNodes(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(got))
	}
}

func TestPickHealthyComputeNode(t *testing.T) {
	setupSchedulerTest(t)

	nodesMap := map[string]Node{
		"c1": {ID: "c1", Role: "compute", Status: "healthy"},
		"c2": {ID: "c2", Role: "compute", Status: "unhealthy"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nodesMap)
	}))
	defer srv.Close()

	got, err := pickHealthyComputeNode(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != "healthy" {
		t.Errorf("picked node status = %q, want healthy", got.Status)
	}
	if got.ID != "c1" {
		t.Errorf("picked node = %q, want c1", got.ID)
	}
}

func TestPickHealthyComputeNode_NoneHealthy(t *testing.T) {
	setupSchedulerTest(t)

	nodesMap := map[string]Node{
		"c1": {ID: "c1", Role: "compute", Status: "unhealthy"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nodesMap)
	}))
	defer srv.Close()

	_, err := pickHealthyComputeNode(srv.URL)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
