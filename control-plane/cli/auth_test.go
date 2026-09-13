package main

import (
	"testing"
)

// Test auth command URL construction without making HTTP calls.
func TestAuthSetKey_URLConstruction(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "")
	t.Setenv("REGISTRY_URL", "")

	// default registry URL should be used for IAM key endpoint
	got := registryURL() + "/iam/keys"
	want := "http://127.0.0.1:9000/iam/keys"
	if got != want {
		t.Errorf("set-key URL = %q, want %q", got, want)
	}
}

func TestAuthSetKey_URLConstruction_CustomRegistry(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "")
	t.Setenv("REGISTRY_URL", "http://registry:9000")

	got := registryURL() + "/iam/keys"
	want := "http://registry:9000/iam/keys"
	if got != want {
		t.Errorf("set-key URL = %q, want %q", got, want)
	}
}

func TestAuthSetKey_URLConstruction_Gateway(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "http://gw:8000")

	got := registryURL() + "/iam/keys"
	want := "http://gw:8000/v1/iam/keys"
	if got != want {
		t.Errorf("set-key URL via gateway = %q, want %q", got, want)
	}
}

func TestAuthWhoami_URLConstruction(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "")
	t.Setenv("REGISTRY_URL", "")

	got := registryURL() + "/iam/keys"
	want := "http://127.0.0.1:9000/iam/keys"
	if got != want {
		t.Errorf("whoami URL = %q, want %q", got, want)
	}
}

func TestAuthWhoami_URLConstruction_Gateway(t *testing.T) {
	t.Setenv("TINYAWS_API_URL", "https://api.example.com")

	got := registryURL() + "/iam/keys"
	want := "https://api.example.com/v1/iam/keys"
	if got != want {
		t.Errorf("whoami URL via gateway = %q, want %q", got, want)
	}
}
