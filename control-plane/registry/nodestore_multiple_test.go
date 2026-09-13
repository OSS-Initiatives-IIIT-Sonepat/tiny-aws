package main

import (
	"testing"
	"time"
)

// Test storing and loading multiple nodes of different roles.
func TestNodeStoreMultipleRoles(t *testing.T) {
	s := NewNodeStore(":memory:")
	now := time.Now().UTC().Truncate(time.Second)

	nodes := []Node{
		{ID: "n-1", Hostname: "compute-1", Addr: "10.0.0.1", CPUCount: 4, Role: "compute", Status: "healthy", LastSeen: now},
		{ID: "n-2", Hostname: "storage-1", Addr: "10.0.0.2", CPUCount: 2, Role: "storage", Status: "healthy", LastSeen: now},
		{ID: "n-3", Hostname: "compute-2", Addr: "10.0.0.3", CPUCount: 8, Role: "compute", Status: "unhealthy", LastSeen: now},
	}

	for _, n := range nodes {
		if err := s.Save(n); err != nil {
			t.Fatalf("Save(%s): %v", n.ID, err)
		}
	}

	loaded, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(loaded))
	}

	// verify each node round-trips correctly
	for _, want := range nodes {
		got, ok := loaded[want.ID]
		if !ok {
			t.Errorf("node %s not found", want.ID)
			continue
		}
		if got.Role != want.Role {
			t.Errorf("%s: role = %q, want %q", want.ID, got.Role, want.Role)
		}
		if got.Status != want.Status {
			t.Errorf("%s: status = %q, want %q", want.ID, got.Status, want.Status)
		}
		if got.CPUCount != want.CPUCount {
			t.Errorf("%s: cpu = %d, want %d", want.ID, got.CPUCount, want.CPUCount)
		}
		if got.Addr != want.Addr {
			t.Errorf("%s: addr = %q, want %q", want.ID, got.Addr, want.Addr)
		}
	}
}
