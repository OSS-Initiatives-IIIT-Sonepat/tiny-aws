package main

import (
	"testing"
)

func TestInstanceStoreCreateAndLoadAll(t *testing.T) {
	s := NewNodeStore(":memory:")
	is := NewInstanceStore(s.DB())

	inst := is.Create("node-1", "small")
	if inst.ID != "i-1" {
		t.Errorf("first instance ID = %q, want %q", inst.ID, "i-1")
	}
	if inst.Status != "provisioning" {
		t.Errorf("status = %q, want %q", inst.Status, "provisioning")
	}
	if inst.InstanceType != "small" {
		t.Errorf("instance_type = %q, want %q", inst.InstanceType, "small")
	}
	if inst.CPULimit != "100%" {
		t.Errorf("cpu_limit = %q, want %q", inst.CPULimit, "100%")
	}
	if inst.MemLimitMB != 512 {
		t.Errorf("mem_limit_mb = %d, want 512", inst.MemLimitMB)
	}

	loaded, err := is.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(loaded))
	}
	if loaded[0].ID != "i-1" {
		t.Errorf("loaded ID = %q", loaded[0].ID)
	}
}

func TestInstanceStoreSequenceIncrement(t *testing.T) {
	s := NewNodeStore(":memory:")
	is := NewInstanceStore(s.DB())

	i1 := is.Create("node-1", "small")
	i2 := is.Create("node-1", "medium")

	if i1.ID != "i-1" || i2.ID != "i-2" {
		t.Errorf("IDs = %q, %q; want i-1, i-2", i1.ID, i2.ID)
	}
}

func TestInstanceStoreTerminate(t *testing.T) {
	s := NewNodeStore(":memory:")
	is := NewInstanceStore(s.DB())

	inst := is.Create("node-1", "small")
	if err := is.Terminate(inst.ID); err != nil {
		t.Fatalf("Terminate: %v", err)
	}

	loaded, _ := is.LoadAll()
	if loaded[0].Status != "terminated" {
		t.Errorf("status = %q, want %q", loaded[0].Status, "terminated")
	}
}

func TestInstanceStoreSetStatus(t *testing.T) {
	s := NewNodeStore(":memory:")
	is := NewInstanceStore(s.DB())

	inst := is.Create("node-1", "nano")
	is.SetStatus(inst.ID, "running")

	loaded, _ := is.LoadAll()
	if loaded[0].Status != "running" {
		t.Errorf("status = %q, want %q", loaded[0].Status, "running")
	}
}

func TestInstanceTypeSpecs(t *testing.T) {
	s := NewNodeStore(":memory:")
	is := NewInstanceStore(s.DB())

	tests := []struct {
		itype string
		cpu   string
		mem   int
	}{
		{"nano", "25%", 128},
		{"micro", "50%", 256},
		{"small", "100%", 512},
		{"medium", "200%", 1024},
		{"large", "400%", 2048},
	}
	for _, tt := range tests {
		inst := is.Create("node-1", tt.itype)
		if inst.CPULimit != tt.cpu {
			t.Errorf("%s: cpu = %q, want %q", tt.itype, inst.CPULimit, tt.cpu)
		}
		if inst.MemLimitMB != tt.mem {
			t.Errorf("%s: mem = %d, want %d", tt.itype, inst.MemLimitMB, tt.mem)
		}
	}
}

func TestInstanceStoreDefaultType(t *testing.T) {
	s := NewNodeStore(":memory:")
	is := NewInstanceStore(s.DB())

	inst := is.Create("node-1", "")
	if inst.InstanceType != "small" {
		t.Errorf("default type = %q, want %q", inst.InstanceType, "small")
	}
}

func TestInstanceStoreUnknownType(t *testing.T) {
	s := NewNodeStore(":memory:")
	is := NewInstanceStore(s.DB())

	inst := is.Create("node-1", "xlarge")
	// unknown type falls back to small defaults
	if inst.CPULimit != "100%" {
		t.Errorf("unknown type cpu = %q, want %q (small default)", inst.CPULimit, "100%")
	}
}
