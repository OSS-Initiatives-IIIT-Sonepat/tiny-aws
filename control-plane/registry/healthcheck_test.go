package main

import (
	"testing"
	"time"
)

// simulateHealthCheck runs the same logic as checkNodesHealth (one tick).
func simulateHealthCheck() {
	now := time.Now()

	nodesMu.Lock()
	defer nodesMu.Unlock()

	for id, node := range nodes {
		age := now.Sub(node.LastSeen)

		if age > 5*time.Minute && node.Status == "unhealthy" {
			delete(nodes, id)
			_ = store.Delete(id)
			continue
		}

		if age > 30*time.Second {
			node.Status = "unhealthy"
			nodes[id] = node
			_ = store.Save(node)
		}
	}
}

func TestHealthCheck_NodeBecomesUnhealthy(t *testing.T) {
	setupHandlerTest(t)

	nodes["node-1"] = Node{
		ID: "node-1", Hostname: "h", CPUCount: 1,
		Role: "compute", Status: "healthy",
		LastSeen: time.Now().Add(-31 * time.Second),
	}
	store.Save(nodes["node-1"])

	simulateHealthCheck()

	got, ok := nodes["node-1"]
	if !ok {
		t.Fatal("node should still exist")
	}
	if got.Status != "unhealthy" {
		t.Errorf("status = %q, want unhealthy", got.Status)
	}
}

func TestHealthCheck_RecentNodeStaysHealthy(t *testing.T) {
	setupHandlerTest(t)

	nodes["node-1"] = Node{
		ID: "node-1", Hostname: "h", CPUCount: 1,
		Role: "compute", Status: "healthy",
		LastSeen: time.Now().Add(-10 * time.Second),
	}
	store.Save(nodes["node-1"])

	simulateHealthCheck()

	if nodes["node-1"].Status != "healthy" {
		t.Errorf("recent node should stay healthy, got %q", nodes["node-1"].Status)
	}
}

func TestHealthCheck_UnhealthyNodePrunedAfter5Min(t *testing.T) {
	setupHandlerTest(t)

	nodes["node-1"] = Node{
		ID: "node-1", Hostname: "h", CPUCount: 1,
		Role: "compute", Status: "unhealthy",
		LastSeen: time.Now().Add(-6 * time.Minute),
	}
	store.Save(nodes["node-1"])

	simulateHealthCheck()

	if _, ok := nodes["node-1"]; ok {
		t.Error("stale unhealthy node should have been pruned")
	}

	// verify deleted from store too
	loaded, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if _, ok := loaded["node-1"]; ok {
		t.Error("stale node should be deleted from store")
	}
}

func TestHealthCheck_UnhealthyNodeNotPrunedBefore5Min(t *testing.T) {
	setupHandlerTest(t)

	nodes["node-1"] = Node{
		ID: "node-1", Hostname: "h", CPUCount: 1,
		Role: "compute", Status: "unhealthy",
		LastSeen: time.Now().Add(-3 * time.Minute),
	}
	store.Save(nodes["node-1"])

	simulateHealthCheck()

	if _, ok := nodes["node-1"]; !ok {
		t.Error("unhealthy node under 5min should not be pruned")
	}
}
