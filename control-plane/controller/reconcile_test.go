package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestReconcileOnce_CleansTerminatedWorkspaces(t *testing.T) {
	// Create temp workspace dirs for 3 instances
	tmpDir := t.TempDir()
	t.Setenv("TEMP", tmpDir)
	t.Setenv("TMP", tmpDir)

	ids := []string{"i-aaa", "i-bbb", "i-ccc"}
	for _, id := range ids {
		dir := filepath.Join(tmpDir, "tinyaws", id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Drop a marker file so we can verify deletion
		os.WriteFile(filepath.Join(dir, "data.txt"), []byte("x"), 0o644)
	}

	// Mock registry: i-aaa and i-ccc terminated, i-bbb running
	instances := []Instance{
		{ID: "i-aaa", Status: "terminated"},
		{ID: "i-bbb", Status: "running"},
		{ID: "i-ccc", Status: "terminated"},
	}
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(instances)
	}))
	defer registry.Close()

	t.Setenv("REGISTRY_URL", registry.URL)

	cleaned := make(map[string]bool)
	reconcileOnce(cleaned)

	// i-aaa should be cleaned
	if _, err := os.Stat(filepath.Join(tmpDir, "tinyaws", "i-aaa")); !os.IsNotExist(err) {
		t.Error("i-aaa workspace should be deleted")
	}
	// i-bbb should still exist
	if _, err := os.Stat(filepath.Join(tmpDir, "tinyaws", "i-bbb")); os.IsNotExist(err) {
		t.Error("i-bbb workspace should still exist")
	}
	// i-ccc should be cleaned
	if _, err := os.Stat(filepath.Join(tmpDir, "tinyaws", "i-ccc")); !os.IsNotExist(err) {
		t.Error("i-ccc workspace should be deleted")
	}

	// cleaned map should track terminated ones
	if !cleaned["i-aaa"] || !cleaned["i-ccc"] {
		t.Errorf("cleaned map = %v, want i-aaa and i-ccc", cleaned)
	}
	if cleaned["i-bbb"] {
		t.Error("i-bbb should not be in cleaned map")
	}
}

func TestReconcileOnce_SkipsAlreadyCleaned(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TEMP", tmpDir)
	t.Setenv("TMP", tmpDir)

	instances := []Instance{{ID: "i-xxx", Status: "terminated"}}
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(instances)
	}))
	defer registry.Close()

	t.Setenv("REGISTRY_URL", registry.URL)

	// Pre-mark as cleaned
	cleaned := map[string]bool{"i-xxx": true}
	reconcileOnce(cleaned)

	// Should not attempt removal (no error, no crash) — just a no-op
	if len(cleaned) != 1 {
		t.Errorf("cleaned map changed unexpectedly: %v", cleaned)
	}
}

func TestReconcileOnce_RegistryBadJSON(t *testing.T) {
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer registry.Close()

	t.Setenv("REGISTRY_URL", registry.URL)
	cleaned := make(map[string]bool)
	reconcileOnce(cleaned)

	if len(cleaned) != 0 {
		t.Errorf("cleaned map should be empty on bad JSON: %v", cleaned)
	}
}
