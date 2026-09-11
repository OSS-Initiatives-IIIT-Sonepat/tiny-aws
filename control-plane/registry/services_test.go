package main

import (
	"testing"
)

func TestServiceStoreCreateAndLoadAll(t *testing.T) {
	s := NewNodeStore(":memory:")
	ss := NewServiceStore(s.DB())

	svc := ss.Create("node-1", "i-1", "http://store/deploy.zip", 3000, 1234)
	if svc.ID != "svc-1" {
		t.Errorf("ID = %q, want %q", svc.ID, "svc-1")
	}
	if svc.Status != "running" {
		t.Errorf("status = %q, want %q", svc.Status, "running")
	}
	if svc.Port != 3000 {
		t.Errorf("port = %d, want 3000", svc.Port)
	}

	loaded, err := ss.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 service, got %d", len(loaded))
	}
	if loaded[0].NodeID != "node-1" {
		t.Errorf("node_id = %q, want %q", loaded[0].NodeID, "node-1")
	}
}

func TestServiceStoreSequenceIncrement(t *testing.T) {
	s := NewNodeStore(":memory:")
	ss := NewServiceStore(s.DB())

	s1 := ss.Create("node-1", "i-1", "", 3000, 100)
	s2 := ss.Create("node-1", "i-2", "", 3001, 200)

	if s1.ID != "svc-1" || s2.ID != "svc-2" {
		t.Errorf("IDs = %q, %q; want svc-1, svc-2", s1.ID, s2.ID)
	}
}

func TestServiceStoreUpdateStatus(t *testing.T) {
	s := NewNodeStore(":memory:")
	ss := NewServiceStore(s.DB())

	svc := ss.Create("node-1", "i-1", "", 3000, 100)
	if err := ss.UpdateStatus(svc.ID, "stopped"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	loaded, _ := ss.LoadAll()
	if loaded[0].Status != "stopped" {
		t.Errorf("status = %q, want %q", loaded[0].Status, "stopped")
	}
}

func TestServiceStoreSaveUpserts(t *testing.T) {
	s := NewNodeStore(":memory:")
	ss := NewServiceStore(s.DB())

	svc := ss.Create("node-1", "i-1", "", 3000, 100)
	svc.Status = "crashed"
	if err := ss.Save(svc); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, _ := ss.LoadAll()
	if len(loaded) != 1 {
		t.Fatalf("expected 1 service after upsert, got %d", len(loaded))
	}
	if loaded[0].Status != "crashed" {
		t.Errorf("status = %q, want %q", loaded[0].Status, "crashed")
	}
}
