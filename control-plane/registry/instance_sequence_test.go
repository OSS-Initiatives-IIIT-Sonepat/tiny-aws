package main

import (
	"testing"
	"time"
)

func TestInstanceStoreSequenceRestoredFromDB(t *testing.T) {
	s := NewNodeStore(":memory:")
	db := s.DB()

	// First store: create some instances so the DB has rows.
	is1 := NewInstanceStore(db)
	is1.Create("node-1", "small") // i-1
	is1.Create("node-1", "small") // i-2
	is1.Create("node-1", "small") // i-3

	// Simulate restart: create a new InstanceStore from the same DB.
	// It should read max sequence (3) from existing rows.
	is2 := NewInstanceStore(db)
	inst := is2.Create("node-1", "small")
	if inst.ID != "i-4" {
		t.Errorf("after restart, next ID = %q, want %q", inst.ID, "i-4")
	}
}

func TestInstanceStoreSequenceRestoredEmpty(t *testing.T) {
	s := NewNodeStore(":memory:")
	db := s.DB()

	// No pre-existing instances — sequence should start at 0.
	is := NewInstanceStore(db)
	inst := is.Create("node-1", "nano")
	if inst.ID != "i-1" {
		t.Errorf("first ID = %q, want %q", inst.ID, "i-1")
	}
}

func TestInstanceStoreSequenceRestoredFromManualInsert(t *testing.T) {
	s := NewNodeStore(":memory:")
	db := s.DB()

	// Manually insert rows as if from a previous run, to verify
	// NewInstanceStore scans the last rowid correctly.
	is1 := NewInstanceStore(db)
	_ = is1 // just to create the table

	now := time.Now().UTC().Format(time.RFC3339)
	db.Exec(`INSERT INTO instances (id, node_id, status, instance_type, cpu_limit, mem_limit_mb, created_at)
		VALUES ('i-10', 'node-1', 'running', 'small', '100%', 512, ?)`, now)
	db.Exec(`INSERT INTO instances (id, node_id, status, instance_type, cpu_limit, mem_limit_mb, created_at)
		VALUES ('i-20', 'node-1', 'running', 'small', '100%', 512, ?)`, now)

	// New store should pick up i-20 as max (last rowid).
	is2 := NewInstanceStore(db)
	inst := is2.Create("node-1", "small")
	if inst.ID != "i-21" {
		t.Errorf("after manual insert, next ID = %q, want %q", inst.ID, "i-21")
	}
}
