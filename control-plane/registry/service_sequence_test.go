package main

import (
	"fmt"
	"testing"
	"time"
)

func TestServiceStoreSequenceRestoredFromDB(t *testing.T) {
	s := NewNodeStore(":memory:")
	db := s.DB()

	ss1 := NewServiceStore(db)
	ss1.Create("node-1", "i-1", "", 3000, 100) // svc-1
	ss1.Create("node-1", "i-2", "", 3001, 200) // svc-2
	ss1.Create("node-1", "i-3", "", 3002, 300) // svc-3

	// Simulate restart: new ServiceStore from same DB.
	ss2 := NewServiceStore(db)
	svc := ss2.Create("node-1", "i-4", "", 3003, 400)
	if svc.ID != "svc-4" {
		t.Errorf("after restart, next ID = %q, want %q", svc.ID, "svc-4")
	}
}

func TestServiceStoreSequenceRestoredEmpty(t *testing.T) {
	s := NewNodeStore(":memory:")
	db := s.DB()

	ss := NewServiceStore(db)
	svc := ss.Create("node-1", "i-1", "", 3000, 100)
	if svc.ID != "svc-1" {
		t.Errorf("first ID = %q, want %q", svc.ID, "svc-1")
	}
}

func TestServiceStoreSequenceRestoredFromManualInsert(t *testing.T) {
	s := NewNodeStore(":memory:")
	db := s.DB()

	_ = NewServiceStore(db) // create table

	now := time.Now().UTC().Format(time.RFC3339)
	db.Exec(`INSERT INTO services (id, node_id, port, pid, status, deploy_url, created_at)
		VALUES ('svc-10', 'node-1', 3000, 100, 'running', '', ?)`, now)
	db.Exec(`INSERT INTO services (id, node_id, port, pid, status, deploy_url, created_at)
		VALUES ('svc-20', 'node-1', 3001, 200, 'running', '', ?)`, now)

	ss2 := NewServiceStore(db)
	svc := ss2.Create("node-1", "i-1", "", 3002, 300)
	if svc.ID != "svc-21" {
		t.Errorf("after manual insert, next ID = %q, want %q", svc.ID, "svc-21")
	}
}

func TestServiceStoreSequenceMultipleRestarts(t *testing.T) {
	s := NewNodeStore(":memory:")
	db := s.DB()

	for restart := 0; restart < 3; restart++ {
		ss := NewServiceStore(db)
		ss.Create("node-1", fmt.Sprintf("i-%d", restart), "", 3000+restart, restart)
	}

	ss := NewServiceStore(db)
	svc := ss.Create("node-1", "i-final", "", 4000, 999)
	if svc.ID != "svc-4" {
		t.Errorf("after 3 restarts, next ID = %q, want %q", svc.ID, "svc-4")
	}
}
