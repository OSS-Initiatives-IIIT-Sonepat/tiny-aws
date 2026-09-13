package main

import (
	"testing"
)

func TestPickHealthyComputeNodeLocal(t *testing.T) {
	setupHandlerTest(t)

	nodes["c1"] = Node{ID: "c1", Role: "compute", Status: "healthy"}
	nodes["c2"] = Node{ID: "c2", Role: "compute", Status: "unhealthy"}
	nodes["s1"] = Node{ID: "s1", Role: "storage", Status: "healthy"}

	got, err := pickHealthyComputeNodeLocal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "c1" {
		t.Errorf("got node %q, want c1", got.ID)
	}
	if got.Role != "compute" {
		t.Errorf("role = %q, want compute", got.Role)
	}
}

func TestPickHealthyComputeNodeLocal_NoHealthy(t *testing.T) {
	setupHandlerTest(t)

	nodes["c1"] = Node{ID: "c1", Role: "compute", Status: "unhealthy"}
	nodes["s1"] = Node{ID: "s1", Role: "storage", Status: "healthy"}

	_, err := pickHealthyComputeNodeLocal()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPickHealthyComputeNodeLocal_Empty(t *testing.T) {
	setupHandlerTest(t)

	_, err := pickHealthyComputeNodeLocal()
	if err == nil {
		t.Fatal("expected error on empty nodes")
	}
}
