package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspacePath(t *testing.T) {
	got := workspacePath("i-1")
	want := filepath.Join(os.TempDir(), "tinyaws", "i-1")
	if got != want {
		t.Errorf("workspacePath = %q, want %q", got, want)
	}
}

func TestRegistryURL_Default(t *testing.T) {
	t.Setenv("REGISTRY_URL", "")
	if got := registryURL(); got != "http://127.0.0.1:9000" {
		t.Errorf("default = %q", got)
	}
}

func TestRegistryURL_Custom(t *testing.T) {
	t.Setenv("REGISTRY_URL", "http://reg:9000")
	if got := registryURL(); got != "http://reg:9000" {
		t.Errorf("got = %q", got)
	}
}

func TestReconcileOnce_RegistryDown(t *testing.T) {
	t.Setenv("REGISTRY_URL", "http://127.0.0.1:1")
	cleaned := make(map[string]bool)
	// should not panic when registry is unreachable
	reconcileOnce(cleaned)
	if len(cleaned) != 0 {
		t.Error("expected no cleanups")
	}
}
