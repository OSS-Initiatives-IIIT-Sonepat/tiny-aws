package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Test listInstances with node_id and status filters.
func TestListInstances_FilterByNodeID(t *testing.T) {
	s := NewNodeStore(":memory:")
	instanceStore = NewInstanceStore(s.DB())

	instanceStore.Create("node-a", "small")
	instanceStore.Create("node-b", "small")
	instanceStore.Create("node-a", "micro")

	loaded, _ := instanceStore.LoadAll()
	instances = loaded

	req := httptest.NewRequest("GET", "/instances?node_id=node-a", nil)
	w := httptest.NewRecorder()
	listInstances(w, req)

	var got []Instance
	json.Unmarshal(w.Body.Bytes(), &got)
	if len(got) != 2 {
		t.Errorf("expected 2 instances for node-a, got %d", len(got))
	}
	for _, inst := range got {
		if inst.NodeID != "node-a" {
			t.Errorf("expected node-a, got %q", inst.NodeID)
		}
	}
}

func TestListInstances_FilterByStatusStore(t *testing.T) {
	s := NewNodeStore(":memory:")
	instanceStore = NewInstanceStore(s.DB())

	inst := instanceStore.Create("node-a", "small")
	instanceStore.SetStatus(inst.ID, "running")
	instanceStore.Create("node-a", "small") // stays provisioning

	loaded, _ := instanceStore.LoadAll()
	instances = loaded

	req := httptest.NewRequest("GET", "/instances?status=running", nil)
	w := httptest.NewRecorder()
	listInstances(w, req)

	var got []Instance
	json.Unmarshal(w.Body.Bytes(), &got)
	if len(got) != 1 {
		t.Errorf("expected 1 running instance, got %d", len(got))
	}
}

func TestListInstances_NoFilters(t *testing.T) {
	s := NewNodeStore(":memory:")
	instanceStore = NewInstanceStore(s.DB())
	instances = nil

	// manually add to in-memory slice
	instances = []Instance{
		{ID: "i-1", NodeID: "n1", Status: "running", CreatedAt: time.Now().UTC()},
		{ID: "i-2", NodeID: "n2", Status: "terminated", CreatedAt: time.Now().UTC()},
	}

	req := httptest.NewRequest("GET", "/instances", nil)
	w := httptest.NewRecorder()
	listInstances(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var got []Instance
	json.Unmarshal(w.Body.Bytes(), &got)
	if len(got) != 2 {
		t.Errorf("expected 2, got %d", len(got))
	}
}
