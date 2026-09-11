package main

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestNewNodeStore(t *testing.T) {
	s := NewNodeStore(":memory:")
	if s == nil {
		t.Fatal("expected non-nil store")
	}
	if s.DB() == nil {
		t.Fatal("expected non-nil DB handle")
	}
}

func TestNodeStoreSaveAndLoadAll(t *testing.T) {
	s := NewNodeStore(":memory:")

	node := Node{
		ID:       "node-1",
		Hostname: "host-a",
		Addr:     "10.0.0.1",
		CPUCount: 4,
		Role:     "compute",
		Status:   "healthy",
		LastSeen: time.Now().UTC().Truncate(time.Second),
	}

	if err := s.Save(node); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 node, got %d", len(loaded))
	}
	got := loaded["node-1"]
	if got.Hostname != "host-a" {
		t.Errorf("hostname = %q, want %q", got.Hostname, "host-a")
	}
	if got.Addr != "10.0.0.1" {
		t.Errorf("addr = %q, want %q", got.Addr, "10.0.0.1")
	}
	if got.CPUCount != 4 {
		t.Errorf("cpu_count = %d, want 4", got.CPUCount)
	}
}

func TestNodeStoreSaveUpdatesExisting(t *testing.T) {
	s := NewNodeStore(":memory:")

	node := Node{
		ID: "node-1", Hostname: "host-a", CPUCount: 2,
		Role: "compute", Status: "healthy", LastSeen: time.Now().UTC(),
	}
	s.Save(node)

	node.Status = "unhealthy"
	s.Save(node)

	loaded, _ := s.LoadAll()
	if loaded["node-1"].Status != "unhealthy" {
		t.Errorf("status = %q, want %q", loaded["node-1"].Status, "unhealthy")
	}
	if len(loaded) != 1 {
		t.Errorf("expected 1 node after upsert, got %d", len(loaded))
	}
}

func TestNodeStoreDelete(t *testing.T) {
	s := NewNodeStore(":memory:")
	node := Node{
		ID: "node-1", Hostname: "h", CPUCount: 1,
		Role: "compute", Status: "healthy", LastSeen: time.Now().UTC(),
	}
	s.Save(node)
	if err := s.Delete("node-1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	loaded, _ := s.LoadAll()
	if len(loaded) != 0 {
		t.Errorf("expected 0 nodes after delete, got %d", len(loaded))
	}
}

func TestNodeStoreDeleteNonExistent(t *testing.T) {
	s := NewNodeStore(":memory:")
	// should not error on missing key
	if err := s.Delete("does-not-exist"); err != nil {
		t.Fatalf("Delete non-existent failed: %v", err)
	}
}
