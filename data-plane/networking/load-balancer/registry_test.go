package main

import (
	"testing"
)

// Test registryURL helper function.
func TestRegistryURL_DefaultValue(t *testing.T) {
	t.Setenv("REGISTRY_URL", "")
	got := registryURL()
	if got != "http://127.0.0.1:9000" {
		t.Errorf("registryURL() = %q, want %q", got, "http://127.0.0.1:9000")
	}
}

func TestRegistryURL_CustomValue(t *testing.T) {
	t.Setenv("REGISTRY_URL", "http://registry:9000")
	got := registryURL()
	if got != "http://registry:9000" {
		t.Errorf("registryURL() = %q, want %q", got, "http://registry:9000")
	}
}
